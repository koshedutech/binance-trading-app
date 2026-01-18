# Story 11.18: Blocking Reason Tracker

**Epic:** Epic 11: Position Decision Engine
**Priority:** P0 (Critical - Core blocking logic)
**Status:** ready-for-dev
**Dependencies:** Stories 11.1 (Redis State Management), 11.15 (Additive Score Calculator)

---

## Goal Statement

Implement comprehensive blocking reason tracking to provide clear visibility into why position entry signals are blocked or restricted. This enables transparency in decision logic, facilitates debugging, and allows users to understand and override blocks when appropriate.

---

## Acceptance Criteria

- [ ] **AC1: Blocking Reason Storage in Redis** - All blocking reasons for a coin are stored in Redis state with category, severity, and reason code tagged
- [ ] **AC2: Block Category Classification** - Blocking reasons are classified into three categories (Hard Block, Soft Block, Warning) with clear type distinction
- [ ] **AC3: UI Display of Blocking Reasons** - Frontend displays blocking reasons clearly with category indicators, explanation text, and override capabilities (where applicable)
- [ ] **AC4: Historical Blocking Analysis** - Historical blocking reason data is tracked and available for analysis, enabling pattern detection and performance insights
- [ ] **AC5: Configurable Block Thresholds** - Block threshold values are configurable per strategy and persisted through settings lifecycle

---

## Implementation Tasks

### Task 1: Design Blocking Reason Data Structures
**Description:** Create Go structs for blocking reason tracking and classification

**Subtasks:**
- Define `BlockingCategory` enum (HARD_BLOCK, SOFT_BLOCK, WARNING)
- Define `BlockingReason` struct with category, severity, code, description, timestamp
- Define `BlockReasonCode` enum for standard reason codes (TREND_DIVERGENCE, ADX_TOO_LOW, SCORE_BELOW_THRESHOLD, etc.)
- Define `BlockingReasonTracker` struct with methods for adding/removing reasons
- Create constants for reason explanations and override capabilities

**Files to Create/Modify:**
- Create: `internal/models/blocking_reasons.go`
- Modify: `internal/models/models_user.go` (add blocking-related fields)

### Task 2: Implement Blocking Reason Service
**Description:** Build the core service for tracking and managing blocking reasons

**Subtasks:**
- Implement `BlockingTrackerService` struct with dependencies on StateManager and CacheService
- Implement `AddBlockingReason()` - add reason to coin state
- Implement `RemoveBlockingReason()` - remove reason from coin state
- Implement `GetBlockingReasons()` - retrieve all reasons for a symbol
- Implement `ClearBlockingReasons()` - clear all reasons for a symbol
- Implement `IsHardBlocked()` - check if coin has any hard blocks
- Implement `HasSoftBlock()` - check if coin has soft blocks
- Add methods for batch operations (multiple coins)
- Add proper logging for all blocking operations

**Files to Create/Modify:**
- Create: `internal/services/blocking_tracker_service.go`
- Modify: `internal/api/handlers_decisions.go`

### Task 3: Integrate with Settings Lifecycle
**Description:** Add blocking thresholds to user settings following the Settings Lifecycle Rule

**Subtasks:**
- Add block thresholds to `default-settings.json` (ADX threshold, score threshold, etc.)
- Add `BlockingConfig` struct to settings model
- Create database migration for new `blocking_config` table
- Implement settings persistence in `repository_user_mode_config.go`
- Add cache handlers in `settings_cache_service.go` for blocking config extraction/merge
- Update `admin_defaults_cache.go` to include default blocking thresholds

**Files to Create/Modify:**
- Modify: `default-settings.json`
- Modify: `internal/models/models_user.go`
- Create: `internal/database/migrations/000X_add_blocking_config.sql`
- Modify: `internal/database/repository_user_mode_config.go`
- Modify: `internal/cache/settings_cache_service.go`
- Modify: `internal/cache/admin_defaults_cache.go`

### Task 4: Implement Redis Blocking Reason Storage
**Description:** Store blocking reasons in Redis with proper key structure and TTL

**Subtasks:**
- Define Redis key structure: `decision:user:{userID}:coin:{symbol}:blocking_reasons`
- Implement `StoreBlockingReasonsInRedis()` with JSONB encoding
- Implement `GetBlockingReasonsFromRedis()` with cache miss handling
- Store blocking reason array with timestamps and category metadata
- Implement blocking reason history (keep last 50 blocking events per symbol)
- Define history key: `decision:user:{userID}:blocking_history:{symbol}`
- Add TTL management (5 minute TTL for current reasons, 7 day for history)
- Add cleanup routines for expired blocking data

**Files to Create/Modify:**
- Create: `internal/cache/blocking_cache_service.go`
- Modify: `internal/decision/state_manager.go` (add blocking reason methods)
- Modify: `internal/cache/redis.go`

