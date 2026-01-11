# Story 5.3: Global Circuit Breaker Database Integration

**Story ID:** SAFETY-5.3
**Epic:** Epic 5 - Global Safety Controls & Circuit Breaker Enhancement
**Priority:** P1 (High - Safety Critical)
**Estimated Effort:** 8 hours
**Author:** BMAD Agent (Bob - Scrum Master)
**Status:** Ready for Development
**Depends On:** Story 4.1, Story 4.2, Story 4.4

---

## Problem Statement

### Current State

- Global Circuit Breaker limits are **hardcoded** in `DefaultGinieAutopilotConfig()`
- All users share the same safety limits:
  - `MaxLossPerHour = $100`
  - `MaxDailyLoss = $300`
  - `MaxConsecutiveLosses = 3`
  - `CooldownMinutes = 30`
- No way for users to customize global safety limits
- Users with different capital sizes cannot adjust limits
- Changes require code modification and redeployment
- Global CB config not exposed via API

### Expected Behavior

- Global Circuit Breaker limits stored per-user in database
- Users can customize their own safety limits via API
- Frontend allows editing global CB config
- Sensible defaults applied for new users
- Validation prevents unsafe configurations (e.g., $1 daily loss)
- Changes persist across sessions
- Clear separation: Mode CB (per-mode) vs Global CB (account-wide)

### Technical Context

**Two Circuit Breakers Exist:**

1. **Mode Circuit Breaker** (per trading mode)
   - Already in database via `user_mode_configs` table
   - Configurable per-mode (scalp, swing, position, ultra_fast)
   - Status: ✅ Complete

2. **Global Circuit Breaker** (account-wide)
   - Currently hardcoded in `ginie_autopilot.go`
   - Applies to ALL trading modes
   - Status: ❌ This story fixes this

---

## User Story

> As a trader with custom capital allocation,
> I want to configure my own Global Circuit Breaker limits,
> So that the autopilot's safety controls match my account size and risk tolerance.

---

## Design Mockup

### Frontend: Global Circuit Breaker Settings Panel

