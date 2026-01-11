# Story 4.14: New User & Load Defaults System

**Story ID:** SETTINGS-4.14
**Epic:** Epic 4 - Database-First Mode Configuration System
**Priority:** P0 (Critical - User Onboarding & Reset Feature)
**Estimated Effort:** 6 hours
**Author:** BMAD Agent (Bob - Scrum Master)
**Status:** Ready for Development
**Depends On:** Story 4.13

---

## Problem Statement

### Current State

1. **New User Creation**: New users get hardcoded defaults scattered across Go code
2. **No Reset Feature**: Users cannot reset their settings to "factory defaults"
3. **Inconsistent Defaults**: Different code paths may produce different initial values

### Expected Behavior

1. **New User**: Copy ALL settings from `default-settings.json` to user's database record
2. **Load Defaults**: "Load Defaults" button available in UI to reset settings
3. **Preview Before Reset**: Show user what will change before applying defaults
4. **Granular Control**: Load defaults for individual groups OR entire settings

---

## User Stories

### Story A: New User Onboarding
> As a new user registering on the platform,
> I want my account to be initialized with the recommended default settings,
> So that I can start trading safely without manual configuration.

### Story B: Reset to Defaults
> As an existing user who has modified settings,
> I want to reset specific setting groups (or all settings) to defaults,
> So that I can recover from misconfiguration or try the recommended approach.

### Story C: Preview Changes
> As a user about to load defaults,
> I want to see exactly what will change before confirming,
> So that I don't accidentally lose important customizations.

---

## Acceptance Criteria

### AC4.14.1: New User Settings Initialization
- [ ] When new user registers, ALL settings copied from `default-settings.json`
- [ ] Settings stored in `user_mode_configs` and related tables
- [ ] All 5 mode configs initialized with complete sub-configurations
- [ ] Global settings (circuit breaker, LLM, etc.) initialized
- [ ] Log shows: `[USER-INIT] Initialized settings for user X from default-settings.json`

### AC4.14.2: Load Defaults API Endpoints
- [ ] `POST /api/user/settings/load-defaults` - Load ALL defaults
- [ ] `POST /api/user/settings/load-defaults/:group` - Load specific group
- [ ] `GET /api/user/settings/diff/:group` - Get diff between user and defaults
- [ ] Response includes preview of changes before applying

### AC4.14.3: Load Defaults for Mode Configs
- [ ] `POST /api/futures/ginie/modes/:mode/load-defaults` - Reset single mode
- [ ] `POST /api/futures/ginie/modes/load-defaults` - Reset ALL modes
- [ ] Preview shows: current value, default value, change indicator
- [ ] Only changed settings displayed in preview

### AC4.14.4: Load Defaults for Other Groups
- [ ] Position Optimization: `POST /api/futures/position-optimization/load-defaults`
- [ ] Circuit Breaker: `POST /api/futures/circuit-breaker/load-defaults`
- [ ] LLM Config: `POST /api/llm/load-defaults`
- [ ] Capital Allocation: `POST /api/futures/capital-allocation/load-defaults`

### AC4.14.5: Preview Before Apply
- [ ] Each load-defaults endpoint supports `?preview=true` query param
- [ ] Preview returns diff without applying changes
- [ ] Diff format: `{ "setting_path": { "current": X, "default": Y } }`
- [ ] Only different settings included in diff
- [ ] If no differences: `{ "message": "All settings match defaults" }`

### AC4.14.6: UI Load Defaults Buttons
- [ ] GiniePanel: "Load Mode Defaults" dropdown (per-mode + all modes)
- [ ] Position Optimization Panel: "Load Defaults" button
- [ ] User Settings Page: "Reset All to Defaults" button
- [ ] Confirmation dialog shows preview before applying
- [ ] Success toast shows how many settings were reset

---

## Technical Implementation

### Task 1: New User Initialization Service

```go
// internal/auth/user_settings_init.go

package auth

import (
    "context"
    "github.com/your-org/binance-trading-bot/internal/autopilot"
    "github.com/your-org/binance-trading-bot/internal/database"
)

// InitializeUserSettings copies all defaults to a new user
func InitializeUserSettings(ctx context.Context, db *database.Repository, userID string) error {
    defaults, err := autopilot.LoadDefaultSettings()
    if err != nil {
        return fmt.Errorf("failed to load defaults: %w", err)
    }

    log.Printf("[USER-INIT] Initializing settings for user %s from default-settings.json v%s",
        userID, defaults.Metadata.Version)

    // Initialize mode configs
    for modeName, modeConfig := range defaults.ModeConfigs {
        configJSON, err := json.Marshal(modeConfig)
        if err != nil {
            log.Printf("[USER-INIT] Warning: Failed to marshal %s config: %v", modeName, err)
            continue
        }

        err = db.SaveUserModeConfig(ctx, userID, modeName, modeConfig.Enabled, configJSON)
        if err != nil {
            return fmt.Errorf("failed to save %s config: %w", modeName, err)
        }
        log.Printf("[USER-INIT] Initialized mode %s (enabled: %v)", modeName, modeConfig.Enabled)
    }

    // Initialize other settings groups
    // ... circuit breaker, LLM config, etc.

    log.Printf("[USER-INIT] Completed initialization for user %s (%d modes)",
        userID, len(defaults.ModeConfigs))

    return nil
}
```

