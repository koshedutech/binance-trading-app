# Story 11.14: Indicator Performance Tracker

## Story Overview

**Story ID:** 11-14
**Epic:** 11 - Position Decision Engine
**Priority:** P2
**Status:** completed
**Created:** 2026-01-18
**Completed:** 2026-01-18
**Complexity:** Medium (5-7 tasks)

---

## User Story

As a **trading system**, I need to **track which indicator combinations perform best** so that **I can correlate indicator values with trade outcomes, identify high-performing combinations, and surface recommendations to users**.

---

## Acceptance Criteria

- [x] AC1: Log indicator values at entry time (symbol, strategy, all active indicators)
- [x] AC2: Correlate indicator values with trade outcomes (win/loss, PnL%)
- [x] AC3: Identify high-performing indicator combinations by strategy
- [x] AC4: Surface recommendations to users based on historical performance
- [x] AC5: Optional auto-optimization mode flag (implementation deferred, flag only)

---

## Technical Context

### Existing Implementation (from Story 11-12, 11-13)

**IndicatorPerformanceTracker** (`internal/decision/indicators/performance.go`):
- Basic `IndicatorPerformance` struct with trade/win counts
- `PerformanceTracker` with Redis backing
- `RecordTrade(ctx, userID, indicators, segment, won, score, pnl)` - basic recording
- `GetTopCombinations(ctx, userID, segment, limit)` - returns top by win rate

**Missing Functionality:**
- Entry/exit pattern for correlating indicator values with outcomes
- Per-strategy tracking (currently segment-only)
- Correlation analysis between individual indicators and win rate
- Recommendation generation based on performance data

### Related Systems

- **Calibration System** (`internal/decision/calibration.go`): Score-to-probability mapping
- **Indicator Calculation Engine** (`internal/decision/indicators/calculation_engine.go`): Calculates all indicators
- **GinieAutopilot** (`internal/autopilot/ginie_autopilot.go`): Trade entry/exit flow

---

## Tasks

### Task 1: Extend IndicatorPerformanceTracker Service

**File:** `internal/decision/indicator_performance.go`

Create an enhanced service that builds on the existing performance.go:

```go
// TradeIndicatorSnapshot captures indicator values at trade entry
type TradeIndicatorSnapshot struct {
    TradeID      string             `json:"trade_id"`
    Symbol       string             `json:"symbol"`
    Strategy     string             `json:"strategy"`
    Indicators   map[string]float64 `json:"indicators"`    // name -> value
    EntryScore   int                `json:"entry_score"`
    EntryTime    time.Time          `json:"entry_time"`
    MarketRegime string             `json:"market_regime"`
}

// TradeOutcome records the result of a trade
type TradeOutcome struct {
    TradeID     string    `json:"trade_id"`
    IsWin       bool      `json:"is_win"`
    PnLPercent  float64   `json:"pnl_percent"`
    ExitTime    time.Time `json:"exit_time"`
    ExitReason  string    `json:"exit_reason"`
}

// IndicatorPerformanceTracker tracks indicator performance across trades
type IndicatorPerformanceTracker struct {
    cache       *cache.CacheService
    dbRepo      IndicatorPerformanceRepository
    mu          sync.RWMutex
    pendingTrades map[string]*TradeIndicatorSnapshot  // tradeID -> snapshot
}

// RecordTradeEntry logs indicator values when a trade is opened
func (t *IndicatorPerformanceTracker) RecordTradeEntry(ctx context.Context, snapshot *TradeIndicatorSnapshot) error

// RecordTradeExit records the trade outcome and updates all relevant statistics
func (t *IndicatorPerformanceTracker) RecordTradeExit(ctx context.Context, outcome *TradeOutcome) error

// GetIndicatorCorrelations returns correlation coefficients between each indicator and win rate
func (t *IndicatorPerformanceTracker) GetIndicatorCorrelations(ctx context.Context, userID int, strategy string) (map[string]float64, error)

// GetTopPerformingCombinations returns the best indicator sets by win rate and PnL
func (t *IndicatorPerformanceTracker) GetTopPerformingCombinations(ctx context.Context, userID int, strategy string, limit int) ([]IndicatorCombinationStats, error)

// GetRecommendations generates actionable recommendations based on performance data
func (t *IndicatorPerformanceTracker) GetRecommendations(ctx context.Context, userID int, strategy string) ([]Recommendation, error)
```

### Task 2: Create Supporting Types and Repository Interface

**File:** `internal/decision/indicator_performance.go` (continued)