```
┌─────────────────────────────────────────────────────────────────────┐
│  🛡️ GLOBAL CIRCUIT BREAKER (Account-Wide)                          │
│  Applies to ALL trading modes - prevents runaway losses             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ⚠️ These limits protect your ENTIRE account across all modes      │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  Max Loss Per Hour ($)                                       │   │
│  │  ┌────────────────────────────────────────────────────────┐  │   │
│  │  │ 100                                           [Reset]   │  │   │
│  │  └────────────────────────────────────────────────────────┘  │   │
│  │  If losses exceed this in any 1-hour window, trading stops.│   │
│  │  Current: $100 (default)                                    │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  Max Daily Loss ($)                                          │   │
│  │  ┌────────────────────────────────────────────────────────┐  │   │
│  │  │ 300                                           [Reset]   │  │   │
│  │  └────────────────────────────────────────────────────────┘  │   │
│  │  Daily loss limit across all positions and modes.          │   │
│  │  Current: $300 (default)                                    │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  Max Consecutive Losses                                      │   │
│  │  ┌────────────────────────────────────────────────────────┐  │   │
│  │  │ 3                                             [Reset]   │  │   │
│  │  └────────────────────────────────────────────────────────┘  │   │
│  │  Number of losing trades in a row before cooldown.         │   │
│  │  Current: 3 (default)                                       │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  Cooldown Period (minutes)                                   │   │
│  │  ┌────────────────────────────────────────────────────────┐  │   │
│  │  │ 30                                            [Reset]   │  │   │
│  │  └────────────────────────────────────────────────────────┘  │   │
│  │  How long to pause trading after circuit breaker trips.    │   │
│  │  Current: 30 minutes (default)                              │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ⚠️ Minimum Values (For Your Safety):                               │
│  • Max Loss Per Hour: $50                                           │
│  • Max Daily Loss: $50                                              │
│  • Max Consecutive Losses: 2                                        │
│  • Cooldown Minutes: 10                                             │
│                                                                     │
│  [Save Changes]  [Reset All to Defaults]                            │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Acceptance Criteria

### AC5.3.1: Database Schema for Global CB Config
- [ ] New table `user_global_circuit_breaker` created
- [ ] Columns: `user_id`, `max_loss_per_hour`, `max_daily_loss`, `max_consecutive_losses`, `cooldown_minutes`, `created_at`, `updated_at`
- [ ] Foreign key constraint on `user_id` → `users.id`
- [ ] Default values applied if no row exists for user

### AC5.3.2: Backend Repository Methods
- [ ] `GetUserGlobalCircuitBreakerConfig(userID)` returns user's config or defaults
- [ ] `SaveUserGlobalCircuitBreakerConfig(userID, config)` saves/updates config
- [ ] Validation enforces minimum safe values:
  - `MaxLossPerHour >= $50`
  - `MaxDailyLoss >= $50`
  - `MaxConsecutiveLosses >= 2`
  - `CooldownMinutes >= 10`

### AC5.3.3: API Endpoints
- [ ] `GET /api/user/global-circuit-breaker` returns current config
- [ ] `PUT /api/user/global-circuit-breaker` updates config with validation
- [ ] Returns 400 with clear error if validation fails
- [ ] Returns 200 with updated config on success

### AC5.3.4: Ginie Autopilot Integration
- [ ] `NewGinieAutopilot()` loads Global CB config from database
- [ ] Defaults applied if user has no custom config
- [ ] Config passed to `circuit.NewCircuitBreaker()`
- [ ] Hardcoded values in `DefaultGinieAutopilotConfig()` removed

### AC5.3.5: Frontend Global CB Panel
- [ ] New section in GiniePanel or Settings page for Global CB
- [ ] Four input fields for each setting
- [ ] "Reset to Default" button per setting
- [ ] "Save Changes" button
- [ ] Validation errors displayed if values too low
- [ ] Success toast on save

### AC5.3.6: Changes Persist Across Sessions
- [ ] User sets custom limits (e.g., $500 daily)
- [ ] User logs out and back in
- [ ] Autopilot loads custom limits from database
- [ ] Status panel shows custom limits, not defaults

---

## Technical Implementation

### Task 1: Database Migration

```sql
-- migrations/YYYYMMDDHHMMSS_add_user_global_circuit_breaker.up.sql

CREATE TABLE IF NOT EXISTS user_global_circuit_breaker (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    max_loss_per_hour NUMERIC(10, 2) NOT NULL DEFAULT 100.00,
    max_daily_loss NUMERIC(10, 2) NOT NULL DEFAULT 300.00,
    max_consecutive_losses INTEGER NOT NULL DEFAULT 3,
    cooldown_minutes INTEGER NOT NULL DEFAULT 30,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id)
);

CREATE INDEX idx_user_global_cb_user_id ON user_global_circuit_breaker(user_id);

-- migrations/YYYYMMDDHHMMSS_add_user_global_circuit_breaker.down.sql

DROP TABLE IF EXISTS user_global_circuit_breaker;
```

### Task 2: Repository Methods

```go
// internal/database/repository_user_global_cb.go

package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// UserGlobalCircuitBreakerConfig holds global CB settings per user
type UserGlobalCircuitBreakerConfig struct {
	ID                     int       `db:"id" json:"id"`
	UserID                 string    `db:"user_id" json:"user_id"`
	MaxLossPerHour         float64   `db:"max_loss_per_hour" json:"max_loss_per_hour"`
	MaxDailyLoss           float64   `db:"max_daily_loss" json:"max_daily_loss"`
	MaxConsecutiveLosses   int       `db:"max_consecutive_losses" json:"max_consecutive_losses"`
	CooldownMinutes        int       `db:"cooldown_minutes" json:"cooldown_minutes"`
	CreatedAt              time.Time `db:"created_at" json:"created_at"`
	UpdatedAt              time.Time `db:"updated_at" json:"updated_at"`
}

