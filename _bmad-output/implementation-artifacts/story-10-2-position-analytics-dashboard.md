# Story 10.2: Position Analytics Dashboard

**Epic:** 10 - Exit Optimization Engine
**Priority:** P1 (High)
**Status:** done
**Created:** 2026-01-20

---

## User Story

**As a** trader using the Binance Trading Bot,
**I want** a Position Analytics Dashboard that displays historical efficiency analysis, trade categorization, and performance metrics,
**So that** I can understand my trading patterns, identify areas for improvement, and make data-driven decisions about my exit strategies.

---

## Context & Background

### Story 10.1 (Completed) Delivered:
- Position stage management (RISK_ZONE -> BREAKEVEN -> TP1 -> EFFICIENCY)
- Efficiency-based exit tracking with peak profit and exit efficiency calculations
- Classic/New Engine decision modes for position exit decisions
- `PositionCardExpanded` UI component showing real-time position stages
- Database migration `039_trade_efficiency_metrics.sql` storing exit efficiency data
- Repository methods for querying efficiency metrics by user/mode/symbol/strategy

### This Story Builds On:
- The `trade_efficiency_metrics` table populated by Story 10.1
- Existing dashboard patterns from Epic 11 (`DecisionEngineDashboard`, `decision-dashboard.ts` types)
- Frontend component architecture with React/TypeScript
- Backend API handler patterns (Gin framework)

---

## Acceptance Criteria

### AC1: Position Analytics Page/Tab Exists
- [ ] A new "Position Analytics" tab or page is accessible from the main navigation
- [ ] The page loads without errors when no data exists (empty state)
- [ ] The page displays a loading indicator while fetching data
- [ ] Time range filter allows selection: 7d, 30d, 90d, All Time
- **Test:** Navigate to Position Analytics page; verify it renders correctly with time filter

### AC2: Efficiency Over Time Chart
- [ ] Line chart displays exit efficiency percentage over time
- [ ] Data points are aggregated by day for 30d+ ranges, by hour for 7d range
- [ ] Chart shows trend line or moving average overlay
- [ ] Hovering on data points shows: date/time, efficiency %, trade count
- [ ] Chart handles missing data gracefully (gaps in timeline)
- **Test:** With 10+ trades over 7 days, verify chart displays correct efficiency trend

### AC3: Trade Categorization Charts
- [ ] Pie/donut chart shows trade distribution by trading mode (ultra_fast, scalp, swing, position)
- [ ] Pie/donut chart shows trade distribution by exit reason (TP_HIT, SL_HIT, TRAILING_STOP, MANUAL, EFFICIENCY_EXIT)
- [ ] Pie/donut chart shows trade distribution by entry strategy
- [ ] Bar chart shows win/loss distribution by decision mode (classic vs new_engine)
- [ ] All charts display percentages and absolute counts
- **Test:** With trades in multiple modes, verify each chart displays correct distribution

### AC4: Performance Metrics by Mode
- [ ] Table displays performance metrics grouped by trading mode
- [ ] Metrics include: Total Trades, Win Rate, Avg Efficiency, Avg PnL, Total PnL
- [ ] Metrics include: Avg Peak Profit %, Avg Exit Profit %, Profit Factor
- [ ] Table is sortable by any column
- [ ] Highlight row for best performing mode (highest efficiency)
- **Test:** With trades in 3+ modes, verify metrics are calculated correctly per mode

### AC5: Performance Metrics by Strategy
- [ ] Table displays performance metrics grouped by entry strategy
- [ ] Metrics include: Total Trades, Win Rate, Avg Efficiency, Total PnL
- [ ] Metrics include: Avg Hold Time, Breakeven Rate, TP1 Hit Rate
- [ ] Table supports filtering by date range and mode
- **Test:** With trades using 2+ strategies, verify strategy-specific metrics are accurate

### AC6: Performance Metrics by Market Regime
- [ ] Table displays performance metrics grouped by entry regime (TRENDING, RANGING, VOLATILE, CONSOLIDATING)
- [ ] Metrics include: Total Trades, Win Rate, Avg Efficiency, Avg PnL
- [ ] Visual indicator shows which regime performed best
- [ ] Empty state shown when regime data unavailable
- **Test:** Verify regime breakdown displays correctly when regime data exists

