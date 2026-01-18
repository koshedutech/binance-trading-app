# Story 11.15: Additive Score Calculator

**Epic:** Epic 11: Position Decision Engine
**Priority:** P0 (Critical - Core formula change)
**Status:** done
**Dependencies:** Stories 11.1 (Technical Analysis), 11.2 (Context Analyzer)

---

## Goal Statement

Replace the multiplicative scoring formula with an additive scoring system that independently calculates technical, context, LLM, and historical components, then combines them with configurable weights. This enables more transparent and tunable position decision logic with visible component breakdown.

---

## Acceptance Criteria

- [x] **AC1: Independent Component Scoring** - Each scoring component (Technical, Context, LLM, History) calculates a score within its defined range independently, without dependencies on other components
- [ ] **AC2: Component Breakdown Visibility** - UI displays individual component scores and their contributions (deferred to frontend story)
- [x] **AC3: Configurable Strategy Weights** - Admin settings allow per-strategy adjustment of component weights (40/30/20/10 or custom values) that persist in the database and cache
- [x] **AC4: Redis State Storage** - Calculated scores, component breakdown, and weights are stored in Redis cache with proper key structure and TTL management
- [x] **AC5: Historical Score Tracking** - Score history is recorded and retrievable for analysis, enabling performance trending and backtesting

---

## Implementation Tasks

### Task 1: Design Score Data Structures
**Description:** Create Go structs for score calculation and storage

**Subtasks:**
- Define `AdditiveScoreCalculator` struct with component handlers
- Define `ScoreComponent` struct with min, max, and current values
- Define `ScoreBreakdown` struct with all components and weights
- Create `ScoreDimension` enum for component types
- Add score calculation parameters (strategy ID, symbol, timeframe)

**Files to Create/Modify:**
- Create: `internal/models/scoring.go`
- Modify: `internal/models/models_user.go` (add score-related fields)

### Task 2: Implement Additive Score Calculator Engine
**Description:** Build the core calculation logic for additive scoring

**Subtasks:**
- Implement `CalculateAdditiveScore()` function that accepts strategy, symbol, and technical/context data
- Fetch Technical scores from Story 11.1 (0-40 points)
- Fetch Context scores from Story 11.2 (0-30 points)
- Calculate LLM component score (0-20 points with enhanced context)
- Calculate History component score (0-10 points)
- Apply configurable weights and sum components
- Validate final score is within 0-100 range
- Return `ScoreBreakdown` with all components and final score

**Files to Create/Modify:**
- Create: `internal/services/scoring_calculator.go`
- Modify: `internal/api/handlers_decisions.go`

### Task 3: Integrate with Settings Lifecycle
**Description:** Add score configuration to user settings following the Settings Lifecycle Rule

**Subtasks:**
- Add score weights to `default-settings.json` (technical: 40, context: 30, llm: 20, history: 10)
- Add `ScoringConfig` struct to settings model
- Create database migration for new `scoring_config` table
- Implement settings persistence in `repository_user_mode_config.go`
- Add cache handlers in `settings_cache_service.go` for score config extraction/merge
- Update `admin_defaults_cache.go` to include default score weights

**Files to Create/Modify:**
- Modify: `default-settings.json`
- Modify: `internal/models/models_user.go`
- Create: `internal/database/migrations/000X_add_scoring_config.sql`
- Modify: `internal/database/repository_user_mode_config.go`
- Modify: `internal/cache/settings_cache_service.go`
- Modify: `internal/cache/admin_defaults_cache.go`

### Task 4: Implement Redis Score Storage
**Description:** Store calculated scores and component breakdown in Redis cache

**Subtasks:**
- Define Redis key structure: `score:{strategy_id}:{symbol}:{timeframe}`
- Implement `StoreScoreInRedis()` with 5-minute TTL
- Implement `GetScoreFromRedis()` with cache miss handling
- Store full `ScoreBreakdown` as JSON in Redis
- Implement score history queue (keep last 100 scores per symbol)
- Define history key structure: `score:history:{strategy_id}:{symbol}`
- Add cleanup routines for expired score data