// GetUserGlobalCircuitBreakerConfig returns user's global CB config or defaults
func (r *Repository) GetUserGlobalCircuitBreakerConfig(ctx context.Context, userID string) (*UserGlobalCircuitBreakerConfig, error) {
	query := `
		SELECT id, user_id, max_loss_per_hour, max_daily_loss,
		       max_consecutive_losses, cooldown_minutes, created_at, updated_at
		FROM user_global_circuit_breaker
		WHERE user_id = $1
	`

	var config UserGlobalCircuitBreakerConfig
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&config.ID,
		&config.UserID,
		&config.MaxLossPerHour,
		&config.MaxDailyLoss,
		&config.MaxConsecutiveLosses,
		&config.CooldownMinutes,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Return defaults if no config exists
		return &UserGlobalCircuitBreakerConfig{
			UserID:               userID,
			MaxLossPerHour:       100.0,
			MaxDailyLoss:         300.0,
			MaxConsecutiveLosses: 3,
			CooldownMinutes:      30,
			CreatedAt:            time.Now(),
			UpdatedAt:            time.Now(),
		}, nil
	}

	if err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveUserGlobalCircuitBreakerConfig saves or updates user's global CB config
func (r *Repository) SaveUserGlobalCircuitBreakerConfig(ctx context.Context, userID string, config *UserGlobalCircuitBreakerConfig) error {
	// Validation
	if err := validateGlobalCBConfig(config); err != nil {
		return err
	}

	query := `
		INSERT INTO user_global_circuit_breaker
			(user_id, max_loss_per_hour, max_daily_loss, max_consecutive_losses, cooldown_minutes, updated_at)
		VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id)
		DO UPDATE SET
			max_loss_per_hour = EXCLUDED.max_loss_per_hour,
			max_daily_loss = EXCLUDED.max_daily_loss,
			max_consecutive_losses = EXCLUDED.max_consecutive_losses,
			cooldown_minutes = EXCLUDED.cooldown_minutes,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.ExecContext(ctx, query,
		userID,
		config.MaxLossPerHour,
		config.MaxDailyLoss,
		config.MaxConsecutiveLosses,
		config.CooldownMinutes,
	)

	return err
}

// validateGlobalCBConfig enforces minimum safe values
func validateGlobalCBConfig(config *UserGlobalCircuitBreakerConfig) error {
	if config.MaxLossPerHour < 50.0 {
		return errors.New("max_loss_per_hour must be at least $50")
	}
	if config.MaxDailyLoss < 50.0 {
		return errors.New("max_daily_loss must be at least $50")
	}
	if config.MaxConsecutiveLosses < 2 {
		return errors.New("max_consecutive_losses must be at least 2")
	}
	if config.CooldownMinutes < 10 {
		return errors.New("cooldown_minutes must be at least 10")
	}
	return nil
}
```

### Task 3: API Handlers

```go
// internal/api/handlers_global_cb.go

package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleGetGlobalCircuitBreakerConfig returns user's global CB config
// GET /api/user/global-circuit-breaker
func (s *Server) handleGetGlobalCircuitBreakerConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	ctx := context.Background()

	config, err := s.repo.GetUserGlobalCircuitBreakerConfig(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load config"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// handleUpdateGlobalCircuitBreakerConfig updates user's global CB config
// PUT /api/user/global-circuit-breaker
func (s *Server) handleUpdateGlobalCircuitBreakerConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	ctx := context.Background()

	var req struct {
		MaxLossPerHour       float64 `json:"max_loss_per_hour"`
		MaxDailyLoss         float64 `json:"max_daily_loss"`
		MaxConsecutiveLosses int     `json:"max_consecutive_losses"`
		CooldownMinutes      int     `json:"cooldown_minutes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	config := &database.UserGlobalCircuitBreakerConfig{
		UserID:               userID,
		MaxLossPerHour:       req.MaxLossPerHour,
		MaxDailyLoss:         req.MaxDailyLoss,
		MaxConsecutiveLosses: req.MaxConsecutiveLosses,
		CooldownMinutes:      req.CooldownMinutes,
	}

	if err := s.repo.SaveUserGlobalCircuitBreakerConfig(ctx, userID, config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Reload config to get timestamps
	updatedConfig, err := s.repo.GetUserGlobalCircuitBreakerConfig(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Config saved but failed to reload"})
		return
	}

	s.logger.Info("Global circuit breaker config updated",
		"user_id", userID,
		"max_loss_per_hour", config.MaxLossPerHour,
		"max_daily_loss", config.MaxDailyLoss,
		"max_consecutive_losses", config.MaxConsecutiveLosses,
		"cooldown_minutes", config.CooldownMinutes)

	c.JSON(http.StatusOK, updatedConfig)
}
```

### Task 4: Register Routes

```go
// internal/api/routes.go

