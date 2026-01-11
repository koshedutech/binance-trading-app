# Story 4.12: Position Optimizer Database-First Migration

**Story ID:** POS-OPT-4.12
**Epic:** Epic 4 - Database-First Mode Configuration System
**Priority:** P1 (High - Foundation for Position Enhancement Features)
**Estimated Effort:** 6-8 hours
**Author:** Bob (Scrum Master) via BMAD Party Mode
**Status:** Ready for Development

---

## Background

### Current State
The `scalp_reentry` feature provides progressive take-profit management with re-entry at breakeven. Currently:
- Configuration stored in JSON file (`autopilot_settings.json`)
- Only applies to SCALP mode positions
- Database table exists (`user_mode_configs`) but is NOT used for re-entry config
- No per-user configuration support

### Future Vision
This module will be renamed to `position_optimizer` and expanded to:
1. **Re-entry for ALL modes** (ultra_fast, scalp, swing, position)
2. **Hedge order integration** (already developed, will integrate later)
3. **Per-user, per-mode configurations**
4. **Database as single source of truth**

---

## User Story

> As a trader,
> When I configure re-entry settings in the UI,
> I expect those settings to be saved to the database (not JSON),
> So that my per-user configuration persists correctly and works in a multi-user environment.

---

## Scope

### In Scope (This Story)
- [x] Rename `scalp_reentry` → `position_optimizer` in database schema
- [x] Create database tables for position optimizer config
- [x] Migrate API handlers to database-first approach
- [x] Per-mode re-entry enable/disable toggles
- [x] Per-user configuration support
- [x] Update existing code references

### Out of Scope (Future Stories)
- [ ] Hedge order integration (already developed separately, integrate later)
- [ ] UI redesign for multi-mode re-entry
- [ ] Adaptive learning database storage
- [ ] Multi-agent configuration per mode

---

## Technical Design

### 1. Database Schema

#### New Table: `user_position_optimizer_configs`

```sql
-- Per-mode re-entry configuration for each user
CREATE TABLE user_position_optimizer_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode_name VARCHAR(20) NOT NULL CHECK (mode_name IN ('ultra_fast', 'scalp', 'swing', 'position')),
    enabled BOOLEAN NOT NULL DEFAULT false,

    -- TP Levels (mode-specific defaults)
    tp1_percent DECIMAL(5,2) NOT NULL DEFAULT 0.4,
    tp1_sell_percent DECIMAL(5,2) NOT NULL DEFAULT 30,
    tp2_percent DECIMAL(5,2) NOT NULL DEFAULT 0.7,
    tp2_sell_percent DECIMAL(5,2) NOT NULL DEFAULT 50,
    tp3_percent DECIMAL(5,2) NOT NULL DEFAULT 1.0,
    tp3_sell_percent DECIMAL(5,2) NOT NULL DEFAULT 80,

    -- Re-entry settings
    reentry_percent DECIMAL(5,2) NOT NULL DEFAULT 80,
    reentry_price_buffer DECIMAL(5,3) NOT NULL DEFAULT 0.15,
    max_reentry_attempts INT NOT NULL DEFAULT 3,
    reentry_timeout_sec INT NOT NULL DEFAULT 900,

    -- Final portion trailing
    final_trailing_percent DECIMAL(5,2) NOT NULL DEFAULT 5.0,
    final_hold_min_percent DECIMAL(5,2) NOT NULL DEFAULT 20,

    -- Dynamic SL
    dynamic_sl_enabled BOOLEAN NOT NULL DEFAULT true,
    dynamic_sl_max_loss_pct DECIMAL(5,2) NOT NULL DEFAULT 40,
    dynamic_sl_protect_pct DECIMAL(5,2) NOT NULL DEFAULT 60,
    dynamic_sl_update_interval_sec INT NOT NULL DEFAULT 30,

    -- Risk limits
    max_cycles_per_position INT NOT NULL DEFAULT 10,
    stop_loss_percent DECIMAL(5,2) NOT NULL DEFAULT 1.5,

    -- Extended config (JSONB for flexibility)
    config_json JSONB DEFAULT '{}',

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(user_id, mode_name)
);

-- Indexes
CREATE INDEX idx_pos_opt_user_id ON user_position_optimizer_configs(user_id);
CREATE INDEX idx_pos_opt_mode ON user_position_optimizer_configs(mode_name);
CREATE INDEX idx_pos_opt_enabled ON user_position_optimizer_configs(user_id, enabled);
```