**Files to Create/Modify:**
- Create: `internal/cache/score_cache_service.go`
- Modify: `internal/cache/redis.go`

### Task 5: Add Score Component API Endpoints
**Description:** Expose score calculation and breakdown via REST API

**Subtasks:**
- Create `GET /api/v1/strategies/{id}/score/{symbol}` endpoint
- Return component breakdown with individual scores
- Create `PUT /api/v1/strategies/{id}/scoring-config` endpoint for weight updates
- Create `GET /api/v1/strategies/{id}/score-history/{symbol}` endpoint
- Add request validation and error handling
- Implement write-through caching pattern

**Files to Create/Modify:**
- Modify: `internal/api/handlers_decisions.go`
- Modify: `internal/api/routes.go`

### Task 6: Frontend Score Display Component
**Description:** Build React components for score visualization

**Subtasks:**
- Create `ScoreBreakdown.tsx` component showing all components with color coding
- Create `ComponentScore.tsx` sub-component for individual component display
- Create `ScoreGauge.tsx` visual gauge for 0-100 score
- Create `ScoreHistoryChart.tsx` for score trend analysis
- Implement settings UI for weight configuration
- Add score display to position decision view
- Style with Tailwind CSS matching existing design

**Files to Create/Modify:**
- Create: `web/src/components/ScoreBreakdown.tsx`
- Create: `web/src/components/ComponentScore.tsx`
- Create: `web/src/components/ScoreGauge.tsx`
- Create: `web/src/components/ScoreHistoryChart.tsx`
- Create: `web/src/components/ScoreSettingsPanel.tsx`
- Modify: `web/src/pages/PositionDecision.tsx`

---

## Technical Design

### Score Calculation Flow

```
Input: Strategy, Symbol, Timeframe
  ↓
[Technical (0-40)] ← From Story 11.1
[Context (0-30)]   ← From Story 11.2
[LLM (0-20)]       ← Enhanced LLM context
[History (0-10)]   ← Symbol + Strategy perf
  ↓
Apply Weights (configurable per strategy)
  ↓
FINAL_SCORE = (Technical × weight_tech) + (Context × weight_ctx) + (LLM × weight_llm) + (History × weight_hist)
  ↓
Store in Redis + Database
  ↓
Return ScoreBreakdown with all components
```

### Data Structures

```go
// internal/models/scoring.go

type ScoreDimension string

const (
    DimensionTechnical ScoreDimension = "technical"
    DimensionContext   ScoreDimension = "context"
    DimensionLLM       ScoreDimension = "llm"
    DimensionHistory   ScoreDimension = "history"
)

type ComponentScore struct {
    Dimension   ScoreDimension `json:"dimension"`
    Score       float64        `json:"score"`       // Current value (0-max)
    MaxScore    float64        `json:"max_score"`   // Max possible
    Weight      float64        `json:"weight"`      // Percentage contribution
    Details     map[string]interface{} `json:"details"` // Sub-component breakdown
}

type ScoreBreakdown struct {
    StrategyID   string               `json:"strategy_id"`
    Symbol       string               `json:"symbol"`
    Timeframe    string               `json:"timeframe"`
    Components   []ComponentScore     `json:"components"`
    FinalScore   float64              `json:"final_score"`
    Weights      map[string]float64   `json:"weights"`
    Timestamp    time.Time            `json:"timestamp"`
    Confidence   float64              `json:"confidence"` // Overall confidence
}

type ScoringConfig struct {
    StrategyID              string             `json:"strategy_id"`
    TechnicalWeight         float64            `json:"technical_weight"`
    ContextWeight           float64            `json:"context_weight"`
    LLMWeight               float64            `json:"llm_weight"`
    HistoryWeight           float64            `json:"history_weight"`
    MinimumThreshold        float64            `json:"minimum_threshold"`
    EnableComponentTracking bool               `json:"enable_component_tracking"`
    CreatedAt               time.Time          `json:"created_at"`
    UpdatedAt               time.Time          `json:"updated_at"`
}

type AdditiveScoreCalculator struct {
    technicalAnalyzer TechnicalAnalyzer
    contextAnalyzer   ContextAnalyzer
    llmProvider       LLMProvider
    historyStore      HistoryStore
    cache             CacheService
}
```