### AC7: Export Capabilities
- [ ] "Export" button is visible in the dashboard header
- [ ] Export dropdown allows selection: CSV, JSON
- [ ] CSV export includes all metrics data in tabular format
- [ ] JSON export includes structured data matching API response format
- [ ] Exported filename includes date range: `position-analytics-{startDate}-{endDate}.{ext}`
- [ ] Export respects current filter selections (time range, mode filter)
- **Test:** Export data for 30d range; verify file contains correct data and format

### AC8: API Endpoints
- [ ] `GET /api/futures/position-analytics/summary` returns aggregated metrics
- [ ] `GET /api/futures/position-analytics/efficiency-timeline` returns time-series data
- [ ] `GET /api/futures/position-analytics/distribution` returns categorization data
- [ ] `GET /api/futures/position-analytics/export` returns data for export
- [ ] All endpoints support query params: `range` (7d/30d/90d/all), `mode` (optional filter)
- [ ] All endpoints return appropriate error messages for invalid parameters
- **Test:** Call each endpoint with valid/invalid params; verify response structure

### AC9: Performance Requirements
- [ ] Dashboard initial load completes in < 2 seconds with 1000 trades
- [ ] Chart rendering completes in < 500ms
- [ ] Export of 1000 trades completes in < 3 seconds
- [ ] Page does not freeze during data processing
- **Test:** Load dashboard with 500+ trades; measure load time

### AC10: Empty State & Error Handling
- [ ] Empty state displays when no efficiency metrics exist
- [ ] Empty state includes helpful message: "No trade efficiency data yet. Complete some trades to see analytics."
- [ ] API errors display user-friendly error message
- [ ] Retry button available when data fetch fails
- **Test:** Clear efficiency data; verify empty state displays correctly

---

## Implementation Tasks

### Task 1: Create TypeScript Types for Position Analytics
**Subtasks:**
- 1.1 Create `web/src/types/positionAnalytics.ts` with interface definitions
- 1.2 Define `PositionAnalyticsSummary` interface (aggregated metrics)
- 1.3 Define `EfficiencyTimelineData` interface (time-series data points)
- 1.4 Define `TradeDistribution` interface (categorization data)
- 1.5 Define `ModePerformance` and `StrategyPerformance` interfaces
- 1.6 Define API response wrapper types
- 1.7 Add helper constants for colors, labels, time ranges

### Task 2: Backend - Create API Handlers
**Subtasks:**
- 2.1 Create `internal/api/handlers_position_analytics.go`
- 2.2 Implement `handleGetPositionAnalyticsSummary()` handler
- 2.3 Implement `handleGetEfficiencyTimeline()` handler
- 2.4 Implement `handleGetTradeDistribution()` handler
- 2.5 Implement `handleExportPositionAnalytics()` handler (CSV/JSON)
- 2.6 Add query parameter validation (range, mode)
- 2.7 Register routes in `internal/api/server.go`

### Task 3: Backend - Create Repository Methods for Analytics
**Subtasks:**
- 3.1 Add `GetEfficiencyTimeline(userID, startTime, endTime, aggregation)` to repository
- 3.2 Add `GetTradeDistributionByMode(userID, startTime, endTime)` to repository
- 3.3 Add `GetTradeDistributionByExitReason(userID, startTime, endTime)` to repository
- 3.4 Add `GetTradeDistributionByStrategy(userID, startTime, endTime)` to repository
- 3.5 Add `GetModePerformanceMetrics(userID, startTime, endTime)` to repository
- 3.6 Add `GetStrategyPerformanceMetrics(userID, startTime, endTime)` to repository
- 3.7 Add `GetRegimePerformanceMetrics(userID, startTime, endTime)` to repository

### Task 4: Frontend - Create Efficiency Timeline Chart Component
**Subtasks:**
- 4.1 Create `web/src/components/PositionAnalytics/EfficiencyTimelineChart.tsx`
- 4.2 Integrate Recharts library for line chart rendering
- 4.3 Add trend line calculation and overlay
- 4.4 Implement custom tooltip with efficiency details
- 4.5 Add loading skeleton and empty state
- 4.6 Add responsive sizing for different screen widths

