# Story 4.16: Settings Comparison & Risk Display

**Story ID:** SETTINGS-4.16
**Epic:** Epic 4 - Database-First Mode Configuration System
**Priority:** P2 (Medium - User Experience Enhancement)
**Estimated Effort:** 6 hours
**Author:** BMAD Agent (Bob - Scrum Master)
**Status:** Ready for Development
**Depends On:** Story 4.13, Story 4.14

---

## Problem Statement

### Current State

- Users modify settings without understanding the impact
- No way to see what differs from recommended defaults
- No risk warnings for dangerous configurations
- Users don't know if they've deviated from safe settings

### Expected Behavior

- Dedicated page showing user's settings vs defaults
- Only display CHANGED settings (not everything)
- Each changed setting shows risk level and recommendation
- Grouped by setting category for easy navigation
- Quick action to reset individual settings or groups

---

## User Story

> As a trader who has modified my settings,
> I want to see a clear comparison of my settings vs the recommended defaults,
> So that I understand the risks of my customizations and can make informed decisions.

---

## Design Mockup

```
┌─────────────────────────────────────────────────────────────────────┐
│  ⚙️ YOUR SETTINGS vs RECOMMENDED DEFAULTS                           │
│  Last compared: 2026-01-05 12:00 UTC                                │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  📊 SUMMARY                                                         │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  Total Changes: 8                                            │   │
│  │  🔴 High Risk: 2    🟡 Medium Risk: 3    🟢 Low Risk: 3     │   │
│  │                                                              │   │
│  │  [Reset All to Defaults]                                     │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ─────────────────────────────────────────────────────────────────  │
│                                                                     │
│  📈 MODE CONFIGS (4 changes)                     [Reset Group]      │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  🔴 ultra_fast.enabled                                       │   │
│  │     YOUR VALUE: true                                         │   │
│  │     DEFAULT:    false                                        │   │
│  │     ⚠️ HIGH RISK: Ultra-fast mode has high loss potential.   │   │
│  │     💡 Keep disabled until you have significant experience.  │   │
│  │     [Reset to Default]                                       │   │
│  │                                                              │   │
│  │  🟡 scalp.confidence.min_confidence                          │   │
│  │     YOUR VALUE: 30                                           │   │
│  │     DEFAULT:    40                                           │   │
│  │     ⚠️ MEDIUM RISK: Lower confidence = more trades, more     │   │
│  │        risk exposure.                                        │   │
│  │     💡 Keep at 40+ for balanced risk/reward.                 │   │
│  │     [Reset to Default]                                       │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  🛡️ CIRCUIT BREAKER (2 changes)                  [Reset Group]      │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  🔴 circuit_breaker.enabled                                  │   │
│  │     YOUR VALUE: false                                        │   │
│  │     DEFAULT:    true                                         │   │
│  │     ⚠️ HIGH RISK: Disabling circuit breaker removes loss     │   │
│  │        protection. Unlimited losses possible!                │   │
│  │     💡 STRONGLY RECOMMENDED: Keep enabled at all times.      │   │
│  │     [Reset to Default]                                       │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  🤖 LLM CONFIG (2 changes)                       [Reset Group]      │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  🟢 llm_config.timeout_ms                                    │   │
│  │     YOUR VALUE: 3000                                         │   │
│  │     DEFAULT:    5000                                         │   │
│  │     ℹ️ LOW RISK: Shorter timeout may skip LLM analysis       │   │
│  │        in volatile markets.                                  │   │
│  │     💡 5000ms recommended for thorough analysis.             │   │
│  │     [Reset to Default]                                       │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ✅ POSITION OPTIMIZATION (no changes)                              │
│     All settings match defaults.                                    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Acceptance Criteria

### AC4.16.1: Settings Comparison API
- [ ] `GET /api/user/settings/comparison` returns full comparison
- [ ] Only changed settings included (not all 500+)
- [ ] Each change includes: path, current, default, risk_info
- [ ] Grouped by category (mode_configs, circuit_breaker, etc.)
- [ ] Summary with total changes and risk breakdown

### AC4.16.2: Risk Information Display
- [ ] High risk (🔴): Dangerous configurations
- [ ] Medium risk (🟡): Suboptimal but manageable
- [ ] Low risk (🟢): Minor deviations, safe
- [ ] Each setting shows impact and recommendation from `_risk_info`

### AC4.16.3: Settings Comparison Page
- [ ] New page: `/settings/comparison` or `/user/settings`
- [ ] Shows only groups with changes
- [ ] Expandable/collapsible groups
- [ ] Summary section at top with risk counts

### AC4.16.4: Individual Reset Actions
- [ ] "Reset to Default" button per setting
- [ ] Confirmation before reset
- [ ] Success toast with setting name

### AC4.16.5: Group Reset Actions
- [ ] "Reset Group" button per category
- [ ] Shows preview of all settings in group that will change
- [ ] Confirmation dialog before applying

### AC4.16.6: All Match State
- [ ] If all settings match defaults, show success message
- [ ] "All your settings match the recommended defaults!"
- [ ] No reset buttons needed

---

## Technical Implementation

### Task 1: Comprehensive Comparison API

```go
// internal/api/handlers_settings_comparison.go

