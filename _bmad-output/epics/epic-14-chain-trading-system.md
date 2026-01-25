# Epic 14: Chain Trading System - Coin Profiler & Entry Decision Enhancement

## Epic Overview

**Epic ID:** EPIC-14
**Status:** Ready for Implementation
**Created:** 2026-01-25
**Last Updated:** 2026-01-25
**Priority:** High

---

## Vision

Build a complete **Chain Trading System** that operates independently of the legacy Ginie Autopilot, providing:

1. **Coin Profiler** - WebSocket-based real-time data collection driven by strategy requirements
2. **Entry Decision System Enhancement** - Strategy-first view with pattern/score-based entries
3. **Exit Decision System Integration** - Position monitoring for existing holdings
4. **Trading Control** - ON/OFF button to control entry execution

---

## Problem Statement

Current issues with the chain-based trading system:

1. **No Independent Data Collection** - Chain system relies on Ginie Autopilot's scanner
2. **No Real-time Data** - Current system uses polling, not WebSocket event-driven updates
3. **Strategy-Data Disconnect** - Strategies define data requirements but no system aggregates them
4. **Missing UI Integration** - Entry Decision System doesn't show strategy-specific coin matching
5. **No Pattern vs Score Distinction** - UI doesn't differentiate between pattern-based (100% match) and score-based strategies

### Key Insight from Discussion

The architecture follows a **Strategy-Driven (Reverse) Flow**:

```
Strategies (already configured)
    ↓ (Entry Decision System reads enabled strategies)
Entry Decision System (Brain/Mediator)
    ↓ (aggregates data requirements, sends to Coin Profiler)
Coin Profiler (Data Collector)
    ↓ (collects ONLY requested data via WebSocket)
Entry Decision System (receives data, makes decisions)
    ↓
Execute Entry via Chain System
```

---

## Architecture Overview

### System Components

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          FUTURES PAGE (Existing)                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  TRADE LIFECYCLE                          [Trading: ON / OFF]       │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ ▼ COIN PROFILER (NEW)                                    ● Live     │   │
│  │   WebSocket data collection for strategies + positions              │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ ▼ ENTRY DECISION SYSTEM (ENHANCED)                       ● Active   │   │
│  │   Strategy-first view: Mode → Strategy → Matching Coins             │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ ▶ ORDERS (Existing)                                                 │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ ▶ POSITIONS (Existing)                                              │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Data Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           COIN PROFILER                                      │
│                        (Central Data Hub)                                    │
│                                                                             │
│  Data Sources:                                                              │
│  ┌──────────────────────────────────┬────────────────────────────────────┐ │
│  │  FROM ENABLED STRATEGIES         │  FROM OPEN POSITIONS               │ │
│  │  (Entry Decision needs)          │  (Exit Decision needs)             │ │
│  ├──────────────────────────────────┼────────────────────────────────────┤ │
│  │  - Read enabled strategies       │  - Read open positions             │ │
│  │  - Aggregate timeframe needs     │  - Get position symbols            │ │
│  │  - Aggregate indicator needs     │  - Get exit timeframe needs        │ │
│  └──────────────────────────────────┴────────────────────────────────────┘ │
│                                                                             │
│  WebSocket Subscriptions:                                                   │
│  - Combined deduplicated symbol list                                        │
│  - Subscribe to required timeframes per symbol                              │
│  - Real-time delta updates (event-driven, NOT polling)                      │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Trading Control States

| Button State | Coin Profiler | Entry Decision | Exit Decision |
|--------------|---------------|----------------|---------------|
| **ON** | Runs (strategies + positions) | Active (new entries) | Active |
| **OFF** | Runs (positions only) | Paused (no new entries) | Active |

---

## Strategy Types

### Pattern-Based Strategies (100% Match Required)

Examples: Volume Imbalance (Ravindra), Breakout Patterns

- All conditions must be met (Step 1 ✓ → Step 2 ✓ → Step 3 ✓)
- Display: Step progress, not scores
- Entry triggered when: ALL steps complete

```
┌─────────┬───────────┬─────────────────┬─────────────────┐
│ Symbol  │ Status    │ Pattern         │ Progress        │
├─────────┼───────────┼─────────────────┼─────────────────┤
│ ETHUSDT │ ⚡ READY  │ Step 3 Complete │ All conditions  │
│ BTCUSDT │ Watching  │ Step 2          │ 3/6 candles     │
│ SOLUSDT │ Watching  │ Step 1          │ Accumulation    │
└─────────┴───────────┴─────────────────┴─────────────────┘
```