### Task 5: Frontend - Create Trade Distribution Charts Component
**Subtasks:**
- 5.1 Create `web/src/components/PositionAnalytics/TradeDistributionCharts.tsx`
- 5.2 Implement pie chart for mode distribution using Recharts
- 5.3 Implement pie chart for exit reason distribution
- 5.4 Implement pie chart for strategy distribution
- 5.5 Implement bar chart for decision mode win/loss breakdown
- 5.6 Add legend with counts and percentages
- 5.7 Add loading skeletons

### Task 6: Frontend - Create Performance Metrics Tables Component
**Subtasks:**
- 6.1 Create `web/src/components/PositionAnalytics/PerformanceMetricsTable.tsx`
- 6.2 Implement sortable table for mode performance metrics
- 6.3 Implement sortable table for strategy performance metrics
- 6.4 Implement table for regime performance metrics
- 6.5 Add highlighting for best performing row
- 6.6 Add column tooltips explaining each metric
- 6.7 Add loading skeletons

### Task 7: Frontend - Create Export Functionality
**Subtasks:**
- 7.1 Create `web/src/components/PositionAnalytics/ExportButton.tsx`
- 7.2 Implement CSV generation from analytics data
- 7.3 Implement JSON generation from analytics data
- 7.4 Add file download trigger with proper filename
- 7.5 Add loading state during export
- 7.6 Add error handling for export failures

### Task 8: Frontend - Create Main Position Analytics Page
**Subtasks:**
- 8.1 Create `web/src/components/PositionAnalytics/PositionAnalyticsDashboard.tsx`
- 8.2 Implement page layout with grid for charts and tables
- 8.3 Add time range selector dropdown
- 8.4 Add optional mode filter
- 8.5 Integrate all child components
- 8.6 Implement data fetching with React hooks
- 8.7 Add refresh button functionality
- 8.8 Add empty state component
- 8.9 Add error boundary for component failures

### Task 9: Frontend - Add Navigation and Routing
**Subtasks:**
- 9.1 Add "Position Analytics" to navigation menu
- 9.2 Add route in App.tsx or router configuration
- 9.3 Add appropriate icon (BarChart2 or similar from lucide-react)
- 9.4 Ensure protected route (requires authentication)

### Task 10: Testing
**Subtasks:**
- 10.1 Write unit tests for repository methods
- 10.2 Write API handler tests with mock data
- 10.3 Write React component tests for dashboard
- 10.4 Test export functionality (CSV and JSON)
- 10.5 Performance test with large dataset (1000+ trades)
- 10.6 Manual testing of all acceptance criteria

---

## Technical Notes

### Database Queries
Leverage existing `trade_efficiency_metrics` table indexes:
- `idx_eff_user_mode_time` for mode-based time queries
- `idx_eff_strategy` for strategy grouping
- `idx_eff_regime` for regime grouping
- `idx_eff_category` for trade category analysis

### Efficiency Timeline Aggregation
```sql
-- Daily aggregation for 30d+ range
SELECT DATE(exit_time) as date,
       AVG(exit_efficiency) as avg_efficiency,
       COUNT(*) as trade_count
FROM trade_efficiency_metrics
WHERE user_id = $1 AND exit_time >= $2
GROUP BY DATE(exit_time)
ORDER BY date;

-- Hourly aggregation for 7d range
SELECT DATE_TRUNC('hour', exit_time) as hour,
       AVG(exit_efficiency) as avg_efficiency,
       COUNT(*) as trade_count
FROM trade_efficiency_metrics
WHERE user_id = $1 AND exit_time >= $2
GROUP BY DATE_TRUNC('hour', exit_time)
ORDER BY hour;
```

### Performance Metrics Calculation
```go
type ModePerformanceMetrics struct {
    Mode            string  `json:"mode"`
    TotalTrades     int     `json:"total_trades"`
    WinCount        int     `json:"win_count"`
    LossCount       int     `json:"loss_count"`
    WinRate         float64 `json:"win_rate"`
    AvgEfficiency   float64 `json:"avg_efficiency"`
    AvgPeakProfit   float64 `json:"avg_peak_profit"`
    AvgExitProfit   float64 `json:"avg_exit_profit"`
    TotalPnL        float64 `json:"total_pnl"`
    AvgPnL          float64 `json:"avg_pnl"`
    ProfitFactor    float64 `json:"profit_factor"`
    BreakevenRate   float64 `json:"breakeven_rate"`
    TP1HitRate      float64 `json:"tp1_hit_rate"`
    AvgHoldMinutes  float64 `json:"avg_hold_minutes"`
}
```

