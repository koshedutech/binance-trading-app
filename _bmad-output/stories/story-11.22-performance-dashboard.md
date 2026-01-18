# Story 11.22: Performance Dashboard

## Story
**ID:** 11-22-performance-dashboard
**Epic:** 11 - Position Decision Engine
**Priority:** P2
**Status:** done

## Description
Analytics dashboard for decision engine performance. Provides visualization of score distributions, calibration accuracy over time, strategy performance comparisons, regime classification statistics, and blocking reason frequency analysis.

## Acceptance Criteria
- [x] Dashboard displays score distribution analysis (histogram/chart)
- [x] Dashboard shows calibration accuracy metrics
- [x] Dashboard compares strategy performance (win rate, P&L by strategy)
- [x] Dashboard shows regime classification stats
- [x] Dashboard displays blocking reason frequency
- [x] Dashboard integrates with existing API endpoints

## Tasks/Subtasks
- [x] Create TypeScript types for Dashboard
  - [x] Define DashboardStats interface
  - [x] Define ScoreDistribution interface
  - [x] Define CalibrationAccuracy interface
  - [x] Define StrategyPerformance interface
  - [x] Define RegimeStats interface
  - [x] Define BlockingFrequency interface
- [x] Create ScoreDistributionChart component
  - [x] Histogram visualization using recharts
  - [x] Score bucket breakdown (0-25, 26-50, 51-75, 76-100)
  - [x] Trade count per bucket
- [x] Create CalibrationAccuracyCard component
  - [x] Show expected vs actual win rates
  - [x] Delta calculation with status indicator
  - [x] Strategy selector
- [x] Create StrategyComparisonTable component
  - [x] Table with strategy name, trades, win rate, avg P&L
  - [x] Sortable columns
  - [x] Profit factor and Sharpe ratio display
- [x] Create RegimeStatsCard component
  - [x] Pie chart or bar chart of regime distribution
  - [x] Percentage breakdown by regime type
  - [x] Time period selector
- [x] Create BlockingReasonChart component
  - [x] Bar chart of blocking reason frequency
  - [x] Category breakdown (Hard/Soft/Warning)
  - [x] Top reasons list
- [x] Create DecisionEngineDashboard page
  - [x] Main dashboard layout with metric cards
  - [x] Date range selector (Last 7 Days, 30 Days, All Time)
  - [x] Integrate all chart components
  - [x] Loading states and error handling
- [x] Create backend dashboard aggregation endpoint
  - [x] GET /api/futures/decision/dashboard/stats endpoint
  - [x] Aggregate data from existing sources
  - [x] Support date range filtering
- [x] Add route to application
  - [x] Add DecisionEngineDashboard to App.tsx routes
  - [x] Add navigation link to dashboard
- [x] Verify build and functionality
  - [x] Run build and verify no TypeScript errors
  - [x] Test API endpoint returns data
  - [x] Verify charts render correctly

## Dev Notes

### Architecture
- Components follow existing TradeLifecycle and PositionCard patterns
- Uses recharts library (already installed) for visualizations
- Integrates with existing API endpoints:
  - `/api/futures/indicators/performance/:strategy` - Indicator performance stats
  - `/api/futures/calibration/confidence/:strategy` - Calibration confidence
  - `/api/futures/calibration/data/:strategy` - Calibration bucket data
  - `/api/strategy-performance` - Strategy performance metrics
- May need new aggregate endpoint for blocking reasons and regime stats

### File Structure
```
web/src/
├── types/
│   └── decision-dashboard.ts          # Dashboard TypeScript types
└── components/
    └── DecisionDashboard/
        ├── index.ts                   # Exports
        ├── DecisionEngineDashboard.tsx # Main dashboard page
        ├── ScoreDistributionChart.tsx # Score histogram
        ├── CalibrationAccuracyCard.tsx # Calibration metrics
        ├── StrategyComparisonTable.tsx # Strategy comparison
        ├── RegimeStatsCard.tsx        # Regime breakdown
        └── BlockingReasonChart.tsx    # Blocking frequency
```

### API Endpoints Used
- `GET /api/futures/indicators/performance/:strategy` - Existing
- `GET /api/futures/calibration/confidence/:strategy` - Existing
- `GET /api/futures/calibration/data/:strategy` - Existing
- `GET /api/strategy-performance` - Existing
- `GET /api/futures/decision/dashboard/stats` - NEW (aggregate endpoint)

