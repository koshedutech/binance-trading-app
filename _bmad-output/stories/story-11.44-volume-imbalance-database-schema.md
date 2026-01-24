# Story 11.44: Volume Imbalance - Database Schema & Repository

## Story Overview

**Story ID:** 11.44
**Epic:** Epic 11 - Position Decision Engine
**Parent Story:** 11.43 (Ravindra Volume Imbalance Strategy)
**Priority:** P0 (Critical)
**Status:** Done
**Created:** 2026-01-24

---

## Business Context

Story 11.43 implemented the core pattern detection logic. This story adds the database persistence layer for the strategy hierarchy (Mode → Strategy Group → Sub-Strategy).

---

## Scope

### In Scope
- Database migration for `user_strategy_group_settings` table
- Database migration for `user_sub_strategy_settings` table
- Repository layer for CRUD operations
- Cache layer integration (Redis)
- User initialization with default strategy settings

### Out of Scope
- API endpoints (Story 11.45)
- UI components (Story 11.46)
- LLM validation (Story 11.47)

---

## Technical Implementation

### Task 1: Database Migration

**File:** `migrations/047_strategy_hierarchy_tables.sql`

```sql
-- Migration 047: Strategy Hierarchy Tables
-- Supports Mode → Strategy Group → Sub-Strategy architecture

-- Strategy Group Settings (base settings per mode/group)
CREATE TABLE IF NOT EXISTS user_strategy_group_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode VARCHAR(20) NOT NULL,           -- 'scalp', 'swing', 'position', 'ultra_fast'
    strategy_group VARCHAR(20) NOT NULL, -- 'breakout', 'trending', 'range', 'volatile'
    enabled BOOLEAN DEFAULT false,

    -- Base settings (inherited by all sub-strategies)
    timeframe VARCHAR(10) NOT NULL DEFAULT '15m',
    position_size_percent DECIMAL(5,2) DEFAULT 2.0,
    max_leverage INTEGER DEFAULT 10,
    max_positions INTEGER DEFAULT 3,
    min_volume_usdt DECIMAL(15,2) DEFAULT 1000000,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(user_id, mode, strategy_group)
);

CREATE INDEX idx_strategy_group_user_mode
ON user_strategy_group_settings(user_id, mode);

CREATE INDEX idx_strategy_group_enabled
ON user_strategy_group_settings(user_id, enabled) WHERE enabled = true;

-- Sub-Strategy Settings (strategy-specific settings)
CREATE TABLE IF NOT EXISTS user_sub_strategy_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode VARCHAR(20) NOT NULL,
    strategy_group VARCHAR(20) NOT NULL,
    sub_strategy VARCHAR(50) NOT NULL,   -- 'ravindra_volume_imbalance', 'classic_breakout'
    enabled BOOLEAN DEFAULT false,

    -- Strategy-specific settings (JSONB for flexibility)
    settings JSONB DEFAULT '{}',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(user_id, mode, strategy_group, sub_strategy)
);

CREATE INDEX idx_sub_strategy_enabled
ON user_sub_strategy_settings(user_id, enabled) WHERE enabled = true;

CREATE INDEX idx_sub_strategy_lookup
ON user_sub_strategy_settings(user_id, mode, strategy_group);

-- Comments
COMMENT ON TABLE user_strategy_group_settings IS 'Strategy group settings per mode (breakout, trending, range, volatile)';
COMMENT ON TABLE user_sub_strategy_settings IS 'Sub-strategy specific settings (e.g., ravindra_volume_imbalance parameters)';
```

### Task 2: Repository Layer

**File:** `internal/database/repository_strategy_hierarchy.go`