#### New Table: `user_position_optimizer_global`

```sql
-- Global/shared settings per user (AI, adaptive learning, etc.)
CREATE TABLE user_position_optimizer_global (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,

    -- Master toggle
    global_enabled BOOLEAN NOT NULL DEFAULT true,

    -- AI Configuration
    use_ai_decisions BOOLEAN NOT NULL DEFAULT true,
    ai_min_confidence DECIMAL(3,2) NOT NULL DEFAULT 0.65,
    ai_tp_optimization BOOLEAN NOT NULL DEFAULT true,
    ai_dynamic_sl BOOLEAN NOT NULL DEFAULT true,

    -- Multi-agent
    use_multi_agent BOOLEAN NOT NULL DEFAULT true,
    enable_sentiment_agent BOOLEAN NOT NULL DEFAULT true,
    enable_risk_agent BOOLEAN NOT NULL DEFAULT true,
    enable_tp_agent BOOLEAN NOT NULL DEFAULT true,

    -- Adaptive learning
    enable_adaptive_learning BOOLEAN NOT NULL DEFAULT true,
    adaptive_window_trades INT NOT NULL DEFAULT 20,
    adaptive_min_trades INT NOT NULL DEFAULT 10,
    adaptive_max_reentry_adjust DECIMAL(5,2) NOT NULL DEFAULT 20,

    -- Daily limits
    max_daily_reentries INT NOT NULL DEFAULT 50,
    min_position_size_usd DECIMAL(10,2) NOT NULL DEFAULT 10,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 2. Default TP Levels Per Mode

| Mode | TP1% | TP2% | TP3% | Rationale |
|------|------|------|------|-----------|
| `ultra_fast` | 0.2 | 0.4 | 0.6 | Very quick trades, small targets |
| `scalp` | 0.4 | 0.7 | 1.0 | Current scalp_reentry defaults |
| `swing` | 1.5 | 3.0 | 5.0 | Larger moves, longer holds |
| `position` | 3.0 | 6.0 | 10.0 | Major trends, big targets |

### 3. File Renames

| Current File | New File |
|--------------|----------|
| `internal/autopilot/scalp_reentry_types.go` | `internal/autopilot/position_optimizer_types.go` |
| `internal/autopilot/scalp_reentry_logic.go` | `internal/autopilot/position_optimizer_logic.go` |
| `internal/autopilot/scalp_reentry_agents.go` | `internal/autopilot/position_optimizer_agents.go` |
| `internal/autopilot/scalp_reentry_learning.go` | `internal/autopilot/position_optimizer_learning.go` |
| `internal/ai/llm/scalp_reentry_prompts.go` | `internal/ai/llm/position_optimizer_prompts.go` |
| `web/src/components/ScalpReentryMonitor.tsx` | `web/src/components/PositionOptimizerMonitor.tsx` |

### 4. Type Renames

| Current Type | New Type |
|--------------|----------|
| `ScalpReentryConfig` | `PositionOptimizerConfig` |
| `ScalpReentryStatus` | `PositionOptimizerStatus` |
| `ScalpReentryMarketData` | `PositionOptimizerMarketData` |
| `ReentryCycle` | `OptimizationCycle` |
| `ScalpReentryOrchestrator` | `PositionOptimizerOrchestrator` |

### 5. API Endpoint Changes

| Current Endpoint | New Endpoint |
|------------------|--------------|
| `GET /api/futures/ginie/scalp-reentry-config` | `GET /api/futures/ginie/position-optimizer/config` |
| `POST /api/futures/ginie/scalp-reentry-config` | `POST /api/futures/ginie/position-optimizer/config` |
| `POST /api/futures/ginie/scalp-reentry/toggle` | `POST /api/futures/ginie/position-optimizer/toggle` |
| `GET /api/futures/ginie/scalp-reentry/positions` | `GET /api/futures/ginie/position-optimizer/positions` |
| `GET /api/futures/ginie/scalp-reentry/positions/:symbol` | `GET /api/futures/ginie/position-optimizer/positions/:symbol` |

**New Endpoints:**

| Method | Endpoint | Purpose |
|--------|----------|---------|
| `GET` | `/api/futures/ginie/position-optimizer/modes` | Get all mode configs for user |
| `POST` | `/api/futures/ginie/position-optimizer/modes/:mode` | Update specific mode config |
| `POST` | `/api/futures/ginie/position-optimizer/modes/:mode/toggle` | Toggle mode enabled |
| `GET` | `/api/futures/ginie/position-optimizer/global` | Get global settings |
| `POST` | `/api/futures/ginie/position-optimizer/global` | Update global settings |

### 6. Repository Layer

**New File:** `internal/database/repository_position_optimizer.go`

```go
// GetUserPositionOptimizerConfig retrieves position optimizer config for a mode
func (r *Repository) GetUserPositionOptimizerConfig(ctx context.Context, userID, modeName string) (*PositionOptimizerModeConfig, error)