### Task 2: Settings Diff Service

```go
// internal/autopilot/settings_diff.go

package autopilot

// SettingsDiff represents a difference between user and default value
type SettingsDiff struct {
    Path        string      `json:"path"`
    CurrentVal  interface{} `json:"current"`
    DefaultVal  interface{} `json:"default"`
    RiskInfo    *RiskInfo   `json:"risk_info,omitempty"`
}

// DiffResult contains all differences for a settings group
type DiffResult struct {
    Group        string         `json:"group"`
    TotalChanges int            `json:"total_changes"`
    Differences  []SettingsDiff `json:"differences"`
    AllMatch     bool           `json:"all_match"`
}

// GetModeConfigDiff compares user mode config with defaults
func GetModeConfigDiff(userConfig, defaultConfig *ModeFullConfig) (*DiffResult, error) {
    result := &DiffResult{
        Group:       userConfig.ModeName,
        Differences: []SettingsDiff{},
    }

    // Compare enabled
    if userConfig.Enabled != defaultConfig.Enabled {
        result.Differences = append(result.Differences, SettingsDiff{
            Path:       "enabled",
            CurrentVal: userConfig.Enabled,
            DefaultVal: defaultConfig.Enabled,
            RiskInfo:   defaultConfig.RiskInfo["enabled"],
        })
    }

    // Compare confidence thresholds
    if userConfig.Confidence != nil && defaultConfig.Confidence != nil {
        if userConfig.Confidence.MinConfidence != defaultConfig.Confidence.MinConfidence {
            result.Differences = append(result.Differences, SettingsDiff{
                Path:       "confidence.min_confidence",
                CurrentVal: userConfig.Confidence.MinConfidence,
                DefaultVal: defaultConfig.Confidence.MinConfidence,
            })
        }
        // ... compare other confidence fields
    }

    // ... compare all other sub-sections

    result.TotalChanges = len(result.Differences)
    result.AllMatch = result.TotalChanges == 0

    return result, nil
}
```

### Task 3: Load Defaults API Handlers

```go
// internal/api/handlers_settings_defaults.go

// handleLoadModeDefaults resets a mode to default configuration
// POST /api/futures/ginie/modes/:mode/load-defaults?preview=true
func (s *Server) handleLoadModeDefaults(c *gin.Context) {
    userID := c.GetString("user_id")
    modeName := c.Param("mode")
    previewOnly := c.Query("preview") == "true"

    // Get default config
    defaultConfig, err := autopilot.GetDefaultModeConfig(modeName)
    if err != nil {
        c.JSON(400, gin.H{"error": fmt.Sprintf("Invalid mode: %s", modeName)})
        return
    }

    // Get user's current config
    ctx := context.Background()
    userConfig, err := s.repo.GetUserModeConfig(ctx, userID, modeName)
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to get current config"})
        return
    }

    // Calculate diff
    diff, err := autopilot.GetModeConfigDiff(userConfig, defaultConfig)
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to calculate diff"})
        return
    }

    // Preview only - return diff without applying
    if previewOnly {
        c.JSON(200, gin.H{
            "preview":      true,
            "mode":         modeName,
            "changes":      diff.TotalChanges,
            "differences":  diff.Differences,
            "all_match":    diff.AllMatch,
            "message":      getPreviewMessage(diff),
        })
        return
    }

    // Apply defaults
    if diff.AllMatch {
        c.JSON(200, gin.H{
            "success": true,
            "message": fmt.Sprintf("%s mode already matches defaults", modeName),
            "changes": 0,
        })
        return
    }

    // Save default config to user's database
    configJSON, _ := json.Marshal(defaultConfig)
    err = s.repo.SaveUserModeConfig(ctx, userID, modeName, defaultConfig.Enabled, configJSON)
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to save defaults"})
        return
    }

    // Update in-memory if Ginie is running
    if s.ginieAutopilot != nil {
        s.ginieAutopilot.RefreshModeConfig(modeName)
    }

    log.Printf("[LOAD-DEFAULTS] User %s reset %s mode to defaults (%d changes)",
        userID, modeName, diff.TotalChanges)

    c.JSON(200, gin.H{
        "success":  true,
        "mode":     modeName,
        "changes":  diff.TotalChanges,
        "message":  fmt.Sprintf("Reset %s to defaults", modeName),
        "applied":  diff.Differences,
    })
}

// handleLoadAllModeDefaults resets ALL modes to defaults
// POST /api/futures/ginie/modes/load-defaults?preview=true
func (s *Server) handleLoadAllModeDefaults(c *gin.Context) {
    userID := c.GetString("user_id")
    previewOnly := c.Query("preview") == "true"

    defaults, err := autopilot.LoadDefaultSettings()
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to load defaults"})
        return
    }

    allDiffs := make(map[string]*DiffResult)
    totalChanges := 0

    for modeName, defaultConfig := range defaults.ModeConfigs {
        ctx := context.Background()
        userConfig, _ := s.repo.GetUserModeConfig(ctx, userID, modeName)
        if userConfig != nil {
            diff, _ := autopilot.GetModeConfigDiff(userConfig, defaultConfig)
            if diff != nil && diff.TotalChanges > 0 {
                allDiffs[modeName] = diff
                totalChanges += diff.TotalChanges
            }
        }
    }

    if previewOnly {
        c.JSON(200, gin.H{
            "preview":       true,
            "total_changes": totalChanges,
            "modes":         allDiffs,
        })
        return
    }

    // Apply all defaults
    // ... similar to single mode, but loop through all

    c.JSON(200, gin.H{
        "success":       true,
        "total_changes": totalChanges,
        "message":       "Reset all modes to defaults",
    })
}

func getPreviewMessage(diff *DiffResult) string {
    if diff.AllMatch {
        return "All settings already match defaults - no changes needed"
    }
    return fmt.Sprintf("%d settings will be changed to default values", diff.TotalChanges)
}
```