// SettingComparison represents a single setting difference
type SettingComparison struct {
    Path        string      `json:"path"`
    Group       string      `json:"group"`
    CurrentVal  interface{} `json:"current"`
    DefaultVal  interface{} `json:"default"`
    RiskLevel   string      `json:"risk_level"`   // high, medium, low
    Impact      string      `json:"impact"`
    Recommendation string   `json:"recommendation"`
}

// ComparisonResult is the full comparison response
type ComparisonResult struct {
    Timestamp      string                           `json:"timestamp"`
    TotalChanges   int                              `json:"total_changes"`
    HighRiskCount  int                              `json:"high_risk_count"`
    MediumRiskCount int                             `json:"medium_risk_count"`
    LowRiskCount   int                              `json:"low_risk_count"`
    AllMatch       bool                             `json:"all_match"`
    Groups         map[string]*GroupComparison      `json:"groups"`
}

// GroupComparison represents differences in one category
type GroupComparison struct {
    GroupName   string               `json:"group_name"`
    DisplayName string               `json:"display_name"`
    ChangeCount int                  `json:"change_count"`
    Differences []SettingComparison  `json:"differences"`
}

// handleGetSettingsComparison returns full settings comparison
// GET /api/user/settings/comparison
func (s *Server) handleGetSettingsComparison(c *gin.Context) {
    userID := c.GetString("user_id")
    ctx := context.Background()

    // Load defaults
    defaults, err := autopilot.LoadDefaultSettings()
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to load defaults"})
        return
    }

    result := &ComparisonResult{
        Timestamp: time.Now().UTC().Format(time.RFC3339),
        Groups:    make(map[string]*GroupComparison),
    }

    // Compare mode configs
    modeGroup := &GroupComparison{
        GroupName:   "mode_configs",
        DisplayName: "Mode Configurations",
        Differences: []SettingComparison{},
    }

    for modeName, defaultConfig := range defaults.ModeConfigs {
        userConfig, err := s.repo.GetUserModeConfig(ctx, userID, modeName)
        if err != nil {
            continue
        }

        // Compare each field
        diffs := compareModeConfigs(modeName, userConfig, defaultConfig)
        modeGroup.Differences = append(modeGroup.Differences, diffs...)
    }
    modeGroup.ChangeCount = len(modeGroup.Differences)
    if modeGroup.ChangeCount > 0 {
        result.Groups["mode_configs"] = modeGroup
    }

    // Compare circuit breaker
    // ... similar logic

    // Compare LLM config
    // ... similar logic

    // Count by risk level
    for _, group := range result.Groups {
        for _, diff := range group.Differences {
            result.TotalChanges++
            switch diff.RiskLevel {
            case "high":
                result.HighRiskCount++
            case "medium":
                result.MediumRiskCount++
            case "low":
                result.LowRiskCount++
            }
        }
    }

    result.AllMatch = result.TotalChanges == 0

    c.JSON(200, result)
}