### Task 5: Add Blocking Reason API Endpoints
**Description:** Expose blocking reason data and management via REST API

**Subtasks:**
- Create `GET /api/v1/coins/{symbol}/blocking-reasons` endpoint
- Return all current blocking reasons with category and explanation
- Create `POST /api/v1/coins/{symbol}/blocking-reasons/override` endpoint for soft block overrides
- Create `GET /api/v1/coins/{symbol}/blocking-history` endpoint with pagination
- Create `GET /api/v1/coins/{symbol}/block-analysis` endpoint for historical patterns
- Add request validation and authorization checks
- Implement write-through caching pattern

**Files to Create/Modify:**
- Modify: `internal/api/handlers_decisions.go`
- Modify: `internal/api/routes.go`

### Task 6: Add Blocking Reason Decision Integration
**Description:** Integrate blocking tracker with decision engine flow

**Subtasks:**
- Update `AdditiveScoreCalculator` to trigger blocking reasons when thresholds not met
- Update `DecisionEngine` to check blocking reasons before returning READY decision
- Add blocking reasons to `Decision` struct output
- Ensure blocking reasons are included in Redis state updates
- Add blocking reason check in signal validation pipeline

**Files to Create/Modify:**
- Modify: `internal/services/scoring_calculator.go`
- Modify: `internal/decision/decision_engine.go`
- Modify: `internal/models/models_decision.go`

---

## Technical Design

### Blocking Reason Data Structures

```go
// internal/models/blocking_reasons.go

type BlockingCategory string

const (
    BlockingCategoryHardBlock BlockingCategory = "HARD_BLOCK"
    BlockingCategorySoftBlock BlockingCategory = "SOFT_BLOCK"
    BlockingCategoryWarning   BlockingCategory = "WARNING"
)

type BlockingReasonCode string

const (
    // Hard Blocks
    BlockReasonTrendDivergence    BlockingReasonCode = "TREND_DIVERGENCE"
    BlockReasonADXTooLow          BlockingReasonCode = "ADX_TOO_LOW"
    BlockReasonConflictingSignals BlockingReasonCode = "CONFLICTING_SIGNALS"

    // Soft Blocks
    BlockReasonScoreBelowThreshold BlockingReasonCode = "SCORE_BELOW_THRESHOLD"
    BlockReasonLowLLMConfidence   BlockingReasonCode = "LOW_LLM_CONFIDENCE"
    BlockReasonIndicatorMismatch  BlockingReasonCode = "INDICATOR_MISMATCH"

    // Warnings
    BlockReasonLowVolume          BlockingReasonCode = "LOW_VOLUME"
    BlockReasonWideSpread         BlockingReasonCode = "WIDE_SPREAD"
    BlockReasonHighVolatility     BlockingReasonCode = "HIGH_VOLATILITY"
)

type BlockingReason struct {
    Code        BlockingReasonCode `json:"code"`
    Category    BlockingCategory   `json:"category"`
    Description string             `json:"description"`
    Severity    int                `json:"severity"`           // 1-10, higher = more severe
    Details     map[string]interface{} `json:"details"`       // Contextual data
    Timestamp   int64              `json:"timestamp"`         // Unix milliseconds
    CanOverride bool               `json:"can_override"`      // True for soft blocks only
}

type BlockingReasonTracker struct {
    Symbol          string             `json:"symbol"`
    ActiveReasons   []BlockingReason   `json:"active_reasons"`
    BlockedSince    int64              `json:"blocked_since"`    // When first blocked
    TotalBlockingTime int64            `json:"total_blocking_time"` // Cumulative ms
    OverrideCount   int                `json:"override_count"`   // User overrides
}

type BlockingConfig struct {
    StrategyID              string             `json:"strategy_id"`
    ADXMinThreshold         float64            `json:"adx_min_threshold"`
    ScoreMinThreshold       float64            `json:"score_min_threshold"`
    MinVolumeRatio          float64            `json:"min_volume_ratio"`
    MaxSpreadPercent        float64            `json:"max_spread_percent"`
    RequireTrendAlignment   bool               `json:"require_trend_alignment"`
    AllowSoftBlockOverride  bool               `json:"allow_soft_block_override"`
    WarningLevel            int                `json:"warning_level"`
    CreatedAt               time.Time          `json:"created_at"`
    UpdatedAt               time.Time          `json:"updated_at"`
}
```

### Blocking Reason Categories

| Category | Type | Override | Example |
|----------|------|----------|---------|
| **HARD_BLOCK** | Cannot override | No | Trend divergence (1h vs 15m), ADX < threshold |
| **SOFT_BLOCK** | Can override with confirmation | Yes | Score below threshold, Low LLM confidence |
| **WARNING** | Informational | No | Low volume, Wide spread, High volatility |

### Redis Key Structure