// Add to authenticated routes:
authenticated.GET("/user/global-circuit-breaker", s.handleGetGlobalCircuitBreakerConfig)
authenticated.PUT("/user/global-circuit-breaker", s.handleUpdateGlobalCircuitBreakerConfig)
```

### Task 5: Update Ginie Autopilot to Load from Database

```go
// internal/autopilot/ginie_autopilot.go

// Update NewGinieAutopilot() function:

func NewGinieAutopilot(
	analyzer *GinieAnalyzer,
	futuresClient binance.FuturesClient,
	logger *logging.Logger,
	repo *database.Repository,
	userID string,
) *GinieAutopilot {
	config := DefaultGinieAutopilotConfig()

	// Load user's Global Circuit Breaker config from database
	ctx := context.Background()
	globalCBConfig, err := repo.GetUserGlobalCircuitBreakerConfig(ctx, userID)
	if err != nil {
		logger.Error("Failed to load global CB config, using defaults", "error", err)
	} else {
		// Apply user's custom global CB settings
		config.CBMaxLossPerHour = globalCBConfig.MaxLossPerHour
		config.CBMaxDailyLoss = globalCBConfig.MaxDailyLoss
		config.CBMaxConsecutiveLosses = globalCBConfig.MaxConsecutiveLosses
		config.CBCooldownMinutes = globalCBConfig.CooldownMinutes

		logger.Info("Loaded user's global circuit breaker config",
			"user_id", userID,
			"max_loss_per_hour", globalCBConfig.MaxLossPerHour,
			"max_daily_loss", globalCBConfig.MaxDailyLoss,
			"max_consecutive_losses", globalCBConfig.MaxConsecutiveLosses,
			"cooldown_minutes", globalCBConfig.CooldownMinutes)
	}

	// Create Ginie's circuit breaker with loaded config
	cbConfig := &circuit.CircuitBreakerConfig{
		Enabled:              config.CircuitBreakerEnabled,
		MaxLossPerHour:       config.CBMaxLossPerHour,
		MaxDailyLoss:         config.CBMaxDailyLoss,
		MaxConsecutiveLosses: config.CBMaxConsecutiveLosses,
		CooldownMinutes:      config.CBCooldownMinutes,
		MaxTradesPerMinute:   10,
		MaxDailyTrades:       100,
	}

	// ... rest of function
}
```

### Task 6: Remove Hardcoded Defaults

```go
// internal/autopilot/ginie_autopilot.go

// Update DefaultGinieAutopilotConfig() to use zero values:

func DefaultGinieAutopilotConfig() *GinieAutopilotConfig {
	return &GinieAutopilotConfig{
		// ... other settings ...

		// Circuit breaker - values loaded from database per-user
		CircuitBreakerEnabled:  true,
		CBMaxLossPerHour:       0,  // Loaded from DB
		CBMaxDailyLoss:         0,  // Loaded from DB
		CBMaxConsecutiveLosses: 0,  // Loaded from DB
		CBCooldownMinutes:      0,  // Loaded from DB

		// ... rest of config ...
	}
}
```

### Task 7: Frontend Global CB Settings Panel

```tsx
// web/src/components/GlobalCircuitBreakerPanel.tsx

import React, { useEffect, useState } from 'react';
import {
  Box, Typography, TextField, Button, Alert, Paper, Divider
} from '@mui/material';
import { toast } from 'react-toastify';

interface GlobalCBConfig {
  max_loss_per_hour: number;
  max_daily_loss: number;
  max_consecutive_losses: number;
  cooldown_minutes: number;
}

const DEFAULTS: GlobalCBConfig = {
  max_loss_per_hour: 100,
  max_daily_loss: 300,
  max_consecutive_losses: 3,
  cooldown_minutes: 30,
};

const MINIMUMS: GlobalCBConfig = {
  max_loss_per_hour: 50,
  max_daily_loss: 50,
  max_consecutive_losses: 2,
  cooldown_minutes: 10,
};