// compareModeConfigs compares user vs default mode config
func compareModeConfigs(modeName string, user, def *autopilot.ModeFullConfig) []SettingComparison {
    var diffs []SettingComparison

    // Compare enabled
    if user.Enabled != def.Enabled {
        riskInfo := def.GetRiskInfo("enabled")
        diffs = append(diffs, SettingComparison{
            Path:           fmt.Sprintf("%s.enabled", modeName),
            Group:          "mode_configs",
            CurrentVal:     user.Enabled,
            DefaultVal:     def.Enabled,
            RiskLevel:      riskInfo.GetLevel(),
            Impact:         riskInfo.Impact,
            Recommendation: riskInfo.Recommendation,
        })
    }

    // Compare confidence
    if user.Confidence != nil && def.Confidence != nil {
        if user.Confidence.MinConfidence != def.Confidence.MinConfidence {
            diffs = append(diffs, SettingComparison{
                Path:           fmt.Sprintf("%s.confidence.min_confidence", modeName),
                Group:          "mode_configs",
                CurrentVal:     user.Confidence.MinConfidence,
                DefaultVal:     def.Confidence.MinConfidence,
                RiskLevel:      "medium",
                Impact:         "Lower confidence allows more trades with higher risk",
                Recommendation: "Keep at default or higher for safety",
            })
        }
        // ... compare other confidence fields
    }

    // Compare size
    if user.Size != nil && def.Size != nil {
        if user.Size.Leverage != def.Size.Leverage {
            riskLevel := "low"
            if user.Size.Leverage > def.Size.Leverage {
                riskLevel = "high"
            }
            diffs = append(diffs, SettingComparison{
                Path:           fmt.Sprintf("%s.size.leverage", modeName),
                Group:          "mode_configs",
                CurrentVal:     user.Size.Leverage,
                DefaultVal:     def.Size.Leverage,
                RiskLevel:      riskLevel,
                Impact:         "Higher leverage increases both profit and loss potential",
                Recommendation: "Use lower leverage for safer trading",
            })
        }
        // ... compare other size fields
    }

    // ... compare all other sub-sections

    return diffs
}
```

### Task 2: Reset Individual Setting API

```go
// handleResetSingleSetting resets one specific setting to default
// POST /api/user/settings/reset
func (s *Server) handleResetSingleSetting(c *gin.Context) {
    userID := c.GetString("user_id")
    ctx := context.Background()

    var req struct {
        Path string `json:"path"` // e.g., "scalp.confidence.min_confidence"
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "Invalid request"})
        return
    }

    // Parse path to get group and setting
    parts := strings.SplitN(req.Path, ".", 2)
    if len(parts) < 2 {
        c.JSON(400, gin.H{"error": "Invalid setting path"})
        return
    }

    modeName := parts[0]
    settingPath := parts[1]

    // Get default value
    defaultConfig, err := autopilot.GetDefaultModeConfig(modeName)
    if err != nil {
        c.JSON(400, gin.H{"error": "Invalid mode"})
        return
    }

    // Get user's current config
    userConfig, err := s.repo.GetUserModeConfig(ctx, userID, modeName)
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to get current config"})
        return
    }

    // Apply default value to specific path
    oldValue, newValue, err := applyDefaultToPath(userConfig, defaultConfig, settingPath)
    if err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // Save updated config
    configJSON, _ := json.Marshal(userConfig)
    err = s.repo.SaveUserModeConfig(ctx, userID, modeName, userConfig.Enabled, configJSON)
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to save config"})
        return
    }

    log.Printf("[SETTINGS-RESET] User %s reset %s from %v to %v",
        userID, req.Path, oldValue, newValue)

    c.JSON(200, gin.H{
        "success":   true,
        "path":      req.Path,
        "old_value": oldValue,
        "new_value": newValue,
        "message":   fmt.Sprintf("Reset %s to default", req.Path),
    })
}
```

### Task 3: Frontend Settings Comparison Page

```tsx
// web/src/pages/SettingsComparison.tsx

import React, { useEffect, useState } from 'react';
import {
  Box, Typography, Card, CardContent, Chip, Button,
  Accordion, AccordionSummary, AccordionDetails,
  Alert, AlertTitle, IconButton, Tooltip
} from '@mui/material';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import WarningIcon from '@mui/icons-material/Warning';
import ErrorIcon from '@mui/icons-material/Error';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import RestoreIcon from '@mui/icons-material/Restore';

interface SettingComparison {
  path: string;
  group: string;
  current: any;
  default: any;
  risk_level: 'high' | 'medium' | 'low';
  impact: string;
  recommendation: string;
}

interface GroupComparison {
  group_name: string;
  display_name: string;
  change_count: number;
  differences: SettingComparison[];
}

interface ComparisonResult {
  timestamp: string;
  total_changes: number;
  high_risk_count: number;
  medium_risk_count: number;
  low_risk_count: number;
  all_match: boolean;
  groups: Record<string, GroupComparison>;
}

const RiskIcon: React.FC<{ level: string }> = ({ level }) => {
  switch (level) {
    case 'high':
      return <ErrorIcon sx={{ color: 'error.main' }} />;
    case 'medium':
      return <WarningIcon sx={{ color: 'warning.main' }} />;
    default:
      return <CheckCircleIcon sx={{ color: 'success.main' }} />;
  }
};