### Score-Based Strategies (Threshold Match)

Examples: Trend Following, Momentum

- Weighted criteria with partial matching
- Display: Score out of 100
- Entry triggered when: Score ≥ threshold (e.g., 75)

```
┌─────────┬───────┬─────────────────┬─────────────────┐
│ Symbol  │ Score │ Trend           │ Status          │
├─────────┼───────┼─────────────────┼─────────────────┤
│ BTCUSDT │ 82    │ Strong Bullish  │ ⚡ READY        │
│ ETHUSDT │ 68    │ Bullish         │ Below threshold │
│ SOLUSDT │ 54    │ Neutral         │ Watching        │
└─────────┴───────┴─────────────────┴─────────────────┘
```

---

## Mode + Strategy Combinations

Same strategy can exist in multiple modes with different timeframes:

```
Volume Imbalance Strategy:
├── Scalp Mode → 15m timeframe
├── Swing Mode → 1h timeframe
└── Position Mode → 4h timeframe

Coin Profiler Display for BTCUSDT:
├── Volume Imbalance (Scalp) - 15m - Step 2
├── Volume Imbalance (Swing) - 1h - Step 1
└── Classic Breakout (Scalp) - 5m - Near breakout
```

---

## Package Structure

```
internal/
├── coinprofiler/              # NEW PACKAGE
│   ├── profiler.go            # Main Coin Profiler service
│   ├── requirements.go        # Strategy requirement aggregation
│   ├── websocket.go           # Binance WebSocket handler
│   ├── store.go               # Redis storage for coin data
│   └── types.go               # Shared types
│
├── entrydecision/             # NEW PACKAGE
│   ├── system.go              # Entry Decision System core
│   ├── strategy_reader.go     # Reads enabled strategies
│   ├── pattern_matcher.go     # Pattern-based strategy matching
│   ├── score_calculator.go    # Score-based strategy calculation
│   └── entry_executor.go      # Triggers entries via Chain
│
├── exitdecision/              # NEW PACKAGE
│   ├── system.go              # Exit Decision System core
│   ├── position_reader.go     # Reads open positions (Epic 10)
│   └── exit_executor.go       # Executes exits
│
└── chaintrading/              # NEW PACKAGE
    ├── controller.go          # Trading ON/OFF control
    └── orchestrator.go        # Coordinates all components
```

---

## API Endpoints

### Chain Trading Control

```
POST /api/chain-trading/start     # Turn trading ON
POST /api/chain-trading/stop      # Turn trading OFF
GET  /api/chain-trading/status    # Get system status
```

### Coin Profiler

```
GET  /api/coin-profiler/status           # Profiler status
GET  /api/coin-profiler/coins            # All profiled coins
GET  /api/coin-profiler/coin/:symbol     # Single coin detail
GET  /api/coin-profiler/requirements     # Aggregated requirements
```

### Entry Decision Enhancement

```
GET  /api/entry-decision/strategies      # Enabled strategies with matching coins
GET  /api/entry-decision/candidates      # All entry candidates
```

---

## Stories

### PART A: Coin Profiler Core

#### Story 14.1: Coin Profiler - Core Service Structure
**Priority:** P0
**Status:** Ready for Implementation

Create the core Coin Profiler service with lifecycle management.

**Acceptance Criteria:**
- Create `internal/coinprofiler/` package
- CoinProfiler struct with Start(), Stop(), IsRunning() methods
- Integrate with UserAutopilotManager lifecycle
- Configuration for WebSocket connection settings
- Graceful shutdown handling

**Files to Create:**
- `internal/coinprofiler/profiler.go`
- `internal/coinprofiler/types.go`

---

#### Story 14.2: Coin Profiler - Strategy Requirement Aggregation
**Priority:** P0
**Status:** Ready for Implementation

Aggregate data requirements from all enabled strategies.

**Acceptance Criteria:**
- Read enabled strategies from database (use existing repository)
- Extract timeframe requirements per strategy
- Extract indicator/data requirements per strategy
- Combine and deduplicate requirements
- Support mode+strategy combinations (same strategy, different modes)

**Key Logic:**
```go
type StrategyRequirements struct {
    Mode        string   // "scalp", "swing", "position"
    Strategy    string   // "volume_imbalance", "breakout"
    SubStrategy string   // "ravindra_volume_imbalance"
    Timeframes  []string // ["5m", "15m", "1h"]
    DataFields  []string // ["volume", "taker_buy_volume", "ohlc"]
    Filters     map[string]interface{} // min_volume, etc.
}

func AggregateRequirements(strategies []StrategyRequirements) *AggregatedRequirements
```