export const GlobalCircuitBreakerPanel: React.FC = () => {
  const [config, setConfig] = useState<GlobalCBConfig>(DEFAULTS);
  const [loading, setLoading] = useState(true);
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    fetchConfig();
  }, []);

  const fetchConfig = async () => {
    setLoading(true);
    const response = await fetch('/api/user/global-circuit-breaker', {
      headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
    });
    const data = await response.json();
    setConfig({
      max_loss_per_hour: data.max_loss_per_hour,
      max_daily_loss: data.max_daily_loss,
      max_consecutive_losses: data.max_consecutive_losses,
      cooldown_minutes: data.cooldown_minutes,
    });
    setLoading(false);
  };

  const handleChange = (field: keyof GlobalCBConfig, value: number) => {
    setConfig({ ...config, [field]: value });

    // Validate
    const minValue = MINIMUMS[field];
    if (value < minValue) {
      setErrors({ ...errors, [field]: `Minimum: ${minValue}` });
    } else {
      const { [field]: _, ...rest } = errors;
      setErrors(rest);
    }
  };

  const handleSave = async () => {
    if (Object.keys(errors).length > 0) {
      toast.error('Please fix validation errors');
      return;
    }

    const response = await fetch('/api/user/global-circuit-breaker', {
      method: 'PUT',
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(config),
    });

    if (response.ok) {
      toast.success('Global circuit breaker config saved');
      fetchConfig(); // Reload
    } else {
      const error = await response.json();
      toast.error(error.error || 'Failed to save config');
    }
  };

  const handleReset = (field: keyof GlobalCBConfig) => {
    handleChange(field, DEFAULTS[field]);
  };

  const handleResetAll = () => {
    setConfig(DEFAULTS);
    setErrors({});
  };

  if (loading) return <CircularProgress />;

  return (
    <Paper sx={{ p: 3, mb: 3 }}>
      <Typography variant="h6" gutterBottom>
        🛡️ Global Circuit Breaker (Account-Wide)
      </Typography>
      <Typography variant="body2" color="text.secondary" gutterBottom>
        Applies to ALL trading modes - prevents runaway losses
      </Typography>

      <Divider sx={{ my: 2 }} />

      <Alert severity="warning" sx={{ mb: 3 }}>
        These limits protect your ENTIRE account across all modes
      </Alert>

      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
        {/* Max Loss Per Hour */}
        <Box>
          <Typography variant="subtitle2">Max Loss Per Hour ($)</Typography>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
            <TextField
              type="number"
              value={config.max_loss_per_hour}
              onChange={(e) => handleChange('max_loss_per_hour', parseFloat(e.target.value))}
              error={!!errors.max_loss_per_hour}
              helperText={errors.max_loss_per_hour || 'If losses exceed this in any 1-hour window, trading stops.'}
              fullWidth
            />
            <Button size="small" onClick={() => handleReset('max_loss_per_hour')}>
              Reset
            </Button>
          </Box>
        </Box>

        {/* Max Daily Loss */}
        <Box>
          <Typography variant="subtitle2">Max Daily Loss ($)</Typography>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
            <TextField
              type="number"
              value={config.max_daily_loss}
              onChange={(e) => handleChange('max_daily_loss', parseFloat(e.target.value))}
              error={!!errors.max_daily_loss}
              helperText={errors.max_daily_loss || 'Daily loss limit across all positions and modes.'}
              fullWidth
            />
            <Button size="small" onClick={() => handleReset('max_daily_loss')}>
              Reset
            </Button>
          </Box>
        </Box>

        {/* Max Consecutive Losses */}
        <Box>
          <Typography variant="subtitle2">Max Consecutive Losses</Typography>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
            <TextField
              type="number"
              value={config.max_consecutive_losses}
              onChange={(e) => handleChange('max_consecutive_losses', parseInt(e.target.value))}
              error={!!errors.max_consecutive_losses}
              helperText={errors.max_consecutive_losses || 'Number of losing trades in a row before cooldown.'}
              fullWidth
            />
            <Button size="small" onClick={() => handleReset('max_consecutive_losses')}>
              Reset
            </Button>
          </Box>
        </Box>

        {/* Cooldown Minutes */}
        <Box>
          <Typography variant="subtitle2">Cooldown Period (minutes)</Typography>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
            <TextField
              type="number"
              value={config.cooldown_minutes}
              onChange={(e) => handleChange('cooldown_minutes', parseInt(e.target.value))}
              error={!!errors.cooldown_minutes}
              helperText={errors.cooldown_minutes || 'How long to pause trading after circuit breaker trips.'}
              fullWidth
            />
            <Button size="small" onClick={() => handleReset('cooldown_minutes')}>
              Reset
            </Button>
          </Box>
        </Box>
      </Box>

      <Alert severity="info" sx={{ mt: 3 }}>
        <Typography variant="body2">
          <strong>Minimum Values (For Your Safety):</strong><br />
          • Max Loss Per Hour: ${MINIMUMS.max_loss_per_hour}<br />
          • Max Daily Loss: ${MINIMUMS.max_daily_loss}<br />
          • Max Consecutive Losses: {MINIMUMS.max_consecutive_losses}<br />
          • Cooldown Minutes: {MINIMUMS.cooldown_minutes}
        </Typography>
      </Alert>

      <Box sx={{ display: 'flex', gap: 2, mt: 3 }}>
        <Button variant="contained" onClick={handleSave} disabled={Object.keys(errors).length > 0}>
          Save Changes
        </Button>
        <Button variant="outlined" onClick={handleResetAll}>
          Reset All to Defaults
        </Button>
      </Box>
    </Paper>
  );
};
```

### Task 8: Integrate Panel into GiniePanel

```tsx
// web/src/components/GiniePanel.tsx

import { GlobalCircuitBreakerPanel } from './GlobalCircuitBreakerPanel';

// Add in appropriate section:
<GlobalCircuitBreakerPanel />
```

---

## API Reference

### Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/user/global-circuit-breaker` | Get user's global CB config |
| PUT | `/api/user/global-circuit-breaker` | Update user's global CB config |

### Request: Update Global CB Config

```json
{
  "max_loss_per_hour": 150.0,
  "max_daily_loss": 500.0,
  "max_consecutive_losses": 5,
  "cooldown_minutes": 45
}
```

### Response: Global CB Config