### UI Design Reference
```
┌─────────────────────────────────────────────────────────────────┐
│ Decision Engine Dashboard                    [Last 7 Days ▼]   │
├─────────────────────────────────────────────────────────────────┤
│ ┌─────────────────────┐ ┌─────────────────────┐                │
│ │ Score Distribution  │ │ Calibration Accuracy│                │
│ │ [   Histogram    ]  │ │ Expected: 58%       │                │
│ │                     │ │ Actual: 62%         │                │
│ │                     │ │ Δ +4% (Good)        │                │
│ └─────────────────────┘ └─────────────────────┘                │
│                                                                 │
│ Strategy Performance                                            │
│ ┌─────────────────────────────────────────────────────────────┐│
│ │ Strategy       │ Trades │ Win Rate │ Avg P&L │ Sharpe      ││
│ │ Trend Follow   │ 145    │ 58%      │ +1.2%   │ 1.4         ││
│ │ Mean Reversion │ 82     │ 52%      │ +0.8%   │ 0.9         ││
│ │ Breakout       │ 34     │ 61%      │ +1.8%   │ 1.6         ││
│ └─────────────────────────────────────────────────────────────┘│
│                                                                 │
│ ┌─────────────────────┐ ┌─────────────────────┐                │
│ │ Regime Stats        │ │ Blocking Reasons    │                │
│ │ TRENDING: 45%       │ │ [  Bar Chart     ]  │                │
│ │ RANGING: 30%        │ │                     │                │
│ │ VOLATILE: 15%       │ │                     │                │
│ │ CONSOLIDATING: 10%  │ │                     │                │
│ └─────────────────────┘ └─────────────────────┘                │
└─────────────────────────────────────────────────────────────────┘
```

### Prerequisites (DONE)
- Story 11-21 (Position Card UI) - DONE
- Story 11-14 (Indicator Performance Tracker) - DONE
- Story 11-17 (Calibration Layer) - DONE

## Dev Agent Record

### Implementation Plan
1. Create TypeScript types for all dashboard data structures
2. Create individual chart/card components using recharts
3. Create main dashboard page integrating all components
4. Create backend aggregate endpoint for dashboard stats
5. Add route and navigation
6. Verify build and test functionality

### Debug Log
- 2026-01-18: Story file created, beginning implementation

### Completion Notes
Story 11.22 completed successfully. The Decision Engine Dashboard provides comprehensive analytics including:
- Score distribution histogram showing trade outcomes by score buckets
- Calibration accuracy metrics comparing expected vs actual win rates
- Strategy comparison table with sortable columns and performance metrics
- Regime classification pie chart showing market regime distribution
- Blocking reason frequency bar chart with category breakdown

The dashboard integrates with the existing calibration service via a new aggregate endpoint.

## File List
### New Files Created
- `web/src/types/decision-dashboard.ts` - TypeScript interfaces for dashboard data
- `web/src/components/DecisionDashboard/index.ts` - Component exports
- `web/src/components/DecisionDashboard/DecisionEngineDashboard.tsx` - Main dashboard page
- `web/src/components/DecisionDashboard/ScoreDistributionChart.tsx` - Score histogram
- `web/src/components/DecisionDashboard/CalibrationAccuracyCard.tsx` - Calibration metrics card
- `web/src/components/DecisionDashboard/StrategyComparisonTable.tsx` - Strategy comparison table
- `web/src/components/DecisionDashboard/RegimeStatsCard.tsx` - Regime distribution pie chart
- `web/src/components/DecisionDashboard/BlockingReasonChart.tsx` - Blocking frequency chart
- `internal/api/handlers_decision_dashboard.go` - Backend dashboard stats endpoint

### Modified Files
- `web/src/App.tsx` - Added route for /decision-dashboard
- `web/src/components/Header.tsx` - Added Analytics navigation link
- `internal/api/server.go` - Registered dashboard stats endpoint

## Change Log
| Date | Change | Author |
|------|--------|--------|
| 2026-01-18 | Story file created | Dev Agent |
| 2026-01-18 | All tasks completed - dashboard implemented with all components | Dev Agent |