**Files to Create:**
- `internal/coinprofiler/requirements.go`

---

#### Story 14.3: Coin Profiler - Position Requirement Aggregation
**Priority:** P0
**Status:** Ready for Implementation

Aggregate data requirements from open positions (for Exit Decision).

**Acceptance Criteria:**
- Read open positions from Epic 10's position tracking
- Extract symbols being held
- Determine timeframes needed for exit monitoring (TP/SL/Trailing)
- Combine with strategy requirements

**Key Logic:**
```go
type PositionRequirements struct {
    Symbol     string
    Timeframes []string // Exit monitoring timeframes
    ExitMode   string   // "tp_sl", "trailing", "both"
}

func GetPositionRequirements(positions []Position) []PositionRequirements
```

---

#### Story 14.4: Coin Profiler - WebSocket Data Collection
**Priority:** P0
**Status:** Ready for Implementation

Implement real-time WebSocket data collection from Binance.

**Acceptance Criteria:**
- Connect to Binance Futures WebSocket (wss://fstream.binance.com)
- Subscribe to kline streams for required symbol+timeframe combinations
- Event-driven updates (NO polling intervals)
- Delta updates only (update changed fields)
- Reconnection handling with exponential backoff
- Connection status tracking

**WebSocket Streams:**
```
Kline: {symbol}@kline_{interval}
Ticker: {symbol}@ticker
BookTicker: {symbol}@bookTicker
```

**Files to Create:**
- `internal/coinprofiler/websocket.go`

---

#### Story 14.5: Coin Profiler - Redis Storage
**Priority:** P0
**Status:** Ready for Implementation

Store profiled coin data in Redis for fast access.

**Acceptance Criteria:**
- Redis hash per coin: `profiler:coin:{symbol}`
- Store OHLCV data per timeframe
- Store calculated indicators
- Delta updates only (change detection)
- TTL management for stale data
- Atomic operations for consistency

**Redis Structure:**
```
profiler:coin:BTCUSDT
├── price: 43150.20
├── volume_24h: 2100000000
├── volatility: 2.4
├── timeframe:5m: {open, high, low, close, volume, taker_buy_vol}
├── timeframe:15m: {open, high, low, close, volume, taker_buy_vol}
├── timeframe:1h: {open, high, low, close, volume, taker_buy_vol}
├── source: "strategy,position" | "strategy" | "position"
└── updated_at: 1706200000000
```

**Files to Create:**
- `internal/coinprofiler/store.go`

---

#### Story 14.6: Coin Profiler - API Endpoints
**Priority:** P1
**Status:** Ready for Implementation

Create API endpoints for Coin Profiler.

**Acceptance Criteria:**
- GET /api/coin-profiler/status - WebSocket status, symbols count, update rate
- GET /api/coin-profiler/coins - List all profiled coins with summary
- GET /api/coin-profiler/coin/:symbol - Detailed coin data with all timeframes
- GET /api/coin-profiler/requirements - Show aggregated requirements

**Files to Create:**
- `internal/api/handlers_coin_profiler.go`

---

#### Story 14.7: Coin Profiler - Frontend UI Component
**Priority:** P1
**Status:** Ready for Implementation

Create React component for Coin Profiler expandable card.

**Acceptance Criteria:**
- Expandable card in Trade Lifecycle section (before Entry Decision)
- Show connection status, symbols count, update rate
- Show data sources (from strategies vs positions)
- Expandable coin list with columns: Symbol, Price, Vol 24h, Volatility, Source, Strategies Matched
- Click on coin row to expand and show detailed data + strategy matching

**Files to Create:**
- `web/src/components/CoinProfiler/CoinProfilerCard.tsx`
- `web/src/components/CoinProfiler/CoinList.tsx`
- `web/src/components/CoinProfiler/CoinDetail.tsx`

---

### PART B: Entry Decision System Enhancement

#### Story 14.8: Entry Decision - Strategy-First Data Structure
**Priority:** P0
**Status:** Ready for Implementation

Create data structures for strategy-first view.

**Acceptance Criteria:**
- Define StrategyMatch type (coins matching a strategy)
- Support pattern-based strategies (step progress, not scores)
- Support score-based strategies (score values)
- Handle mode+strategy combinations (same strategy in different modes)

**Key Types:**
```go
type StrategyMatch struct {
    Mode        string           // "scalp"
    Strategy    string           // "volume_imbalance"
    SubStrategy string           // "ravindra_volume_imbalance"
    Type        string           // "pattern" | "score"
    Timeframe   string           // "15m"
    Coins       []CoinMatch      // Matching coins
}

type CoinMatch struct {
    Symbol  string
    // For pattern-based:
    Step    int    // 1, 2, 3
    Status  string // "accumulation", "consolidating", "ready"
    Details string // "3/6 candles"
    // For score-based:
    Score   int    // 0-100
    Ready   bool   // Score >= threshold
}
```

**Files to Create:**
- `internal/entrydecision/types.go`

---

#### Story 14.9: Entry Decision - Pattern Matcher (Volume Imbalance)
**Priority:** P0
**Status:** Ready for Implementation

Implement pattern matching for Volume Imbalance strategy using Coin Profiler data.

**Acceptance Criteria:**
- Read coin data from Coin Profiler (Redis)
- Apply Volume Imbalance 3-step pattern detection
- Track step progress per coin
- Detect READY status when all steps complete
- Support different timeframes per mode

**Pattern Detection:**
```
Step 1: Volume Spike (≥2x average) → ACCUMULATION
Step 2: Consolidation (2-6 candles, declining volume) → CONSOLIDATING
Step 3: Breakout (volume surge + price break) → READY
```

**Files to Create:**
- `internal/entrydecision/pattern_volume_imbalance.go`

---

#### Story 14.10: Entry Decision - Score Calculator (Trend Following)
**Priority:** P1
**Status:** Ready for Implementation

Implement score calculation for trend-based strategies.

**Acceptance Criteria:**
- Read coin data from Coin Profiler (Redis)
- Calculate weighted score components
- Apply threshold for READY status
- Support configurable thresholds per strategy

**Files to Create:**
- `internal/entrydecision/score_calculator.go`

---

#### Story 14.11: Entry Decision - Strategy Reader & Matcher
**Priority:** P0
**Status:** Ready for Implementation

Read enabled strategies and match coins to each.

**Acceptance Criteria:**
- Read enabled strategies from database
- For each strategy, get matching coins from Coin Profiler
- Apply appropriate matcher (pattern or score)
- Return structured data for UI display

**Files to Create:**
- `internal/entrydecision/strategy_reader.go`
- `internal/entrydecision/matcher.go`

---

#### Story 14.12: Entry Decision - API Enhancement
**Priority:** P0
**Status:** Ready for Implementation

Enhance Entry Decision API for strategy-first view.

**Acceptance Criteria:**
- GET /api/entry-decision/strategies - Returns all enabled strategies with matching coins
- Response structure supports both pattern and score types
- Include mode+strategy combinations

**Response Example:**
```json
{
  "strategies": [
    {
      "mode": "scalp",
      "strategy": "volume_imbalance",
      "sub_strategy": "ravindra_volume_imbalance",
      "type": "pattern",
      "timeframe": "15m",
      "coins": [
        {"symbol": "ETHUSDT", "step": 3, "status": "ready", "details": "All conditions met"},
        {"symbol": "BTCUSDT", "step": 2, "status": "consolidating", "details": "3/6 candles"}
      ]
    },
    {
      "mode": "swing",
      "strategy": "trend_following",
      "type": "score",
      "timeframe": "1h",
      "threshold": 75,
      "coins": [
        {"symbol": "BTCUSDT", "score": 82, "ready": true},
        {"symbol": "ETHUSDT", "score": 68, "ready": false}
      ]
    }
  ]
}
```

---

#### Story 14.13: Entry Decision - Frontend UI Enhancement
**Priority:** P0
**Status:** Ready for Implementation

Enhance Entry Decision System UI with strategy-first view.

**Acceptance Criteria:**
- Expandable card showing modes (Scalp, Swing, Position)
- Under each mode: list of enabled strategies
- Under each strategy: matching coins with pattern/score status
- Different display for pattern-based (steps) vs score-based (score value)
- Highlight READY coins with indicator
- Show requirements: timeframes, filters

**UI Structure:**
```
▼ SCALP MODE [2 strategies]
  ├── ▼ Volume Imbalance (Ravindra) [Active]
  │     Requirements: 15m, 1h | Min Volume 1M
  │     ┌─────────┬───────────┬─────────────────┬───────────┐
  │     │ Symbol  │ Status    │ Pattern         │ Action    │
  │     ├─────────┼───────────┼─────────────────┼───────────┤
  │     │ ETHUSDT │ ⚡ READY  │ Step 3 Complete │ [ENTER →] │
  │     │ BTCUSDT │ Watching  │ Step 2          │ -         │
  │     └─────────┴───────────┴─────────────────┴───────────┘
  │
  └── ▶ Classic Breakout [Active]

▶ SWING MODE [1 strategy]
▶ POSITION MODE [0 strategies]
```

**Files to Create/Modify:**
- `web/src/components/EntryDecision/StrategyView.tsx`
- `web/src/components/EntryDecision/PatternCoinList.tsx`
- `web/src/components/EntryDecision/ScoreCoinList.tsx`

---

### PART C: Trading Control & Integration

#### Story 14.14: Trading Controller - ON/OFF Button
**Priority:** P0
**Status:** Ready for Implementation

Implement Trading ON/OFF control.

**Acceptance Criteria:**
- Trading ON: Entry Decision active, places new entries
- Trading OFF: Entry Decision paused, no new entries
- Both states: Coin Profiler runs, Exit Decision runs
- Persist state in database
- API endpoints for control

**Files to Create:**
- `internal/chaintrading/controller.go`

---

#### Story 14.15: Trading Controller - Frontend Integration
**Priority:** P0
**Status:** Ready for Implementation

Add Trading ON/OFF toggle to Trade Lifecycle header.

**Acceptance Criteria:**
- Toggle button at Trade Lifecycle section header
- Visual indicator of current state
- Confirmation dialog before turning OFF
- Show impact: "Entry will be paused, positions will continue to be managed"

---

#### Story 14.16: Exit Decision - Integration with Epic 10
**Priority:** P1
**Status:** Ready for Implementation

Integrate Exit Decision with existing position management (Epic 10).

**Acceptance Criteria:**
- Exit Decision reads positions from Epic 10
- Sends position symbol requirements to Coin Profiler
- Uses real-time price data for TP/SL monitoring
- Triggers exits via existing mechanisms

**Files to Create:**
- `internal/exitdecision/system.go`
- `internal/exitdecision/position_reader.go`

---

### PART D: Wiring & Main Integration

#### Story 14.17: Main.go Integration
**Priority:** P0
**Status:** Ready for Implementation

Wire all new components into main.go.

**Acceptance Criteria:**
- Create and start Coin Profiler
- Connect Entry Decision to Coin Profiler
- Connect Exit Decision to Coin Profiler
- Integrate Trading Controller
- Proper lifecycle management (start/stop order)
- Graceful shutdown handling

---

#### Story 14.18: Database Migrations
**Priority:** P0
**Status:** Ready for Implementation

Add necessary database tables/columns.

**Acceptance Criteria:**
- Table for trading control state (ON/OFF per user)
- Any additional columns needed for pattern tracking

---

## Dependencies

| Story | Depends On |
|-------|-----------|
| 14.4 (WebSocket) | 14.1 (Core), 14.2 (Requirements) |
| 14.5 (Redis) | 14.4 (WebSocket) |
| 14.6 (API) | 14.5 (Redis) |
| 14.7 (Frontend) | 14.6 (API) |
| 14.9 (Pattern Matcher) | 14.5 (Redis) |
| 14.11 (Matcher) | 14.9, 14.10 |
| 14.12 (API) | 14.11 (Matcher) |
| 14.13 (Frontend) | 14.12 (API) |
| 14.14 (Controller) | 14.11 (Matcher) |
| 14.16 (Exit Decision) | 14.5 (Redis), Epic 10 |

---

## Existing System Dependencies

| Component | Epic | Description |
|-----------|------|-------------|
| Strategy Hierarchy | Epic 11 | Mode → Strategy → Sub-Strategy configuration |
| Position Management | Epic 10 | Open position tracking, P&L |
| Redis Infrastructure | Epic 6 | Redis connection, caching patterns |
| WebSocket Base | Epic 12 | Existing WebSocket patterns (if any) |
| Order Chains | Epic 7 | ChainEventWriter for entry execution |

---

## Key Design Decisions

1. **Real-time WebSocket, NOT polling** - Event-driven updates, no scan intervals
2. **Strategy-Driven Data Collection** - Only collect data that strategies need
3. **Pattern vs Score distinction** - Different display for different strategy types
4. **Integrated in existing Futures page** - No new pages, uses expandable cards
5. **Coin Profiler serves both Entry and Exit** - Single data source for all decisions

---

## Success Metrics

- WebSocket latency < 100ms for data updates
- Coin Profiler updates visible in UI within 1 second
- Pattern detection accuracy matches Volume Imbalance spec
- Trading ON/OFF state persists across restarts
- No polling intervals in the system (pure event-driven)
