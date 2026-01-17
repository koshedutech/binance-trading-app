# Epic 11: Position Decision Engine

## Epic Overview

**Epic ID:** EPIC-11
**Status:** Ready for Implementation
**Created:** 2026-01-15
**Last Updated:** 2026-01-15
**Priority:** High

---

## Vision

Build a comprehensive Position Decision Engine that replaces the current multiplicative scoring system with:
1. **Redis-First State Management** - Always-current coin state with delta updates
2. **Market Regime Detection** - Auto-classify markets as Trending, Ranging, Volatile, Consolidating
3. **Multi-Strategy Framework** - Different strategies for different market conditions
4. **Configurable Indicators** - User-selectable indicators per segment with averaging
5. **Additive Scoring Formula** - Technical(40) + Context(30) + LLM(20) + History(10)
6. **Calibration Layer** - Learn from trade outcomes to map scores to win probabilities

---

## Problem Statement

Current scoring system issues identified:

1. **Multiplicative Formula** - `StrengthScore × (scan.Score / 100) × adxPenalty` always produces low results
2. **100% Rejection Rate** - 876 blocked entries, 510 rejected signals, 0 passed
3. **LLM Returns Neutral** - Missing context (ADX, trend alignment, market structure, BTC trend)
4. **Single Strategy** - Same logic for all market conditions (trend following only)
5. **No Learning** - System doesn't improve from trade outcomes

### Blocking Reasons Analysis

| Reason | Percentage | Observation |
|--------|------------|-------------|
| Trend Divergence (1h vs 15m) | ~50% | Hard block |
| No Trend/Low ADX (<15) | ~17% | Hard block |
| Entry Confluence (low score) | ~8% | Soft block |
| LLM Confidence | ~25% | Always neutral with 45% |

**Root Cause:** Multiplicative formula + missing LLM context = scores of 13-41% (need 55+)

---

## Architecture Overview

### Data Flow

```
BINANCE WEBSOCKET          REDIS STATE              DECISION ENGINE
     │                         │                          │
     │ Price ticks             │ Coin State               │
     │ ─────────────────────>  │ ─ symbol                 │
     │ Klines                  │ ─ market_regime          │
     │ OrderBook               │ ─ indicators             │
     │                         │ ─ scores                 │
     │                         │ ─ blocking_reasons       │
     │                         │                          │
     │                         │ Delta Updates Only       │
     │                         │ ─────────────────────>   │ Strategy Selection
     │                         │                          │ Score Calculation
     │                         │                          │ Decision Output
```

### System Position

```
COIN SEARCH → [POSITION DECISION ENGINE] → ORDER EXECUTION
                      │
                      ├── Market Regime Detection
                      ├── Strategy Selection
                      ├── Indicator Processing
                      ├── Score Calculation
                      ├── Calibration Layer
                      └── Decision Output
```

---

## Stories

### PART A: Infrastructure

#### Story 11.1: Redis State Management
**Priority:** P0
**Status:** Ready for Implementation

Build Redis-first state architecture for all monitored coins.

**Acceptance Criteria:**
- Redis hash per coin: `coin:state:{symbol}`
- Store: price, indicators, scores, regime, blocking reasons
- Delta updates only (update changed fields, not full recalculation)
- TTL management for stale data cleanup
- Atomic operations for consistency

**Key Fields:**
```
coin:state:BTCUSDT
├── price: 45000.00
├── regime: TRENDING
├── active_strategy: trend_following
├── adx: 28.5
├── rsi: 62.3
├── ema_9: 44850.00
├── ema_21: 44720.00
├── trend_1h: BULLISH
├── trend_15m: BULLISH
├── score_technical: 72
├── score_context: 65
├── score_llm: 58
├── score_history: 70
├── score_final: 68
├── blocking_reasons: []
├── last_updated: 1705312800000
└── decision: READY
```

---

#### Story 11.2: Delta Update Processor
**Priority:** P0
**Status:** Ready for Implementation

Implement efficient delta processing for state updates.

**Acceptance Criteria:**
- Compare new values vs cached values
- Only update changed fields in Redis
- Batch multiple field updates in single HSET
- Track update frequency per field
- Performance target: < 1ms per update

**Delta Logic:**
```go
type DeltaProcessor struct {
    cache map[string]map[string]interface{}
}

func (d *DeltaProcessor) Process(symbol string, newState map[string]interface{}) []string {
    // Returns only changed field names
    // Updates Redis with changed fields only
}
```