### Redis Key Structure

```
// Current score
score:{strategy_id}:{symbol}:{timeframe} → JSON(ScoreBreakdown)
TTL: 5 minutes

// Score history queue (last 100)
score:history:{strategy_id}:{symbol} → []JSON(ScoreBreakdown)
TTL: 7 days

// Scoring configuration
scoring:config:{strategy_id} → JSON(ScoringConfig)
TTL: 24 hours (refreshed on change)
```

### Settings Lifecycle Integration

**default-settings.json addition:**
```json
{
  "scoring": {
    "technical_weight": 40,
    "context_weight": 30,
    "llm_weight": 20,
    "history_weight": 10,
    "minimum_threshold": 50,
    "enable_component_tracking": true
  }
}
```

---

## Dependencies

### Hard Dependencies
- **Story 11.1: Technical Analysis** - Must provide Technical component scores
- **Story 11.2: Context Analyzer** - Must provide Context component scores

### Optional Dependencies
- **Story 11.3: LLM Context Enrichment** - Enhances LLM component scoring
- **Story 11.4: WebSocket History** - Provides real-time score updates

---

## Test Requirements

### Unit Tests (internal/services/scoring_calculator_test.go)
- Test independent component calculation
- Test weight application
- Test score normalization (0-100)
- Test invalid input handling
- Test all component combinations

### Integration Tests (internal/api/handlers_decisions_test.go)
- Test API endpoints return correct score breakdown
- Test score persistence in Redis
- Test score history tracking
- Test weight configuration updates
- Test cache invalidation

### Component Tests (web/src/components/__tests__/)
- Test score display accuracy
- Test weight slider functionality
- Test score history chart rendering
- Test color coding for score ranges

### E2E Tests (cypress/e2e/scoring.cy.ts)
- Calculate score for multiple symbols
- Verify score breakdown matches formula
- Verify weight changes affect scores
- Verify score history accumulation

---

## Files to Create

### Backend
```
internal/models/scoring.go
internal/services/scoring_calculator.go
internal/cache/score_cache_service.go
internal/database/migrations/000X_add_scoring_config.sql
internal/database/repository_scoring_config.go
```

### Frontend
```
web/src/components/ScoreBreakdown.tsx
web/src/components/ComponentScore.tsx
web/src/components/ScoreGauge.tsx
web/src/components/ScoreHistoryChart.tsx
web/src/components/ScoreSettingsPanel.tsx
```

### Tests
```
internal/services/scoring_calculator_test.go
internal/cache/score_cache_service_test.go
internal/api/handlers_decisions_test.go
web/src/components/__tests__/ScoreBreakdown.test.tsx
web/src/components/__tests__/ScoreSettingsPanel.test.tsx
cypress/e2e/scoring.cy.ts
```

---

## Files to Modify

### Backend Configuration
```
default-settings.json
internal/models/models_user.go
internal/database/repository_user_mode_config.go
internal/cache/settings_cache_service.go
internal/cache/admin_defaults_cache.go
internal/cache/redis.go
internal/api/handlers_decisions.go
internal/api/routes.go
```

### Frontend
```
web/src/pages/PositionDecision.tsx
web/src/types/index.ts
web/src/api/client.ts
web/src/context/StrategyContext.tsx
```

---

## Success Metrics

- Score calculation executes in <100ms
- All 5 acceptance criteria satisfied
- Component breakdown visible and accurate
- Weights adjustable and persisted
- Score history trackable over 7 days
- Test coverage >85%
- API response time <200ms
- UI renders without lag at 60fps

---

## Notes

- Ensure backward compatibility with existing position decisions
- Document scoring formula changes in user guide
- Plan validation process with stakeholders
- Consider A/B testing old vs new formula
- Setup monitoring for score distribution analysis