```json
{
  "id": 42,
  "user_id": "user123",
  "max_loss_per_hour": 150.0,
  "max_daily_loss": 500.0,
  "max_consecutive_losses": 5,
  "cooldown_minutes": 45,
  "created_at": "2026-01-05T10:00:00Z",
  "updated_at": "2026-01-06T12:30:00Z"
}
```

### Error Response: Validation Failed

```json
{
  "error": "max_daily_loss must be at least $50"
}
```

---

## Testing Requirements

### Test 1: Default Values Applied for New User

```bash
# Get config for user with no custom settings
curl http://localhost:8094/api/user/global-circuit-breaker \
  -H "Authorization: Bearer $TOKEN" | jq

# Expected:
{
  "user_id": "user123",
  "max_loss_per_hour": 100.0,
  "max_daily_loss": 300.0,
  "max_consecutive_losses": 3,
  "cooldown_minutes": 30
}
```

### Test 2: Save Custom Config

```bash
# Save custom limits
curl -X PUT http://localhost:8094/api/user/global-circuit-breaker \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "max_loss_per_hour": 200,
    "max_daily_loss": 600,
    "max_consecutive_losses": 5,
    "cooldown_minutes": 60
  }' | jq

# Expected: 200 OK with updated config
```

### Test 3: Validation Prevents Unsafe Values

```bash
# Try to save dangerously low limits
curl -X PUT http://localhost:8094/api/user/global-circuit-breaker \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "max_loss_per_hour": 10,
    "max_daily_loss": 20,
    "max_consecutive_losses": 1,
    "cooldown_minutes": 5
  }'

# Expected: 400 Bad Request with validation errors
{
  "error": "max_loss_per_hour must be at least $50"
}
```

### Test 4: Config Persists Across Sessions

```bash
# Save custom config
curl -X PUT http://localhost:8094/api/user/global-circuit-breaker \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"max_daily_loss": 500, "max_loss_per_hour": 150, "max_consecutive_losses": 4, "cooldown_minutes": 45}'

# Restart Ginie autopilot (simulate logout/login)
curl -X POST http://localhost:8094/api/futures/ginie/stop \
  -H "Authorization: Bearer $TOKEN"

curl -X POST http://localhost:8094/api/futures/ginie/start \
  -H "Authorization: Bearer $TOKEN"

# Check Ginie status - should show custom limits
curl http://localhost:8094/api/futures/ginie/status \
  -H "Authorization: Bearer $TOKEN" | jq '.circuit_breaker'

# Expected: Custom limits, not defaults
```

### Test 5: Frontend Integration

1. Open `/ginie` page
2. Navigate to Global Circuit Breaker section
3. Change `Max Daily Loss` to `$500`
4. Click "Save Changes"
5. Refresh page
6. Verify: `$500` still displayed (persisted)
7. Click "Reset All to Defaults"
8. Verify: All fields return to default values
9. Click "Save Changes"
10. Verify: Defaults saved to database

---

## Definition of Done

- [ ] Migration creates `user_global_circuit_breaker` table
- [ ] Repository methods: `GetUserGlobalCircuitBreakerConfig`, `SaveUserGlobalCircuitBreakerConfig`
- [ ] Validation enforces minimum safe values
- [ ] API endpoints registered and functional
- [ ] Ginie autopilot loads config from database per-user
- [ ] Defaults applied if no custom config exists
- [ ] Frontend panel allows editing all four settings
- [ ] "Reset to Default" buttons work
- [ ] "Save Changes" persists to database
- [ ] Validation errors displayed in UI
- [ ] Changes persist across sessions
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

- **Story 4.1:** Database Schema Foundation (provides database structure)
- **Story 4.2:** Repository Layer Implementation (provides repository patterns)
- **Story 4.4:** Backend API Handlers (provides API patterns)
- **Story 5.1:** Epic 5 - Global Safety Controls (parent epic)
- **Story 5.2:** Mode Circuit Breaker Integration (related but separate CB)