```
decision:user:{userID}:coin:{symbol}
├── blocking_reasons: [BlockingReason]  (JSON array)
└── blocking_timestamp: unix_ms

decision:user:{userID}:blocking_history:{symbol}
├── events: [BlockingReason]            (Keep last 50 events)
├── patterns: {...}                     (Pattern analysis data)
└── metadata: {...}                     (Historical metadata)
```

### Settings Lifecycle Integration

**default-settings.json addition:**
```json
{
  "blocking_config": {
    "adx_min_threshold": 15,
    "score_min_threshold": 55,
    "min_volume_ratio": 1.0,
    "max_spread_percent": 0.5,
    "require_trend_alignment": true,
    "allow_soft_block_override": true,
    "warning_level": 5
  }
}
```

### Blocking Reason Decision Flow

```
Signal Generated
  ↓
Check Hard Blocks:
  - Trend divergence? → HARD_BLOCK (cannot proceed)
  - ADX < threshold? → HARD_BLOCK (cannot proceed)
  - Conflicting signals? → HARD_BLOCK (cannot proceed)
  ↓ (if no hard blocks)
Check Soft Blocks:
  - Score < threshold? → SOFT_BLOCK (can override)
  - LLM confidence < 40%? → SOFT_BLOCK (can override)
  ↓ (if no blocking)
Check Warnings:
  - Volume < average? → WARNING (informational only)
  - Spread > threshold? → WARNING (informational only)
  ↓
Decision Output:
  - Status: READY / SOFT_BLOCKED / HARD_BLOCKED
  - Blocking Reasons: [array of reasons]
  - Override Available: true/false
```

---

## Dependencies

### Hard Dependencies
- **Story 11.1: Redis State Management** - Provides state storage for blocking reasons
- **Story 11.15: Additive Score Calculator** - Provides score data for soft blocks

### Soft Dependencies
- **Story 11.16: Enhanced LLM Context** - Provides LLM confidence for blocking decisions
- **Story 11.4: Market Regime Detection** - Provides regime data for context blocks

---

## Test Requirements

### Unit Tests (internal/services/blocking_tracker_service_test.go)
- Test adding/removing blocking reasons
- Test blocking reason classification
- Test hard block vs soft block detection
- Test override permission checking
- Test clearing all reasons
- Test batch operations

### Integration Tests (internal/api/handlers_decisions_test.go)
- Test API endpoints return correct blocking reasons
- Test blocking reasons persist in Redis
- Test history tracking functionality
- Test threshold configuration updates
- Test cache invalidation on config changes

### Decision Engine Tests (internal/decision/decision_engine_test.go)
- Test hard blocks prevent READY decision
- Test soft blocks are visible but don't prevent
- Test warnings don't affect decision
- Test blocking reason details in output

### E2E Tests (cypress/e2e/blocking-reasons.cy.ts)
- Generate signal with blocking reasons
- Verify blocking reasons displayed in UI
- Test soft block override flow
- Verify blocking history accumulation

---

## Files to Create

### Backend
```
internal/models/blocking_reasons.go
internal/services/blocking_tracker_service.go
internal/cache/blocking_cache_service.go
internal/database/migrations/000X_add_blocking_config.sql
internal/database/repository_blocking_config.go
```

### Tests
```
internal/services/blocking_tracker_service_test.go
internal/cache/blocking_cache_service_test.go
internal/api/handlers_decisions_test.go
cypress/e2e/blocking-reasons.cy.ts
```

---

## Files to Modify

### Backend Configuration
```
default-settings.json
internal/models/models_user.go
internal/models/models_decision.go
internal/database/repository_user_mode_config.go
internal/cache/settings_cache_service.go
internal/cache/admin_defaults_cache.go
internal/cache/redis.go
internal/decision/state_manager.go
internal/api/handlers_decisions.go
internal/api/routes.go
```

### Decision Engine Integration
```
internal/services/scoring_calculator.go
internal/decision/decision_engine.go
```

---

## Success Metrics

- All blocking reasons clearly categorized and tagged
- Hard blocks prevent signal execution
- Soft blocks visible but allow override
- Blocking history trackable over 7 days
- API response time <100ms for blocking queries
- Test coverage >85%
- UI renders blocking reasons without lag
- Threshold adjustments persist across sessions

---

## Notes

- This is part of the new Position Decision Engine (Epic 11), separate from existing Ginie autopilot
- Do NOT modify any existing autopilot blocking logic
- Hard blocks are final - no override possible
- Soft block overrides are tracked for analysis
- Blocking reason explanations should be user-friendly and actionable
- Consider making blocking reason codes and descriptions i18n-compatible for future multilingual support

---

## Change Log

| Date | Status | Notes |
|------|--------|-------|
| 2026-01-17 | ready-for-dev | Story created with complete implementation plan |