---

#### Story 11.3: WebSocket State Sync
**Priority:** P0
**Status:** Ready for Implementation

Connect WebSocket streams to Redis state management.

**Acceptance Criteria:**
- Price updates trigger indicator recalculation
- Kline closes trigger trend/regime analysis
- Order book updates for spread/liquidity metrics
- Throttling to prevent excessive updates
- Graceful degradation on connection issues

---

### PART B: Market Regime Detection

#### Story 11.4: Market Regime Classifier
**Priority:** P1
**Status:** Ready for Implementation

Implement automatic market regime detection for each coin.

**Regimes:**
| Regime | Characteristics | Preferred Strategy |
|--------|-----------------|-------------------|
| TRENDING | ADX > 25, directional movement | Trend Following |
| RANGING | ADX < 20, price oscillating | Mean Reversion |
| VOLATILE | High ATR, wide swings | Breakout |
| CONSOLIDATING | Low ATR, tight range | Range Trading |

**Acceptance Criteria:**
- Real-time regime classification
- Regime stored in Redis state
- Regime change triggers strategy re-evaluation
- Historical regime tracking for analysis
- Configurable thresholds per timeframe

---

#### Story 11.5: Regime Transition Handler
**Priority:** P1
**Status:** Ready for Implementation

Handle transitions between market regimes gracefully.

**Acceptance Criteria:**
- Detect regime changes with confirmation (avoid whipsaws)
- Minimum time in regime before allowing transition
- Notify active positions of regime changes
- Log regime transitions for analysis
- Optional alerts for significant regime changes

---

### PART C: Multi-Strategy Framework

#### Story 11.6: Strategy Registry
**Priority:** P1
**Status:** Ready for Implementation

Create pluggable strategy architecture.

**Acceptance Criteria:**
- Strategy interface definition
- Registry for available strategies
- Strategy selection based on regime
- Strategy configuration storage
- Easy addition of new strategies

**Interface:**
```go
type Strategy interface {
    Name() string
    SupportedRegimes() []MarketRegime
    RequiredIndicators() []string
    CalculateScore(state *CoinState) StrategyScore
    GetEntryConditions() []Condition
    GetExitConditions() []Condition
}
```

---

#### Story 11.7: Strategy Selector
**Priority:** P1
**Status:** Ready for Implementation

Automatic strategy selection based on market regime.

**Acceptance Criteria:**
- Match regime to supported strategies
- User preference override capability
- Fallback strategy when no match
- Strategy switch cooldown (prevent rapid switching)
- Log strategy selections for analysis

---

### PART D: Strategy Implementations

#### Story 11.8: Trend Following Strategy
**Priority:** P1
**Status:** Ready for Implementation

Implement trend following strategy (current approach, refined).

**Entry Conditions:**
- ADX > 25 (strong trend)
- Price above EMA 21 (for longs)
- RSI 40-70 (not overbought)
- Volume confirmation
- Trend alignment (15m = 1h direction)

**Exit Conditions:**
- Trend reversal detection
- ADX drops below 20
- Price crosses EMA 21
- Trailing stop hit

---

#### Story 11.9: Mean Reversion Strategy
**Priority:** P2
**Status:** Planning

Implement mean reversion for ranging markets.

**Entry Conditions:**
- ADX < 20 (no trend)
- RSI < 30 (oversold) or RSI > 70 (overbought)
- Price at Bollinger Band extremes
- Support/resistance proximity