// GetAllUserPositionOptimizerConfigs retrieves all mode configs for a user
func (r *Repository) GetAllUserPositionOptimizerConfigs(ctx context.Context, userID string) (map[string]*PositionOptimizerModeConfig, error)

// SaveUserPositionOptimizerConfig saves/updates a mode config (UPSERT)
func (r *Repository) SaveUserPositionOptimizerConfig(ctx context.Context, userID, modeName string, config *PositionOptimizerModeConfig) error

// UpdateUserPositionOptimizerEnabled updates only the enabled flag for a mode
func (r *Repository) UpdateUserPositionOptimizerEnabled(ctx context.Context, userID, modeName string, enabled bool) error

// GetUserPositionOptimizerGlobal retrieves global settings for a user
func (r *Repository) GetUserPositionOptimizerGlobal(ctx context.Context, userID string) (*PositionOptimizerGlobalConfig, error)

// SaveUserPositionOptimizerGlobal saves/updates global settings
func (r *Repository) SaveUserPositionOptimizerGlobal(ctx context.Context, userID string, config *PositionOptimizerGlobalConfig) error

// InitializeUserPositionOptimizer creates default configs for a new user
func (r *Repository) InitializeUserPositionOptimizer(ctx context.Context, userID string) error
```

---

## Acceptance Criteria

### AC4.12.1: Database Schema Created
- [ ] Migration file `013_position_optimizer.sql` created
- [ ] `user_position_optimizer_configs` table exists with all columns
- [ ] `user_position_optimizer_global` table exists with all columns
- [ ] Indexes created for performance
- [ ] Foreign key constraints to users table

### AC4.12.2: Repository Layer Implemented
- [ ] `repository_position_optimizer.go` created
- [ ] All CRUD methods implemented
- [ ] Per-mode config retrieval works
- [ ] Global config retrieval works
- [ ] UPSERT operations work correctly

### AC4.12.3: API Handlers Database-First
- [ ] `GET /position-optimizer/config` reads from database
- [ ] `POST /position-optimizer/config` writes to database first
- [ ] `GET /position-optimizer/modes` returns all mode configs
- [ ] `POST /position-optimizer/modes/:mode` updates specific mode
- [ ] `POST /position-optimizer/modes/:mode/toggle` toggles mode enabled
- [ ] Old endpoints return deprecation warning + redirect

### AC4.12.4: Files and Types Renamed
- [ ] All `scalp_reentry_*.go` files renamed to `position_optimizer_*.go`
- [ ] All `ScalpReentry*` types renamed to `PositionOptimizer*`
- [ ] All references updated throughout codebase
- [ ] No compilation errors

### AC4.12.5: Per-Mode Configuration
- [ ] Each of 4 modes has independent re-entry config
- [ ] Default TP levels set per mode (see table above)
- [ ] Mode can be enabled/disabled independently
- [ ] UI shows all 4 modes with toggles

### AC4.12.6: Backward Compatibility
- [ ] Existing `scalp_reentry` positions continue to work
- [ ] Mode internally mapped: `scalp_reentry` → `position_optimizer` (scalp)
- [ ] JSON config migrated to database on first run
- [ ] Old API endpoints work with deprecation warning

### AC4.12.7: Data Migration
- [ ] Migration script reads from `autopilot_settings.json`
- [ ] Existing `scalp_reentry_config` copied to `scalp` mode in database
- [ ] Global settings extracted and saved separately
- [ ] Migration is idempotent (can run multiple times safely)

---

## Testing Requirements

### Test 1: Database Schema
```bash
# Verify tables exist
docker exec binance-bot-postgres psql -U trading_bot -d trading_bot -c "\d user_position_optimizer_configs"
docker exec binance-bot-postgres psql -U trading_bot -d trading_bot -c "\d user_position_optimizer_global"
```

### Test 2: Per-Mode Config
```bash
# Get all mode configs
curl -s http://localhost:8094/api/futures/ginie/position-optimizer/modes \
  -H "Authorization: Bearer $TOKEN"