```go
// IndicatorCombinationStats tracks performance for a specific indicator combination
type IndicatorCombinationStats struct {
    CombinationHash string             `json:"combination_hash"`
    Indicators      []string           `json:"indicators"`
    TradeCount      int                `json:"trade_count"`
    WinCount        int                `json:"win_count"`
    WinRate         float64            `json:"win_rate"`
    AvgPnLPercent   float64            `json:"avg_pnl_percent"`
    AvgEntryScore   float64            `json:"avg_entry_score"`
    LastUpdated     time.Time          `json:"last_updated"`
}

// IndicatorCorrelation tracks individual indicator correlation with success
type IndicatorCorrelation struct {
    IndicatorName     string  `json:"indicator_name"`
    WinCorrelation    float64 `json:"win_correlation"`    // -1 to 1
    PnLCorrelation    float64 `json:"pnl_correlation"`    // -1 to 1
    SampleSize        int     `json:"sample_size"`
    AvgValueOnWin     float64 `json:"avg_value_on_win"`
    AvgValueOnLoss    float64 `json:"avg_value_on_loss"`
}

// Recommendation represents an actionable suggestion
type Recommendation struct {
    Type           string   `json:"type"`            // "ADD_INDICATOR", "REMOVE_INDICATOR", "ADJUST_THRESHOLD"
    Priority       string   `json:"priority"`        // "HIGH", "MEDIUM", "LOW"
    Indicator      string   `json:"indicator,omitempty"`
    Segment        string   `json:"segment,omitempty"`
    CurrentValue   float64  `json:"current_value,omitempty"`
    SuggestedValue float64  `json:"suggested_value,omitempty"`
    Reason         string   `json:"reason"`
    ConfidenceLevel float64 `json:"confidence_level"` // 0-1
    BasedOnTrades  int      `json:"based_on_trades"`
}

// IndicatorPerformanceRepository interface for database operations
type IndicatorPerformanceRepository interface {
    SaveTradeSnapshot(ctx context.Context, snapshot *TradeIndicatorSnapshot) error
    UpdateTradeOutcome(ctx context.Context, tradeID string, outcome *TradeOutcome) error
    GetTradeSnapshots(ctx context.Context, userID int, strategy string, limit int) ([]*TradeIndicatorSnapshot, error)
    GetCompletedTrades(ctx context.Context, userID int, strategy string, limit int) ([]*CompletedTrade, error)
}
```

### Task 3: Implement Correlation Calculation

**File:** `internal/decision/indicator_performance.go`

```go
// calculateCorrelation computes Pearson correlation between indicator values and outcomes
func (t *IndicatorPerformanceTracker) calculateCorrelation(values []float64, wins []bool) float64 {
    // Convert wins to 1/0
    // Calculate mean of both series
    // Calculate correlation coefficient
}

// buildCorrelationData aggregates trade data for correlation analysis
func (t *IndicatorPerformanceTracker) buildCorrelationData(ctx context.Context, userID int, strategy string) (map[string][]float64, []bool, error)
```

### Task 4: Implement Recommendation Generation

**File:** `internal/decision/indicator_performance.go`

```go
// analyzeIndicatorPerformance generates recommendations based on data
func (t *IndicatorPerformanceTracker) analyzeIndicatorPerformance(
    correlations map[string]float64,
    combinations []IndicatorCombinationStats,
    minTrades int,
) []Recommendation {
    // 1. Find poorly performing indicators (negative correlation)
    // 2. Find high-performing combinations
    // 3. Suggest adding indicators from winning combinations
    // 4. Suggest removing indicators from losing combinations
    // 5. Set confidence based on sample size
}
```

### Task 5: Create API Endpoints

**File:** `internal/api/handlers_indicator_performance.go`

```go
// GET /api/futures/indicators/performance/{strategy}
func (s *Server) handleGetIndicatorPerformance(w http.ResponseWriter, r *http.Request)

// GET /api/futures/indicators/recommendations/{strategy}
func (s *Server) handleGetIndicatorRecommendations(w http.ResponseWriter, r *http.Request)

// GET /api/futures/indicators/correlations/{strategy}
func (s *Server) handleGetIndicatorCorrelations(w http.ResponseWriter, r *http.Request)
```

### Task 6: Wire into Trade Flow

**File:** Update `internal/autopilot/ginie_autopilot.go`

On trade entry:
```go
// After position opened successfully
snapshot := &decision.TradeIndicatorSnapshot{
    TradeID:    orderID,
    Symbol:     symbol,
    Strategy:   strategy,
    Indicators: currentIndicatorValues,
    EntryScore: score,
    EntryTime:  time.Now(),
}
g.indicatorPerfTracker.RecordTradeEntry(ctx, snapshot)
```

On trade exit:
```go
// After position closed
outcome := &decision.TradeOutcome{
    TradeID:    orderID,
    IsWin:      realizedPnL > 0,
    PnLPercent: pnlPercent,
    ExitTime:   time.Now(),
    ExitReason: exitReason,
}
g.indicatorPerfTracker.RecordTradeExit(ctx, outcome)
```