**Exit Conditions:**
- Return to mean (middle Bollinger)
- RSI returns to neutral (40-60)
- Time-based exit (ranging trades shouldn't last long)

---

#### Story 11.10: Breakout Strategy
**Priority:** P2
**Status:** Planning

Implement breakout detection for volatile markets.

**Entry Conditions:**
- ATR expansion (above average)
- Volume spike (> 2x average)
- Price breaks key level
- Momentum confirmation

**Exit Conditions:**
- Failed breakout (price returns inside range)
- Target reached (1.5-2x ATR)
- Volume dies down

---

#### Story 11.11: Range Trading Strategy
**Priority:** P2
**Status:** Planning

Implement range trading for consolidating markets.

**Entry Conditions:**
- Clear support/resistance levels
- Price at range boundary
- Low ATR (tight range)
- Reversal candlestick patterns

**Exit Conditions:**
- Opposite range boundary
- Range breakout
- Time-based exit

---

### PART E: Configurable Indicators

#### Story 11.12: Indicator Segment Framework
**Priority:** P1
**Status:** Ready for Implementation

Allow users to select 2-3 indicators per segment and average them.

**Segments:**
| Segment | Purpose | Default Indicators |
|---------|---------|-------------------|
| Trend | Direction determination | EMA Cross, MACD, SuperTrend |
| Momentum | Strength measurement | RSI, Stochastic, CCI |
| Volatility | Risk assessment | ATR, Bollinger Width, Keltner |
| Volume | Confirmation | OBV, Volume SMA, VWAP |

**Acceptance Criteria:**
- User can select 2-3 indicators per segment
- System calculates average or weighted average
- Indicator weights configurable
- Track which combinations perform best
- Global learning across all users (aggregated)

---

#### Story 11.13: Indicator Calculation Engine
**Priority:** P1
**Status:** Ready for Implementation

Efficient indicator calculation with caching.

**Acceptance Criteria:**
- Calculate all selected indicators per coin
- Cache intermediate values (avoid recalculation)
- Incremental updates on new data
- Normalization to 0-100 scale
- Performance target: < 5ms for full indicator set

---

#### Story 11.14: Indicator Performance Tracker
**Priority:** P2
**Status:** Planning

Track which indicator combinations perform best.

**Acceptance Criteria:**
- Log indicator values at entry time
- Correlate with trade outcomes
- Identify high-performing combinations
- Surface recommendations to users
- Optional auto-optimization mode

---

### PART F: Scoring & Decision

#### Story 11.15: Additive Score Calculator
**Priority:** P0
**Status:** Ready for Implementation

Replace multiplicative formula with additive scoring.

**New Formula:**
```
FINAL_SCORE = Technical(40) + Context(30) + LLM(20) + History(10)

Technical (0-40 points):
- Trend alignment: 0-15
- Momentum: 0-10
- Volatility: 0-10
- Volume: 0-5

Context (0-30 points):
- Market regime match: 0-10
- Timeframe alignment: 0-10
- BTC/market trend: 0-10

LLM (0-20 points):
- LLM confidence: 0-20 (with enhanced context)

History (0-10 points):
- Symbol performance: 0-5
- Strategy performance: 0-5
```

**Acceptance Criteria:**
- Each component scored independently
- Component breakdown visible in UI
- Configurable weights per strategy
- Score stored in Redis state
- Historical score tracking

---

#### Story 11.16: Enhanced LLM Context
**Priority:** P1
**Status:** Ready for Implementation

Provide comprehensive context to LLM for better decisions.

**Additional Context:**
- ADX value and interpretation
- Price position vs EMAs
- VWAP position
- Higher timeframe trend
- Market structure (HH/HL or LH/LL)
- BTC trend and correlation
- Recent support/resistance levels
- Volume analysis

**Acceptance Criteria:**
- Structured context object for LLM
- All relevant data included
- Clear formatting for LLM parsing
- Reduced "neutral" responses
- Improved directional confidence

---

#### Story 11.17: Calibration Layer
**Priority:** P1
**Status:** Ready for Implementation

Learn from trade outcomes to improve score interpretation.

**Mechanism:**
```
Score 75-100 → Historical win rate: 72% → Calibrated confidence: 72%
Score 50-74  → Historical win rate: 58% → Calibrated confidence: 58%
Score 25-49  → Historical win rate: 41% → Calibrated confidence: 41%
Score 0-24   → Historical win rate: 23% → Calibrated confidence: 23%
```

**Acceptance Criteria:**
- Track entry score → actual outcome
- Build score-to-probability mapping
- Update calibration periodically (daily/weekly)
- Separate calibration per strategy
- Display calibrated confidence in UI

---

#### Story 11.18: Blocking Reason Tracker
**Priority:** P0
**Status:** Ready for Implementation

Clear tracking of why signals are blocked.

**Blocking Categories:**
| Category | Type | Example |
|----------|------|---------|
| Hard Block | Cannot override | Trend divergence, ADX < threshold |
| Soft Block | Can override with confirmation | Score below threshold |
| Warning | Informational | Low volume, wide spread |

**Acceptance Criteria:**
- All blocking reasons stored in Redis
- Category and severity tagged
- UI displays blocking reasons clearly
- Historical blocking analysis
- Configurable block thresholds

---

### PART G: Execution & Tracking

#### Story 11.19: Decision Output Interface
**Priority:** P1
**Status:** Ready for Implementation

Clean interface between Decision Engine and Order Execution.

**Decision Object:**
```go
type Decision struct {
    Symbol          string
    Action          string  // ENTER_LONG, ENTER_SHORT, HOLD, EXIT
    Score           int
    CalibratedConf  float64
    Strategy        string
    Regime          MarketRegime
    BlockingReasons []BlockingReason
    Indicators      map[string]float64
    Timestamp       int64
}
```

**Acceptance Criteria:**
- Standardized decision format
- All context included for execution
- Audit trail for every decision
- Integration with existing order execution
- Rollback capability

---

#### Story 11.20: Actor Tracking System
**Priority:** P1
**Status:** Ready for Implementation

Track whether trades are initiated by User or Ginie Auto.

**Actors:**
| Actor | Description |
|-------|-------------|
| USER | Manual trade via UI |
| GINIE_AUTO | Automatic trade by autopilot |
| GINIE_SUGGESTED | User accepted Ginie suggestion |

**Acceptance Criteria:**
- Actor stored with every trade
- Filter trades by actor in history
- Separate performance metrics per actor
- UI shows actor badge on positions
- Analytics by actor type

---

#### Story 11.21: Position Card UI
**Priority:** P2
**Status:** Planning

Unified UI for position management.

**Features:**
- Collapsible cards per coin
- Show current state (regime, score, indicators)
- Display blocking reasons with explanations
- Manual + Auto trading through same interface
- Real-time updates via WebSocket

---

#### Story 11.22: Performance Dashboard
**Priority:** P2
**Status:** Planning

Analytics dashboard for decision engine performance.

**Metrics:**
- Score distribution analysis
- Calibration accuracy over time
- Strategy performance comparison
- Regime classification accuracy
- Blocking reason frequency

---

### PART H: Settings & Calibration Management

#### Story 11.23: Decision Engine Settings Structure
**Priority:** P0
**Status:** Ready for Implementation

Create settings structure for Decision Engine (mirrors mode_configs pattern).

**Settings Flow:**
```
NEW USER:
default-settings.json → Copy to user's DB (user_settings table)

USER LOGIN:
User's DB settings → Load to Redis cache → System reads from Redis

RESET/RESTORE:
default-settings.json → Replace user's DB → Update Redis cache
```

**JSON Structure (in default-settings.json):**
```json
{
  "decision_engine": {
    "strategies": {
      "trend_following": {
        "strategy_name": "trend_following",
        "enabled": true,
        "market_regime": {
          "adx_min": 25,
          "adx_max": 100,
          "volatility_min": "medium",
          "volatility_max": "high",
          "regime_confirmation_candles": 3
        },
        "indicators": {
          "trend": {
            "selected": ["ema_cross", "macd"],
            "averaging_method": "weighted",
            "weights": {"ema_cross": 0.6, "macd": 0.4}
          },
          "momentum": {
            "selected": ["rsi"],
            "averaging_method": "simple",
            "weights": {}
          },
          "volatility": {
            "selected": ["atr"],
            "averaging_method": "simple",
            "weights": {}
          },
          "volume": {
            "selected": ["vwap"],
            "averaging_method": "simple",
            "weights": {}
          }
        },
        "entry_conditions": {
          "rsi_min": 40,
          "rsi_max": 70,
          "require_trend_alignment": true,
          "volume_confirmation": true,
          "ema_period": 21
        },
        "exit_conditions": {
          "adx_exit_threshold": 20,
          "trailing_enabled": true,
          "trend_reversal_exit": true,
          "time_based_exit_enabled": false,
          "max_hold_duration": ""
        },
        "scoring": {
          "technical_weight": 40,
          "context_weight": 30,
          "llm_weight": 20,
          "history_weight": 10,
          "min_score": 55,
          "high_confidence_score": 75,
          "ultra_confidence_score": 90
        },
        "calibration_config": {
          "enabled": true,
          "learning_window_hours": 168,
          "min_trades_for_calibration": 50,
          "update_interval_hours": 24
        }
      },
      "mean_reversion": {
        "strategy_name": "mean_reversion",
        "enabled": false,
        "market_regime": {...},
        "indicators": {...},
        "entry_conditions": {...},
        "exit_conditions": {...},
        "scoring": {...},
        "calibration_config": {...}
      },
      "breakout": {
        "strategy_name": "breakout",
        "enabled": false,
        ...
      },
      "range_trading": {
        "strategy_name": "range_trading",
        "enabled": false,
        ...
      }
    }
  }
}
```

**Acceptance Criteria:**
- Structure mirrors `mode_configs` pattern (strategy = mode equivalent)
- Each strategy has complete, independent settings
- Settings stored in user_settings table as JSONB
- On login, load user's settings to Redis
- On reset, copy from default-settings.json to user DB then update Redis
- API endpoints for CRUD operations on strategy settings

**Redis Key Structure (follows JSON pattern):**
```
decision_engine:settings:{user_id}
├── strategies.trend_following.enabled
├── strategies.trend_following.market_regime.adx_min
├── strategies.trend_following.indicators.trend.selected
├── strategies.trend_following.scoring.min_score
└── ... (all paths from JSON)
```

---

#### Story 11.24: Decision Engine Settings UI
**Priority:** P1
**Status:** Ready for Implementation

Add Decision Engine section to Reset Settings page and strategy-specific settings cards.

**UI Components:**

1. **Reset Settings Page - New Section:**
```
┌─────────────────────────────────────────────────────────────────┐
│ Decision Engine Settings                    [Reset All Strategies]
│ ┌─────────────────────────────────────────────────────────────┐
│ │ Strategy: Trend Following               ✓ Enabled  [Reset]  │
│ │ 45/52 settings match defaults                               │
│ │ ┌───────────────────────────────────────────────────────┐   │
│ │ │ ▶ Market Regime (5)              All Match            │   │
│ │ │ ▶ Indicators (8)                 3 Differences        │   │
│ │ │ ▶ Entry Conditions (5)           1 Difference         │   │
│ │ │ ▶ Exit Conditions (5)            All Match            │   │
│ │ │ ▶ Scoring (6)                    2 Differences        │   │
│ │ │ ▶ Calibration Config (4)         1 Difference         │   │
│ │ └───────────────────────────────────────────────────────┘   │
│ └─────────────────────────────────────────────────────────────┘
│ ┌─────────────────────────────────────────────────────────────┐
│ │ Strategy: Mean Reversion              ✗ Disabled  [Reset]  │
│ │ ...                                                         │
│ └─────────────────────────────────────────────────────────────┘
└─────────────────────────────────────────────────────────────────┘
```

2. **Ginie Autopilot Page - Quick Settings:**
- Strategy enable/disable toggle
- Key parameter adjustments inline
- Reset to defaults button per strategy

3. **Indicator Selection UI:**
- If ≤ 3 options available: Checkboxes
- If > 3 options: Multi-select dropdown
- Weight sliders when weighted averaging selected

**Acceptance Criteria:**
- New "Decision Engine" section in SettingsComparisonView
- Strategy cards expandable like mode cards
- Group-level reset (Market Regime, Indicators, etc.)
- Show current vs default comparison
- Admin can edit defaults (like existing mode editing)
- Real-time updates to Redis on save

---

#### Story 11.25: Calibration Data Storage
**Priority:** P1
**Status:** Ready for Implementation

Separate storage for learned calibration data (not user settings).

**Calibration Data is NOT Settings:**
- Settings = User configurable (stored in user DB, copied from defaults)
- Calibration = System learned (stored separately, per strategy + indicators)

**Storage Structure:**
```
calibration:{user_id}:{strategy}:{indicator_hash}
│
├── score_bucket_0_25
│   ├── total_trades: 100
│   ├── winning_trades: 23
│   ├── win_rate: 0.23
│   └── avg_pnl_percent: -1.2
│
├── score_bucket_26_50
│   ├── total_trades: 150
│   ├── winning_trades: 62
│   ├── win_rate: 0.41
│   └── avg_pnl_percent: 0.3
│
├── score_bucket_51_75
│   ├── total_trades: 200
│   ├── winning_trades: 116
│   ├── win_rate: 0.58
│   └── avg_pnl_percent: 1.1
│
├── score_bucket_76_100
│   ├── total_trades: 80
│   ├── winning_trades: 58
│   ├── win_rate: 0.72
│   └── avg_pnl_percent: 2.4
│
└── metadata
    ├── last_updated: "2026-01-15T10:00:00Z"
    ├── first_trade_date: "2026-01-01T00:00:00Z"
    └── total_calibration_trades: 530
```

**Indicator Hash:**
- Hash of selected indicators: `sha256("rsi+ema_cross+atr")` → first 8 chars
- When user changes indicator selection → new calibration starts fresh
- Old calibration data retained for rollback

**Acceptance Criteria:**
- Calibration data stored in PostgreSQL (calibration_data table) + Redis cache
- Per strategy, per indicator combination
- Automatic updates after each trade close
- Score bucket boundaries configurable
- Migration path when indicators change
- UI shows current calibration accuracy

---

#### Story 11.26: Calibration Data Lifecycle
**Priority:** P1
**Status:** Ready for Implementation

Manage calibration data updates and indicator changes.

**Lifecycle Events:**

1. **Trade Closes:**
```
Trade closes with score 67, result: WIN
→ Update calibration:{user_id}:trend_following:{hash}
→ Increment score_bucket_51_75.total_trades
→ Increment score_bucket_51_75.winning_trades
→ Recalculate win_rate
→ Update metadata.last_updated
```

2. **Indicator Selection Changes:**
```
User changes trend indicators from [ema_cross, macd] to [supertrend]
→ Calculate new indicator_hash
→ Check if calibration exists for new hash
→ If not, initialize fresh calibration buckets (all zeros)
→ Old calibration retained (can be viewed in history)
```

3. **Calibration Reset:**
```
User manually resets calibration
→ Zero out all buckets for current indicator hash
→ Keep metadata showing reset date
→ Archive old calibration for analysis
```

**PostgreSQL Table:**
```sql
CREATE TABLE calibration_data (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    strategy VARCHAR(50) NOT NULL,
    indicator_hash VARCHAR(16) NOT NULL,
    indicator_list JSONB NOT NULL,  -- ["rsi", "ema_cross", "atr"]
    bucket_0_25 JSONB NOT NULL,
    bucket_26_50 JSONB NOT NULL,
    bucket_51_75 JSONB NOT NULL,
    bucket_76_100 JSONB NOT NULL,
    metadata JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, strategy, indicator_hash)
);
```

**Acceptance Criteria:**
- Trade outcome automatically updates calibration
- Indicator change triggers new calibration initialization
- Manual reset option in UI
- Historical calibration data retained
- Calibration confidence indicator (needs N trades before reliable)

---

#### Story 11.27: Calibration Display & UI
**Priority:** P2
**Status:** Planning

Show calibration data and accuracy in UI.

**UI Elements:**

1. **Calibration Card (per strategy):**
```
┌─────────────────────────────────────────────────────────────────┐
│ Calibration: Trend Following                                    │
│ Indicators: RSI + EMA Cross + ATR                               │
│ Total Trades: 530 | Last Updated: 2 hours ago                   │
├─────────────────────────────────────────────────────────────────┤
│ Score Range │ Trades │ Win Rate │ Expected │ Accuracy          │
│ 0-25        │ 100    │ 23%      │ 20%      │ ✓ Good            │
│ 26-50       │ 150    │ 41%      │ 40%      │ ✓ Good            │
│ 51-75       │ 200    │ 58%      │ 60%      │ ✓ Good            │
│ 76-100      │ 80     │ 72%      │ 75%      │ ✓ Good            │
├─────────────────────────────────────────────────────────────────┤
│ [View History]  [Reset Calibration]  [Export Data]              │
└─────────────────────────────────────────────────────────────────┘
```

2. **Calibration Warning Banner:**
- Shows when < minimum trades for reliable calibration
- "Calibration needs 50 more trades to be reliable"

3. **Indicator Change Notice:**
- "You changed indicators. New calibration started."
- Option to view previous calibration

**Acceptance Criteria:**
- Clear visualization of calibration accuracy
- Warning when calibration is unreliable (low trade count)
- History of calibration changes
- Export calibration data for analysis
- Confidence indicator in trading UI

---

## Implementation Phases

### Phase 1: Foundation (Stories 11.1-11.3, 11.15, 11.18, 11.23)
- Redis state management
- Delta updates
- Additive scoring
- Blocking reason tracker
- Settings structure (default-settings.json)

### Phase 2: Intelligence (Stories 11.4-11.7, 11.16-11.17, 11.25-11.26)
- Market regime detection
- Strategy framework
- Enhanced LLM context
- Calibration layer storage & lifecycle

### Phase 3: Strategies (Stories 11.8-11.11)
- Trend following (refined)
- Mean reversion
- Breakout
- Range trading

### Phase 4: Configurability (Stories 11.12-11.14)
- Indicator segment framework
- Calculation engine
- Performance tracker

### Phase 5: Integration (Stories 11.19-11.22, 11.24, 11.27)
- Decision output interface
- Actor tracking
- Position card UI
- Performance dashboard
- Decision Engine Settings UI
- Calibration Display UI

---

## Performance Targets

| Metric | Target |
|--------|--------|
| State update latency | < 1ms |
| Full indicator calculation | < 5ms |
| Score calculation | < 2ms |
| Total decision time | < 10ms |
| Redis memory (600 coins) | < 10MB |
| Startup time (500 coins) | < 10 seconds |

---

## Success Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Signal pass rate | 0% | 15-25% |
| False positive rate | N/A | < 20% |
| Win rate (passed signals) | N/A | > 55% |
| Average score accuracy | N/A | Within 10% of actual |
| Regime classification accuracy | N/A | > 80% |

---

## Dependencies

| Dependency | Status | Notes |
|------------|--------|-------|
| Redis infrastructure | Existing | Already in use |
| Binance WebSocket | Existing | Already subscribed |
| LLM Integration | Existing | Needs context enhancement |
| Order Execution | Existing | Interface integration |
| Epic 10 (Position Management) | In Progress | Complementary system |

---

## Architecture: Two Separate Models

This is a **completely new and separate** decision engine, NOT a modification of the existing one.

```
SCANNING (Coin Search)
         │
         ▼
    ┌─────────┐
    │ SWITCH  │  ← User selects which model to use
    └────┬────┘
         │
    ┌────┴────┐
    │         │
    ▼         ▼
┌────────┐  ┌────────┐
│  OLD   │  │  NEW   │
│ MODEL  │  │ MODEL  │
│(Ginie) │  │(Ep.11) │
└────────┘  └────────┘
    │         │
    └────┬────┘
         │
         ▼
    EXECUTION
```

| Aspect | Old Model (Existing Ginie) | New Model (Epic 11) |
|--------|---------------------------|---------------------|
| Logic | Hardcoded | Parametric/Configurable |
| Settings | Fixed in code | JSON + UI adjustable |
| Strategies | Single approach | Multiple strategies |
| Scoring | Multiplicative | Additive |
| Learning | None | Calibration |
| UI | Current Ginie UI | Completely new UI |
| Code | `ginie_analyzer.go` | New separate files |

**Key Principles:**
1. **Complete Separation** - 100% separate codebase
2. **No Disturbance** - Old model remains untouched
3. **Plug & Play** - Switch between models after scanning
4. **Independent UI** - New UI for new model
5. **Parallel Existence** - Both models can run simultaneously

---

## References

- Analysis session: 876 blocked entries, 100% rejection rate
- Current scoring formula: `internal/autopilot/ginie_analyzer.go:2456`
- LLM prompts: `internal/ai/llm/prompts.go`
- Epic 10: Position Management & Optimization
- Party mode discussion: 2026-01-15
- Settings architecture: `default-settings.json`, `web/src/pages/ResetSettings.tsx`
- Settings comparison UI: `web/src/components/SettingsComparisonView.tsx`

---

## Story Summary

| Part | Stories | Description |
|------|---------|-------------|
| **A: Infrastructure** | 11.1-11.3 | Redis state, delta updates, WebSocket sync |
| **B: Market Regime** | 11.4-11.5 | Regime detection & transitions |
| **C: Strategy Framework** | 11.6-11.7 | Registry & selector |
| **D: Strategies** | 11.8-11.11 | Trend, Mean Reversion, Breakout, Range |
| **E: Indicators** | 11.12-11.14 | Segments, calculation, performance |
| **F: Scoring** | 11.15-11.18 | Additive score, LLM context, calibration, blocking |
| **G: Execution** | 11.19-11.22 | Decision output, actors, UI, dashboard |
| **H: Settings** | 11.23-11.27 | Settings structure, UI, calibration storage & display |

**Total Stories:** 27
