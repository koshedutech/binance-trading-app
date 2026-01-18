# Story 11.21: Position Card UI

## Story
**ID:** 11-21-position-card-ui
**Epic:** 11 - Position Decision Engine
**Priority:** P2
**Status:** review

## Description
Unified UI for position management with collapsible cards per coin, showing current state (regime, score, indicators), blocking reasons with explanations, and support for both manual and auto trading through the same interface with real-time updates via WebSocket.

## Acceptance Criteria
- [x] Position cards display current coin state (regime, score, indicators)
- [x] Cards are collapsible/expandable
- [x] Blocking reasons are displayed with clear explanations
- [x] Cards support both manual and auto trading actions
- [x] Real-time updates via existing WebSocket infrastructure
- [x] Cards integrate with existing position/order data

## Tasks/Subtasks
- [x] Create TypeScript types for Position Card
  - [x] Define MarketRegime, TrendDirection, DecisionState types
  - [x] Define BlockingReason and BlockingSummary interfaces
  - [x] Define ScoreBreakdown and TechnicalIndicators interfaces
  - [x] Define CoinState interface
  - [x] Define PositionCardProps interface
  - [x] Add WebSocket payload types
  - [x] Add display helper constants and utility functions
- [x] Create RegimeBadge component
  - [x] Color-coded badge based on market regime
  - [x] Icon per regime type (trending, ranging, volatile, consolidating)
  - [x] Compact version for tight spaces
- [x] Create ScoreBreakdown component
  - [x] Score cards showing Technical/Context/LLM/History components
  - [x] Progress bars for detailed view
  - [x] FinalScoreBadge compact component
  - [x] ScoreRing circular progress indicator
- [x] Create BlockingReasons component
  - [x] Severity badges (Hard Block, Soft Block, Warning)
  - [x] Expandable list of blocking reasons
  - [x] Show value vs threshold for each reason
  - [x] Compact version for card headers
- [x] Create IndicatorDisplay component
  - [x] ADX, RSI, EMA9, EMA21 display
  - [x] Trend direction badges (1H, 15M)
  - [x] Trend alignment indicator
  - [x] Compact inline version
- [x] Create PositionCard main component
  - [x] Collapsible card design with header
  - [x] Expanded view with full breakdown
  - [x] Trading action buttons (Enter Long, Enter Short, Close)
  - [x] Disable actions when hard blocked
  - [x] Active position indicator
  - [x] Compact card variant
  - [x] Loading skeleton component
- [x] Create useCoinState WebSocket hook
  - [x] Subscribe to COIN_STATE_UPDATE event
  - [x] REST API fallback polling
  - [x] Merge partial state updates
  - [x] useMultipleCoinStates for lists
- [x] Export all components from index.ts

## Dev Notes

### Architecture
- Components follow existing TradeLifecycle card patterns (ChainCard.tsx)
- WebSocket hook follows useWebSocketData pattern from Epic 12
- Types mirror backend decision package structures (coin_state.go, blocking.go)

### File Structure
```
web/src/
├── types/
│   └── position-card.ts          # All TypeScript types
└── components/
    └── PositionCard/
        ├── index.ts              # Exports
        ├── PositionCard.tsx      # Main component
        ├── RegimeBadge.tsx       # Market regime indicator
        ├── ScoreBreakdown.tsx    # Score components
        ├── BlockingReasons.tsx   # Blocking reasons display
        ├── IndicatorDisplay.tsx  # Technical indicators
        └── useCoinState.ts       # WebSocket hook
```

### Usage Example
```tsx
import { PositionCard, useCoinState } from '../components/PositionCard';

function MyComponent() {
  const { state, isConnected, isRealTime } = useCoinState({
    symbol: 'BTCUSDT',
    fallbackFetch: (symbol) => api.getCoinState(symbol),
  });

  return (
    <PositionCard
      state={state}
      showActions={true}
      onEnterLong={(symbol) => handleEnterLong(symbol)}
      onEnterShort={(symbol) => handleEnterShort(symbol)}
    />
  );
}
```

### WebSocket Integration
The hook subscribes to `COIN_STATE_UPDATE` events and merges partial updates:
- Price updates only send price field
- Indicator updates send changed indicators
- Score updates send changed score components
- Full state updates send everything

Falls back to REST polling (30s default) when WebSocket is disconnected.

### Future Enhancements
- Backend API endpoint for fetching coin state (`/api/futures/decision/state/{symbol}`)
- WebSocket event emission from ws_state_sync.go
- Integration with existing Positions page or new Decision Engine dashboard

## Dev Agent Record

### Implementation Plan
1. Create types file with all interfaces and helper functions
2. Build supporting components (RegimeBadge, ScoreBreakdown, BlockingReasons, IndicatorDisplay)
3. Create main PositionCard component with collapsible design
4. Add WebSocket integration hook
5. Export all components and verify build

### Debug Log
- 2026-01-18: All components created and build verified successfully
- Frontend compiles without TypeScript errors
- Docker dev container builds and runs successfully

### Completion Notes
All acceptance criteria met:
1. Position cards display full coin state with regime, scores, and indicators
2. Cards are collapsible with chevron toggle and smooth transitions
3. Blocking reasons show severity (Hard/Soft/Warning) with values and thresholds
4. Trading action buttons included with disabled state for hard blocks
5. WebSocket hook created following Epic 12 patterns
6. Components designed to work with existing position/order data via props

## File List
- `web/src/types/position-card.ts` (new)
- `web/src/components/PositionCard/index.ts` (new)
- `web/src/components/PositionCard/PositionCard.tsx` (new)
- `web/src/components/PositionCard/RegimeBadge.tsx` (new)
- `web/src/components/PositionCard/ScoreBreakdown.tsx` (new)
- `web/src/components/PositionCard/BlockingReasons.tsx` (new)
- `web/src/components/PositionCard/IndicatorDisplay.tsx` (new)
- `web/src/components/PositionCard/useCoinState.ts` (new)

## Change Log
| Date | Change | Author |
|------|--------|--------|
| 2026-01-18 | Initial implementation of all Position Card components | Dev Agent |