### Frontend Component Structure
```
web/src/components/PositionAnalytics/
├── PositionAnalyticsDashboard.tsx  # Main container
├── EfficiencyTimelineChart.tsx     # Line chart
├── TradeDistributionCharts.tsx     # Pie/bar charts
├── PerformanceMetricsTable.tsx     # Sortable tables
├── ExportButton.tsx                # Export dropdown
├── EmptyState.tsx                  # No data state
└── index.ts                        # Exports
```

### API Response Formats
```typescript
// Summary Response
interface PositionAnalyticsSummary {
  totalTrades: number;
  overallWinRate: number;
  overallEfficiency: number;
  totalPnL: number;
  bestMode: string;
  bestStrategy: string;
  modeMetrics: ModePerformanceMetrics[];
  strategyMetrics: StrategyPerformanceMetrics[];
  regimeMetrics: RegimePerformanceMetrics[];
  periodStart: string;
  periodEnd: string;
}

// Timeline Response
interface EfficiencyTimelineResponse {
  dataPoints: EfficiencyTimelineDataPoint[];
  aggregation: 'hourly' | 'daily';
  trendSlope: number; // positive = improving
}

// Distribution Response
interface TradeDistributionResponse {
  byMode: { mode: string; count: number; percentage: number }[];
  byExitReason: { reason: string; count: number; percentage: number }[];
  byStrategy: { strategy: string; count: number; percentage: number }[];
  byDecisionMode: { mode: string; wins: number; losses: number }[];
}
```

---

## Dependencies

### Depends On (Must be completed first):
- **Story 10.1: Position Stage Management** (COMPLETED)
  - Provides `trade_efficiency_metrics` table with data
  - Provides `TradeEfficiencyMetric` Go struct
  - Provides `EfficiencyMetricsRepository` interface

### External Dependencies:
- **Recharts** library for React charting (already in project dependencies)
- **date-fns** for date manipulation (already in project dependencies)
- **lucide-react** for icons (already in project dependencies)

---

## Files to Create