### Task 7: Write Unit Tests

**File:** `internal/decision/indicator_performance_test.go`

Test cases:
1. RecordTradeEntry stores snapshot correctly
2. RecordTradeExit correlates with entry and updates stats
3. Correlation calculation accuracy
4. Top combinations ranking
5. Recommendation generation logic
6. Edge cases (no data, insufficient trades)

---

## Dependencies

- Story 11-12: Indicator Segment Framework (DONE)
- Story 11-13: Indicator Calculation Engine (DONE)
- Story 11-25/11-26: Calibration Data Storage/Lifecycle (DONE)

---

## Test Scenarios

1. **Entry/Exit Flow**: Record entry, record exit, verify stats updated
2. **Correlation Accuracy**: Test with known data, verify correlation coefficient
3. **Ranking Logic**: Multiple combinations, verify correct ordering
4. **Recommendation Quality**: Test recommendation triggers and confidence
5. **API Response**: Verify JSON structure and data accuracy

---

## Files to Create/Modify

### New Files
- `internal/decision/indicator_performance.go` (extended service)
- `internal/decision/indicator_performance_test.go`
- `internal/api/handlers_indicator_performance.go`

### Modified Files
- `internal/autopilot/ginie_autopilot.go` - Wire entry/exit tracking
- `internal/api/server.go` - Register new routes

---

## Dev Notes

### Architecture Requirements
- Use write-through pattern: Redis cache + optional DB persistence
- Follow existing calibration service patterns for consistency
- Minimum 20 trades before generating recommendations
- Correlation requires at least 10 data points

### Implementation Guidance
- Existing `indicators/performance.go` handles per-segment basic stats
- New service at `decision/indicator_performance.go` handles cross-strategy analysis
- Use SHA256 hash (first 8 chars) for indicator combination identification
- Store pending trades in memory during trade lifecycle

### Redis Key Patterns
```
decision:trade_snapshot:{userID}:{tradeID}
decision:indicator_perf:{userID}:{strategy}:{indicatorHash}
decision:indicator_corr:{userID}:{strategy}
```

---

## Definition of Done

- [x] All tasks implemented
- [x] Unit tests passing
- [x] API endpoints returning correct data
- [x] Integration with trade flow verified
- [x] Code review passed

---

## Dev Agent Record

### Implementation Plan
- Phase 1: Create extended tracker service with types
- Phase 2: Implement correlation and recommendation logic
- Phase 3: Create API endpoints
- Phase 4: Wire into autopilot trade flow
- Phase 5: Write and run tests

### Debug Log
- All tests passing for indicator performance tracker
- Build compiles successfully (excluding pre-existing example issues)
- App running and healthy on port 8094

### Completion Notes
Story 11-14 completed with all acceptance criteria met:

1. **Indicator Value Logging**: `RecordTradeEntry` captures all indicator values (ADX, ATR, BTC correlation, sentiment, trend, volatility, volume) at trade entry
2. **Outcome Correlation**: `RecordTradeExit` correlates outcomes with entry snapshots using trade ID matching
3. **Top Combinations**: `GetTopPerformingCombinations` ranks indicator sets by composite score (win rate + avg PnL)
4. **Recommendations**: `GetRecommendations` generates actionable suggestions (add/remove indicators, adjust weights)
5. **Auto-Optimize Flag**: `SetAutoOptimize` and `IsAutoOptimizeEnabled` methods added (implementation deferred per spec)

API endpoints created:
- `GET /api/futures/indicators/performance/{strategy}` - Performance statistics
- `GET /api/futures/indicators/recommendations/{strategy}` - Recommendations
- `GET /api/futures/indicators/correlations/{strategy}` - Correlation analysis
- `GET /api/futures/indicators/top-combinations/{strategy}` - Best combinations

Trade flow integration:
- Entry recording in `recordIndicatorPerformanceEntry`
- Exit recording in `recordIndicatorPerformanceExit`
- Both called from position open/close flows in GinieAutopilot

---

## File List

### Created Files
- `/internal/decision/indicator_performance.go` - Main tracker service with Redis backing
- `/internal/decision/indicator_performance_adapter.go` - Adapter for autopilot interface
- `/internal/decision/indicator_performance_test.go` - Unit tests
- `/internal/api/handlers_indicator_performance.go` - API handlers

### Modified Files
- `/internal/api/server.go` - Added tracker field and setter, registered routes
- `/internal/cache/cache_service.go` - Added ScanKeys method for key pattern scanning
- `/internal/autopilot/ginie_autopilot.go` - Added interface, field, setter, and entry/exit recording

---

## Change Log

| Date | Change | Author |
|------|--------|--------|
| 2026-01-18 | Story created from epic requirements | Claude |
| 2026-01-18 | Implementation completed - all tests passing | Claude |