### Task 4: Register API Routes

```go
// internal/api/server.go - in RegisterRoutes()

// Load Defaults endpoints
futures.POST("/ginie/modes/:mode/load-defaults", s.authMiddleware(), s.handleLoadModeDefaults)
futures.POST("/ginie/modes/load-defaults", s.authMiddleware(), s.handleLoadAllModeDefaults)
futures.POST("/position-optimization/load-defaults", s.authMiddleware(), s.handleLoadPositionOptDefaults)
futures.POST("/circuit-breaker/load-defaults", s.authMiddleware(), s.handleLoadCircuitBreakerDefaults)

// Settings diff endpoints
settings.GET("/diff/modes/:mode", s.authMiddleware(), s.handleGetModeDiff)
settings.GET("/diff/modes", s.authMiddleware(), s.handleGetAllModesDiff)
settings.POST("/load-defaults", s.authMiddleware(), s.handleLoadAllDefaults)
```

### Task 5: Frontend Load Defaults Components

```tsx
// web/src/components/LoadDefaultsButton.tsx

interface LoadDefaultsButtonProps {
  group: string;
  endpoint: string;
  onSuccess?: () => void;
}

export const LoadDefaultsButton: React.FC<LoadDefaultsButtonProps> = ({
  group,
  endpoint,
  onSuccess,
}) => {
  const [showPreview, setShowPreview] = useState(false);
  const [preview, setPreview] = useState<DiffResult | null>(null);
  const [loading, setLoading] = useState(false);

  const fetchPreview = async () => {
    setLoading(true);
    const response = await fetch(`${endpoint}?preview=true`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${token}` },
    });
    const data = await response.json();
    setPreview(data);
    setShowPreview(true);
    setLoading(false);
  };

  const applyDefaults = async () => {
    setLoading(true);
    const response = await fetch(endpoint, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${token}` },
    });
    const data = await response.json();
    if (data.success) {
      toast.success(`Reset ${group} to defaults (${data.changes} changes)`);
      onSuccess?.();
    }
    setShowPreview(false);
    setLoading(false);
  };

  return (
    <>
      <Button onClick={fetchPreview} loading={loading}>
        Load {group} Defaults
      </Button>

      <Dialog open={showPreview} onClose={() => setShowPreview(false)}>
        <DialogTitle>Load Default Settings: {group}</DialogTitle>
        <DialogContent>
          {preview?.all_match ? (
            <Alert severity="info">
              All settings already match defaults - no changes needed.
            </Alert>
          ) : (
            <>
              <Typography>
                {preview?.total_changes} settings will be changed:
              </Typography>
              <Table>
                <TableHead>
                  <TableRow>
                    <TableCell>Setting</TableCell>
                    <TableCell>Current</TableCell>
                    <TableCell>Default</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {preview?.differences.map((diff) => (
                    <TableRow key={diff.path}>
                      <TableCell>{diff.path}</TableCell>
                      <TableCell sx={{ color: 'error.main' }}>
                        {String(diff.current)}
                      </TableCell>
                      <TableCell sx={{ color: 'success.main' }}>
                        {String(diff.default)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setShowPreview(false)}>Cancel</Button>
          {!preview?.all_match && (
            <Button onClick={applyDefaults} color="primary" variant="contained">
              Apply Defaults
            </Button>
          )}
        </DialogActions>
      </Dialog>
    </>
  );
};
```

### Task 6: Integrate into User Registration

```go
// internal/auth/auth_service.go

func (s *AuthService) RegisterUser(ctx context.Context, req RegisterRequest) (*User, error) {
    // ... existing user creation logic ...

    user, err := s.repo.CreateUser(ctx, req.Email, hashedPassword)
    if err != nil {
        return nil, err
    }

    // Initialize settings from defaults
    if err := InitializeUserSettings(ctx, s.repo, user.ID.String()); err != nil {
        log.Printf("[AUTH] Warning: Failed to initialize settings for user %s: %v",
            user.Email, err)
        // Don't fail registration - settings can be initialized later
    }

    return user, nil
}
```

---

## API Reference

### Load Defaults Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/futures/ginie/modes/:mode/load-defaults` | Reset single mode |
| POST | `/api/futures/ginie/modes/load-defaults` | Reset all modes |
| POST | `/api/futures/position-optimization/load-defaults` | Reset position optimization |
| POST | `/api/futures/circuit-breaker/load-defaults` | Reset circuit breaker |
| POST | `/api/user/settings/load-defaults` | Reset ALL settings |

### Query Parameters

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `preview` | boolean | false | If true, returns diff without applying |

### Response Format (Preview)

```json
{
  "preview": true,
  "mode": "scalp",
  "total_changes": 3,
  "all_match": false,
  "differences": [
    {
      "path": "confidence.min_confidence",
      "current": 30,
      "default": 40,
      "risk_info": {
        "impact": "medium",
        "recommendation": "Keep at 40 or higher for safety"
      }
    }
  ],
  "message": "3 settings will be changed to default values"
}
```

### Response Format (Apply)

```json
{
  "success": true,
  "mode": "scalp",
  "changes": 3,
  "message": "Reset scalp to defaults",
  "applied": [
    { "path": "confidence.min_confidence", "old": 30, "new": 40 }
  ]
}
```

---

## Testing Requirements

### Test 1: New User Gets Defaults
```bash
# 1. Register new user
curl -X POST http://localhost:8094/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"newuser@test.com","password":"Test123!"}'

# 2. Login and get token
TOKEN=$(curl -s -X POST http://localhost:8094/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"newuser@test.com","password":"Test123!"}' | jq -r '.token')

# 3. Check mode configs initialized
curl http://localhost:8094/api/futures/ginie/modes \
  -H "Authorization: Bearer $TOKEN" | jq '.modes | length'
# Expected: 5 (all modes initialized)
```

### Test 2: Preview Shows Diff
```bash
# 1. Change scalp min_confidence
curl -X PUT http://localhost:8094/api/futures/ginie/modes/scalp \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"confidence":{"min_confidence":30}}'

# 2. Preview load defaults
curl -X POST "http://localhost:8094/api/futures/ginie/modes/scalp/load-defaults?preview=true" \
  -H "Authorization: Bearer $TOKEN" | jq '.'
# Expected: Shows min_confidence diff (current: 30, default: 40)
```

### Test 3: Apply Defaults
```bash
# Apply without preview
curl -X POST http://localhost:8094/api/futures/ginie/modes/scalp/load-defaults \
  -H "Authorization: Bearer $TOKEN" | jq '.'
# Expected: success=true, changes > 0

# Verify reset
curl http://localhost:8094/api/futures/ginie/modes/scalp \
  -H "Authorization: Bearer $TOKEN" | jq '.confidence.min_confidence'
# Expected: 40 (default value)
```

### Test 4: No Changes When Already Default
```bash
# Load defaults again (should show no changes)
curl -X POST "http://localhost:8094/api/futures/ginie/modes/scalp/load-defaults?preview=true" \
  -H "Authorization: Bearer $TOKEN" | jq '.all_match'
# Expected: true
```

---

## Definition of Done

- [ ] New user registration initializes ALL settings from defaults
- [ ] Load defaults API endpoints implemented for all groups
- [ ] Preview mode shows diff before applying
- [ ] Only changed settings shown in preview
- [ ] UI buttons added for load defaults
- [ ] Confirmation dialog shows preview
- [ ] Success/error toasts displayed
- [ ] All tests pass
- [ ] No regression in trading behavior
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
- **Story 4.15:** Admin Settings Sync (builds on this)
- **Story 4.16:** Settings Comparison & Risk Display (builds on this)