const RiskChip: React.FC<{ level: string }> = ({ level }) => {
  const colors: Record<string, 'error' | 'warning' | 'success'> = {
    high: 'error',
    medium: 'warning',
    low: 'success',
  };
  return (
    <Chip
      size="small"
      label={level.toUpperCase()}
      color={colors[level] || 'default'}
      sx={{ ml: 1 }}
    />
  );
};

export const SettingsComparison: React.FC = () => {
  const [comparison, setComparison] = useState<ComparisonResult | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchComparison();
  }, []);

  const fetchComparison = async () => {
    setLoading(true);
    const response = await fetch('/api/user/settings/comparison', {
      headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
    });
    const data = await response.json();
    setComparison(data);
    setLoading(false);
  };

  const handleResetSetting = async (path: string) => {
    if (!confirm(`Reset ${path} to default?`)) return;

    const response = await fetch('/api/user/settings/reset', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ path }),
    });

    if (response.ok) {
      toast.success(`Reset ${path} to default`);
      fetchComparison(); // Refresh
    }
  };

  const handleResetGroup = async (group: string) => {
    if (!confirm(`Reset all ${group} settings to defaults?`)) return;

    const response = await fetch(`/api/user/settings/${group}/load-defaults`, {
      method: 'POST',
      headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
    });

    if (response.ok) {
      toast.success(`Reset ${group} to defaults`);
      fetchComparison();
    }
  };

  if (loading) return <CircularProgress />;

  if (comparison?.all_match) {
    return (
      <Box sx={{ p: 3 }}>
        <Alert severity="success">
          <AlertTitle>All Settings Match Defaults</AlertTitle>
          Your settings are configured according to the recommended defaults.
          No changes detected.
        </Alert>
      </Box>
    );
  }

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h4" gutterBottom>
        Settings vs Recommended Defaults
      </Typography>
      <Typography variant="body2" color="text.secondary" gutterBottom>
        Last compared: {new Date(comparison?.timestamp || '').toLocaleString()}
      </Typography>

      {/* Summary Card */}
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Typography variant="h6">Summary</Typography>
          <Box sx={{ display: 'flex', gap: 2, mt: 2, alignItems: 'center' }}>
            <Typography>Total Changes: <strong>{comparison?.total_changes}</strong></Typography>
            <Chip icon={<ErrorIcon />} label={`${comparison?.high_risk_count} High Risk`} color="error" />
            <Chip icon={<WarningIcon />} label={`${comparison?.medium_risk_count} Medium`} color="warning" />
            <Chip icon={<CheckCircleIcon />} label={`${comparison?.low_risk_count} Low`} color="success" />
          </Box>
          <Button
            variant="outlined"
            startIcon={<RestoreIcon />}
            sx={{ mt: 2 }}
            onClick={() => handleResetGroup('all')}
          >
            Reset All to Defaults
          </Button>
        </CardContent>
      </Card>

      {/* Groups */}
      {Object.entries(comparison?.groups || {}).map(([key, group]) => (
        <Accordion key={key} defaultExpanded={group.change_count <= 5}>
          <AccordionSummary expandIcon={<ExpandMoreIcon />}>
            <Box sx={{ display: 'flex', alignItems: 'center', width: '100%', justifyContent: 'space-between' }}>
              <Typography variant="h6">
                {group.display_name} ({group.change_count} changes)
              </Typography>
              <Button
                size="small"
                startIcon={<RestoreIcon />}
                onClick={(e) => { e.stopPropagation(); handleResetGroup(key); }}
              >
                Reset Group
              </Button>
            </Box>
          </AccordionSummary>
          <AccordionDetails>
            {group.differences.map((diff, idx) => (
              <Card key={idx} sx={{ mb: 2, borderLeft: 4, borderColor: diff.risk_level === 'high' ? 'error.main' : diff.risk_level === 'medium' ? 'warning.main' : 'success.main' }}>
                <CardContent>
                  <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
                    <RiskIcon level={diff.risk_level} />
                    <Typography variant="subtitle1" sx={{ ml: 1, fontWeight: 'bold' }}>
                      {diff.path}
                    </Typography>
                    <RiskChip level={diff.risk_level} />
                  </Box>

                  <Box sx={{ display: 'flex', gap: 4, mb: 2 }}>
                    <Box>
                      <Typography variant="body2" color="text.secondary">Your Value</Typography>
                      <Typography sx={{ color: 'error.main', fontWeight: 'bold' }}>
                        {String(diff.current)}
                      </Typography>
                    </Box>
                    <Box>
                      <Typography variant="body2" color="text.secondary">Default</Typography>
                      <Typography sx={{ color: 'success.main', fontWeight: 'bold' }}>
                        {String(diff.default)}
                      </Typography>
                    </Box>
                  </Box>

                  <Alert severity={diff.risk_level === 'high' ? 'error' : diff.risk_level === 'medium' ? 'warning' : 'info'} sx={{ mb: 2 }}>
                    <AlertTitle>Impact</AlertTitle>
                    {diff.impact}
                  </Alert>

                  <Typography variant="body2" color="text.secondary">
                    <strong>Recommendation:</strong> {diff.recommendation}
                  </Typography>

                  <Button
                    size="small"
                    startIcon={<RestoreIcon />}
                    sx={{ mt: 2 }}
                    onClick={() => handleResetSetting(diff.path)}
                  >
                    Reset to Default
                  </Button>
                </CardContent>
              </Card>
            ))}
          </AccordionDetails>
        </Accordion>
      ))}
    </Box>
  );
};
```

### Task 4: Add Navigation Link

```tsx
// In sidebar or user menu
<ListItem button component={Link} to="/settings/comparison">
  <ListItemIcon><CompareIcon /></ListItemIcon>
  <ListItemText primary="Settings vs Defaults" />
  {comparison?.high_risk_count > 0 && (
    <Badge badgeContent={comparison.high_risk_count} color="error" />
  )}