# Expected: 4 modes with configs (ultra_fast, scalp, swing, position)
```

### Test 3: Toggle Mode
```bash
# Enable swing re-entry
curl -X POST http://localhost:8094/api/futures/ginie/position-optimizer/modes/swing/toggle \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'

# Verify in database
docker exec binance-bot-postgres psql -U trading_bot -d trading_bot -c \
  "SELECT mode_name, enabled FROM user_position_optimizer_configs WHERE user_id = '<user_id>';"
```

### Test 4: Position Optimizer Applies to All Modes
```bash
# Create a swing position with re-entry enabled
# Verify position shows in /position-optimizer/positions
# Verify TP levels match swing config (1.5%, 3%, 5%)
```

---

## Files to Create/Modify

### New Files
| File | Purpose |
|------|---------|
| `migrations/013_position_optimizer.sql` | Database schema |
| `internal/database/repository_position_optimizer.go` | Database operations |
| `internal/autopilot/position_optimizer_types.go` | Type definitions (renamed) |
| `internal/autopilot/position_optimizer_logic.go` | Core logic (renamed) |
| `internal/autopilot/position_optimizer_agents.go` | AI agents (renamed) |
| `internal/autopilot/position_optimizer_learning.go` | Adaptive learning (renamed) |
| `internal/api/handlers_position_optimizer.go` | API handlers |

### Modified Files
| File | Changes |
|------|---------|
| `internal/api/server.go` | New route registrations |
| `internal/autopilot/ginie_autopilot.go` | Use PositionOptimizer for all modes |
| `internal/autopilot/settings.go` | Update config types |
| `web/src/services/futuresApi.ts` | New API methods |
| `web/src/components/PositionOptimizerMonitor.tsx` | Renamed component |

### Files to Delete (After Migration)
| File | Reason |
|------|--------|
| `internal/autopilot/scalp_reentry_*.go` | Renamed to position_optimizer |
| `web/src/components/ScalpReentryMonitor.tsx` | Renamed |

---

## Dependencies

### Prerequisites
- Story 4.11: DB-First Mode Enabled Status (completed)
- PostgreSQL database running
- User authentication working

### Blocks
- Phase 2: UI redesign for multi-mode position optimizer
- Phase 3: Hedge order integration

---

## Future Integration Note

> **HEDGE ORDER INTEGRATION**
>
> Hedge order functionality has been developed separately by another developer.
> It will be integrated into the `position_optimizer` module in a future story.
> The database schema and types should be designed with hedge integration in mind.
>
> Potential additions for hedge:
> - `hedge_enabled` column in `user_position_optimizer_configs`
> - `hedge_config_json` JSONB column for hedge-specific settings
> - Hedge-related methods in repository

---

## Definition of Done

- [ ] Database migration runs successfully
- [ ] All files renamed from `scalp_reentry` to `position_optimizer`
- [ ] API handlers read/write from database (not JSON)
- [ ] All 4 modes have independent re-entry configs
- [ ] Per-user configuration works
- [ ] Existing positions continue to work
- [ ] All tests pass
- [ ] No compilation errors
- [ ] Code review approved

---

## Approval Sign-Off

- **Scrum Master (Bob):** Ready for review
- **Architect (Winston):** Pending
- **Developer (Amelia):** Pending
- **Product Manager (John):** Pending

---

## Notes

### Why "Position Optimizer"?
This name was chosen because:
1. It clearly indicates the module optimizes position management
2. It's future-proof for hedge order integration
3. It doesn't tie the module to a specific trading mode
4. It accurately describes the functionality (TP optimization, re-entry, dynamic SL)

### Migration Strategy
1. Create new database tables
2. Rename files and types
3. Update API handlers to use database
4. Migrate existing JSON config to database
5. Keep backward compatibility for transition period
6. Remove deprecated code after verification