| File | Description |
|------|-------------|
| `web/src/types/positionAnalytics.ts` | TypeScript type definitions |
| `web/src/components/PositionAnalytics/PositionAnalyticsDashboard.tsx` | Main dashboard page |
| `web/src/components/PositionAnalytics/EfficiencyTimelineChart.tsx` | Efficiency over time line chart |
| `web/src/components/PositionAnalytics/TradeDistributionCharts.tsx` | Distribution pie/bar charts |
| `web/src/components/PositionAnalytics/PerformanceMetricsTable.tsx` | Mode/strategy/regime tables |
| `web/src/components/PositionAnalytics/ExportButton.tsx` | Export dropdown component |
| `web/src/components/PositionAnalytics/EmptyState.tsx` | No data empty state |
| `web/src/components/PositionAnalytics/index.ts` | Component exports |
| `internal/api/handlers_position_analytics.go` | Backend API handlers |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/api/server.go` | Register new position analytics routes |
| `internal/database/repository_efficiency_metrics.go` | Add analytics query methods |
| `web/src/App.tsx` or routing config | Add route for analytics page |
| `web/src/components/Header.tsx` or navigation | Add nav link to analytics |
| `web/src/services/api.ts` | Add API client methods for analytics |

---

## UI Mockup (Text Description)

```
+------------------------------------------------------------------+
| Position Analytics                    [7d v] [Refresh] [Export v] |
+------------------------------------------------------------------+
|                                                                   |
| +---------------------------+   +-------------------------------+ |
| | Efficiency Over Time      |   | Trade Distribution by Mode    | |
| | [Line chart with trend]   |   | [Pie chart: scalp, swing...]  | |
| +---------------------------+   +-------------------------------+ |
|                                                                   |
| +---------------------------+   +-------------------------------+ |
| | Exit Reason Distribution  |   | Decision Mode Win/Loss        | |
| | [Pie: TP_HIT, SL_HIT...]  |   | [Bar: classic vs new_engine]  | |
| +---------------------------+   +-------------------------------+ |
|                                                                   |
| +---------------------------------------------------------------+ |
| | Performance by Mode                           [Sort: Eff v]   | |
| +---------------------------------------------------------------+ |
| | Mode      | Trades | Win% | Efficiency | Avg PnL | Total PnL  | |
| | scalp     |   45   | 68%  |   78.5%    | +$12.50 |  +$562.50  | |
| | swing     |   23   | 74%  |   82.3%    | +$45.20 |  +$1039.60 | |
| +---------------------------------------------------------------+ |
|                                                                   |
| +---------------------------------------------------------------+ |
| | Performance by Strategy                                       | |
| +---------------------------------------------------------------+ |
| | Strategy       | Trades | Win% | Efficiency | BE% | TP1% |    | |
| | trend_follow   |   38   | 71%  |   80.1%    | 84% | 62%  |    | |
| | mean_reversion |   30   | 63%  |   75.2%    | 70% | 45%  |    | |
| +---------------------------------------------------------------+ |
|                                                                   |
+------------------------------------------------------------------+
| Last updated: 2026-01-20 14:30:00                                |
+------------------------------------------------------------------+
```

---

## Estimation

| Task | Effort |
|------|--------|
| Task 1: TypeScript Types | 2 hours |
| Task 2: Backend API Handlers | 4 hours |
| Task 3: Repository Methods | 3 hours |
| Task 4: Efficiency Timeline Chart | 3 hours |
| Task 5: Trade Distribution Charts | 3 hours |
| Task 6: Performance Metrics Tables | 3 hours |
| Task 7: Export Functionality | 2 hours |
| Task 8: Main Dashboard Page | 3 hours |
| Task 9: Navigation/Routing | 1 hour |
| Task 10: Testing | 4 hours |
| **Total** | **28 hours** |

---

## Change Log

| Date | Status | Notes |
|------|--------|-------|
| 2026-01-20 | ready-for-dev | Story created from Epic 10 requirements |
| 2026-01-20 | review | Implementation complete - all 10 tasks implemented |
| 2026-01-20 | done | CODE REVIEW PASSED (8 issues fixed) + QA TRACE PASSED (8/8 critical ACs covered) |

## Implementation Summary

### Files Created
- `web/src/types/positionAnalytics.ts` - TypeScript type definitions with interfaces, constants, and utility functions
- `web/src/components/PositionAnalytics/EfficiencyTimelineChart.tsx` - Line chart with efficiency over time, win rate, and trade count
- `web/src/components/PositionAnalytics/TradeDistributionCharts.tsx` - Pie/bar charts for mode, exit reason, strategy, decision mode distribution
- `web/src/components/PositionAnalytics/PerformanceMetricsTables.tsx` - Sortable tables for mode, strategy, and regime performance metrics
- `web/src/components/PositionAnalytics/ExportButton.tsx` - Export functionality for CSV and JSON
- `web/src/components/PositionAnalytics/PositionAnalyticsDashboard.tsx` - Main dashboard component with all integrations
- `web/src/components/PositionAnalytics/index.ts` - Component exports
- `internal/api/handlers_position_analytics.go` - Backend API handlers for all 4 endpoints

### Files Modified
- `internal/api/server.go` - Added routes for position analytics endpoints
- `internal/database/repository_efficiency_metrics.go` - Added analytics query methods (timeline, distributions, performance metrics)
- `web/src/App.tsx` - Added route `/position-analytics` with ProtectedRoute
- `web/src/components/Header.tsx` - Added navigation link in More menu
- `web/src/services/futuresApi.ts` - Added API client methods for analytics endpoints

### API Endpoints Implemented
- `GET /api/futures/position-analytics/summary` - Aggregated metrics summary
- `GET /api/futures/position-analytics/efficiency-timeline` - Time-series efficiency data
- `GET /api/futures/position-analytics/distribution` - Trade categorization data
- `GET /api/futures/position-analytics/export` - CSV/JSON export

### Build Verification
- Docker build successful
- Health check passing
- Application running on port 8094