</ListItem>
```

---

## API Reference

### Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/user/settings/comparison` | Get full settings comparison |
| POST | `/api/user/settings/reset` | Reset single setting to default |
| POST | `/api/user/settings/:group/load-defaults` | Reset entire group |

### Response: Settings Comparison

```json
{
  "timestamp": "2026-01-05T12:00:00Z",
  "total_changes": 8,
  "high_risk_count": 2,
  "medium_risk_count": 3,
  "low_risk_count": 3,
  "all_match": false,
  "groups": {
    "mode_configs": {
      "group_name": "mode_configs",
      "display_name": "Mode Configurations",
      "change_count": 4,
      "differences": [
        {
          "path": "ultra_fast.enabled",
          "group": "mode_configs",
          "current": true,
          "default": false,
          "risk_level": "high",
          "impact": "Ultra-fast mode has high loss potential",
          "recommendation": "Keep disabled until experienced"
        }
      ]
    }
  }
}
```

---

## Testing Requirements

### Test 1: Comparison Shows Only Changes
```bash
# Make one change
curl -X PUT http://localhost:8094/api/futures/ginie/modes/scalp \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"confidence":{"min_confidence":30}}'

# Get comparison
curl http://localhost:8094/api/user/settings/comparison \
  -H "Authorization: Bearer $TOKEN" | jq '.total_changes'
# Expected: 1 (or more if other changes exist)

# Verify only changed setting in response
curl http://localhost:8094/api/user/settings/comparison \
  -H "Authorization: Bearer $TOKEN" | jq '.groups.mode_configs.differences[0].path'
# Expected: "scalp.confidence.min_confidence"
```

### Test 2: Reset Single Setting
```bash
# Reset the changed setting
curl -X POST http://localhost:8094/api/user/settings/reset \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"path":"scalp.confidence.min_confidence"}'

# Verify comparison shows one less change
curl http://localhost:8094/api/user/settings/comparison \
  -H "Authorization: Bearer $TOKEN" | jq '.total_changes'
# Expected: 0 (if that was the only change)
```

### Test 3: All Match State
```bash
# Load all defaults
curl -X POST http://localhost:8094/api/user/settings/load-defaults \
  -H "Authorization: Bearer $TOKEN"

# Comparison should show all_match
curl http://localhost:8094/api/user/settings/comparison \
  -H "Authorization: Bearer $TOKEN" | jq '.all_match'
# Expected: true
```

---

## Definition of Done

- [ ] Comparison API returns only changed settings
- [ ] Risk levels correctly categorized (high/medium/low)
- [ ] Impact and recommendation displayed for each change
- [ ] Settings grouped by category
- [ ] Summary shows risk count breakdown
- [ ] "Reset to Default" button works per setting
- [ ] "Reset Group" button works per category
- [ ] "All Match" message when no differences
- [ ] Settings comparison page renders correctly
- [ ] Navigation link added to sidebar
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
- **Story 4.14:** New User & Load Defaults (provides reset APIs)
- **Story 4.15:** Admin Settings Sync (admin can update defaults)