```go
// StrategyGroupSettings represents a strategy group configuration
type StrategyGroupSettings struct {
    ID                  string    `db:"id"`
    UserID              string    `db:"user_id"`
    Mode                string    `db:"mode"`
    StrategyGroup       string    `db:"strategy_group"`
    Enabled             bool      `db:"enabled"`
    Timeframe           string    `db:"timeframe"`
    PositionSizePercent float64   `db:"position_size_percent"`
    MaxLeverage         int       `db:"max_leverage"`
    MaxPositions        int       `db:"max_positions"`
    MinVolumeUSDT       float64   `db:"min_volume_usdt"`
    CreatedAt           time.Time `db:"created_at"`
    UpdatedAt           time.Time `db:"updated_at"`
}

// SubStrategySettings represents a sub-strategy configuration
type SubStrategySettings struct {
    ID            string          `db:"id"`
    UserID        string          `db:"user_id"`
    Mode          string          `db:"mode"`
    StrategyGroup string          `db:"strategy_group"`
    SubStrategy   string          `db:"sub_strategy"`
    Enabled       bool            `db:"enabled"`
    Settings      json.RawMessage `db:"settings"`
    CreatedAt     time.Time       `db:"created_at"`
    UpdatedAt     time.Time       `db:"updated_at"`
}

// Repository methods:
// - GetStrategyGroupSettings(userID, mode, group)
// - GetAllStrategyGroups(userID, mode)
// - UpsertStrategyGroupSettings(settings)
// - GetSubStrategySettings(userID, mode, group, subStrategy)
// - GetAllSubStrategies(userID, mode, group)
// - UpsertSubStrategySettings(settings)
// - GetEnabledStrategies(userID) - returns all enabled sub-strategies
```

### Task 3: Cache Layer Integration

**File:** `internal/cache/strategy_hierarchy_cache.go`

Redis keys:
- `user:{userID}:strategy_group:{mode}:{group}` - Strategy group settings
- `user:{userID}:sub_strategy:{mode}:{group}:{subStrategy}` - Sub-strategy settings
- `user:{userID}:enabled_strategies` - List of enabled sub-strategies (for quick lookup)

Methods:
- `GetStrategyGroupFromCache(userID, mode, group)`
- `SetStrategyGroupCache(userID, mode, group, settings)`
- `GetSubStrategyFromCache(userID, mode, group, subStrategy)`
- `SetSubStrategyCache(userID, mode, group, subStrategy, settings)`
- `InvalidateStrategyHierarchyCache(userID)`

### Task 4: User Initialization

**File:** `internal/database/user_initialization.go` (update)

When a new user is created:
1. Read strategy_hierarchy from `default-settings.json`
2. Create `user_strategy_group_settings` rows for each mode/group
3. Create `user_sub_strategy_settings` rows for each sub-strategy
4. Populate Redis cache

---

## Acceptance Criteria

### AC1: Database Tables Created
- [x] Migration 047 creates `user_strategy_group_settings` table
- [x] Migration 047 creates `user_sub_strategy_settings` table
- [x] Unique constraints prevent duplicate entries
- [x] Indexes created for performance (6 indexes + GIN for JSONB)

### AC2: Repository Layer
- [x] CRUD operations for strategy group settings
- [x] CRUD operations for sub-strategy settings
- [x] GetEnabledStrategies returns all active sub-strategies

### AC3: Cache Integration
- [x] Strategy settings cached in Redis
- [x] Cache invalidation on update
- [x] Cache-first read pattern

### AC4: User Initialization
- [x] New users get default strategy settings from default-settings.json
- [x] All modes/groups/sub-strategies initialized
- [x] Cache populated on user creation

---

## Test Plan

1. **Migration Test:** Run migration, verify tables exist with correct schema
2. **Repository Test:** CRUD operations, unique constraint violations
3. **Cache Test:** Cache hit/miss, invalidation
4. **Integration Test:** New user gets default settings

---

## Dependencies

- Story 11.43: Pattern detection logic (completed)
- default-settings.json: strategy_hierarchy section (completed)

---

## Estimation

| Task | Effort |
|------|--------|
| Database migration | Small |
| Repository layer | Medium |
| Cache integration | Medium |
| User initialization | Small |
| Testing | Medium |

**Total:** Medium
