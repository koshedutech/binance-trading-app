# Story 11.45: Volume Imbalance - API Endpoints

## Story Overview

**Story ID:** 11.45
**Epic:** Epic 11 - Position Decision Engine
**Parent Story:** 11.43 (Ravindra Volume Imbalance Strategy)
**Priority:** P1 (High)
**Status:** Done
**Created:** 2026-01-24

---

## Business Context

This story adds the API endpoints for managing the strategy hierarchy. The UI (Story 11.46) will consume these endpoints.

---

## Dependencies

- **Story 11.44:** Database Schema & Repository (MUST be completed first)

---

## Scope

### In Scope
- Strategy group CRUD endpoints
- Sub-strategy CRUD endpoints
- Pattern state endpoint (for Entry Decision Engine)
- Enabled strategies endpoint

### Out of Scope
- UI components (Story 11.46)
- LLM validation (Story 11.47)

---

## Technical Implementation

### Task 1: Strategy Group Endpoints

**File:** `internal/api/handlers_strategy_hierarchy.go`

```go
// GET /api/futures/strategy-groups/:mode
// Returns all strategy groups for a mode with their settings
func (h *Handler) GetStrategyGroups(c *gin.Context) {
    mode := c.Param("mode")
    userID := getUserID(c)

    groups, err := h.strategyRepo.GetAllStrategyGroups(userID, mode)
    // Return with sub-strategy counts
}

// PUT /api/futures/strategy-groups/:mode/:group
// Updates strategy group settings (base settings + enabled)
func (h *Handler) UpdateStrategyGroup(c *gin.Context) {
    mode := c.Param("mode")
    group := c.Param("group")

    var req struct {
        Enabled             bool    `json:"enabled"`
        Timeframe           string  `json:"timeframe"`
        PositionSizePercent float64 `json:"position_size_percent"`
        MaxLeverage         int     `json:"max_leverage"`
        MaxPositions        int     `json:"max_positions"`
        MinVolumeUSDT       float64 `json:"min_volume_usdt"`
    }

    // Validate and update
    // Invalidate cache
}

// GET /api/futures/strategy-groups/:mode/:group/compare
// Compare current settings with defaults
func (h *Handler) CompareStrategyGroup(c *gin.Context) {
    // Return { current, defaults, differences }
}
```

### Task 2: Sub-Strategy Endpoints

```go
// GET /api/futures/sub-strategies/:mode/:group
// Returns all sub-strategies for a group
func (h *Handler) GetSubStrategies(c *gin.Context) {
    mode := c.Param("mode")
    group := c.Param("group")

    subStrategies, err := h.strategyRepo.GetAllSubStrategies(userID, mode, group)
    // Return with settings parsed from JSONB
}

// PUT /api/futures/sub-strategies/:mode/:group/:strategy
// Updates sub-strategy settings
func (h *Handler) UpdateSubStrategy(c *gin.Context) {
    mode := c.Param("mode")
    group := c.Param("group")
    strategy := c.Param("strategy")

    var req struct {
        Enabled  bool            `json:"enabled"`
        Settings json.RawMessage `json:"settings"`
    }

    // Validate settings structure based on strategy type
    // Update DB and cache
}

// GET /api/futures/sub-strategies/:mode/:group/:strategy/compare
// Compare current settings with defaults
func (h *Handler) CompareSubStrategy(c *gin.Context) {
    // Return { current, defaults, differences }
}
```

### Task 3: Pattern State Endpoint

```go
// GET /api/futures/patterns/volume-imbalance
// Returns current pattern states for all symbols
func (h *Handler) GetVolumeImbalancePatterns(c *gin.Context) {
    // Get from VolumeImbalanceDetector
    patterns := h.autopilot.GetVolumeImbalancePatterns()

    // Return pattern states with:
    // - Symbol
    // - State (WATCHING, CONSOLIDATING, READY)
    // - Reference candle info
    // - Consolidation progress
    // - Entry/SL/TP if ready
}

// GET /api/futures/patterns/volume-imbalance/:symbol
// Returns pattern state for specific symbol
func (h *Handler) GetVolumeImbalancePattern(c *gin.Context) {
    symbol := c.Param("symbol")
    pattern := h.autopilot.GetVolumeImbalancePattern(symbol)
}
```

### Task 4: Enabled Strategies Endpoint

```go
// GET /api/futures/enabled-strategies
// Returns all enabled sub-strategies across all modes
func (h *Handler) GetEnabledStrategies(c *gin.Context) {
    userID := getUserID(c)
    strategies := h.strategyRepo.GetEnabledStrategies(userID)

    // Return grouped by mode/strategy_group
}
```

### Task 5: Route Registration

**File:** `internal/api/server.go` (update)

```go
// Strategy Hierarchy routes
futures.GET("/strategy-groups/:mode", h.GetStrategyGroups)
futures.PUT("/strategy-groups/:mode/:group", h.UpdateStrategyGroup)
futures.GET("/strategy-groups/:mode/:group/compare", h.CompareStrategyGroup)

futures.GET("/sub-strategies/:mode/:group", h.GetSubStrategies)
futures.PUT("/sub-strategies/:mode/:group/:strategy", h.UpdateSubStrategy)
futures.GET("/sub-strategies/:mode/:group/:strategy/compare", h.CompareSubStrategy)

futures.GET("/patterns/volume-imbalance", h.GetVolumeImbalancePatterns)
futures.GET("/patterns/volume-imbalance/:symbol", h.GetVolumeImbalancePattern)

futures.GET("/enabled-strategies", h.GetEnabledStrategies)
```

---

## Acceptance Criteria

### AC1: Strategy Group Endpoints
- [ ] GET /strategy-groups/:mode returns all groups with settings
- [ ] PUT /strategy-groups/:mode/:group updates settings
- [ ] Compare endpoint shows differences from defaults

### AC2: Sub-Strategy Endpoints
- [ ] GET /sub-strategies/:mode/:group returns all sub-strategies
- [ ] PUT /sub-strategies/:mode/:group/:strategy updates settings
- [ ] Settings validated based on strategy type

### AC3: Pattern State Endpoint
- [ ] Returns current pattern states for volume imbalance
- [ ] Includes Entry/SL/TP when pattern is READY
- [ ] Updates in real-time as patterns progress

### AC4: Cache Integration
- [ ] Write-through cache on updates
- [ ] Cache invalidation working

---

## Test Plan

1. **API Tests:** Each endpoint with valid/invalid inputs
2. **Auth Tests:** Endpoints require authentication
3. **Cache Tests:** Updates invalidate cache correctly

---

## Estimation

| Task | Effort |
|------|--------|
| Strategy group endpoints | Medium |
| Sub-strategy endpoints | Medium |
| Pattern state endpoint | Small |
| Route registration | Small |
| Testing | Medium |

**Total:** Medium
