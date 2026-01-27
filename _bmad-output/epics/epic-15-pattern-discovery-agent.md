# Epic 15: Research Infrastructure - Data, Features & Backtesting

## Epic Overview

**Epic ID:** EPIC-15
**Status:** Ready for Implementation
**Created:** 2026-01-27
**Last Updated:** 2026-01-27
**Priority:** High

---

## Vision

Build **research infrastructure** that enables fast pattern discovery and strategy validation:

1. **Data Cache** - Store historical candle data locally for fast access
2. **Feature Calculator** - Pre-compute 70+ technical indicators
3. **Backtest Engine** - Fast API for testing trading rules
4. **Walk-Forward Validator** - Proper out-of-sample testing
5. **Data Availability UI** - Show what data is available for research

**Goal:** Enable Claude Code (or any researcher) to test 100+ hypotheses per hour instead of 5-10.

---

## Problem Statement

### Current Research Workflow (Slow)

```
1. Write 50+ line script from scratch
2. Fetch data from Binance API (slow, rate limited)
3. Calculate indicators manually
4. Run backtest
5. Parse results
6. Repeat for next hypothesis

Time per hypothesis: 5-10 minutes
Hypotheses per hour: ~10
```

### Target Research Workflow (Fast)

```
1. Call backtest API endpoint
2. Get instant results (data cached, features pre-calculated)
3. Repeat for next hypothesis

Time per hypothesis: 10-50 seconds
Hypotheses per hour: 100+
```

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      RESEARCH INFRASTRUCTURE                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│                         BINANCE API                                         │
│                              │                                              │
│                              ↓ (fetch once)                                 │
│  ┌───────────────────────────────────────────────────────────────────────┐ │
│  │                     DATA CACHE (PostgreSQL)                            │ │
│  │  • Historical candles (OHLCV + taker buy volume + trade count)        │ │
│  │  • Multiple coins, multiple timeframes                                 │ │
│  │  • 1-5 years of history per coin                                      │ │
│  └───────────────────────────────────────────────────────────────────────┘ │
│                              │                                              │
│                              ↓ (calculate once)                             │
│  ┌───────────────────────────────────────────────────────────────────────┐ │
│  │                    FEATURE CACHE (Redis)                               │ │
│  │  • 70+ pre-calculated features per candle                             │ │
│  │  • RSI, ATR, Bollinger, MACD, Volume ratios, etc.                     │ │
│  │  • Updated when new candles arrive                                    │ │
│  └───────────────────────────────────────────────────────────────────────┘ │
│                              │                                              │
│                              ↓ (use instantly)                              │
│  ┌───────────────────────────────────────────────────────────────────────┐ │
│  │                     BACKTEST ENGINE (Go)                               │ │
│  │  • Simple backtest (train on all data)                                │ │
│  │  • Walk-forward backtest (proper validation)                          │ │
│  │  • Multi-coin testing                                                 │ │
│  │  • Statistical validation (p-value, significance)                     │ │
│  └───────────────────────────────────────────────────────────────────────┘ │
│                              │                                              │
│                              ↓                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐ │
│  │                      API ENDPOINTS                                     │ │
│  │  POST /api/research/backtest                                          │ │
│  │  POST /api/research/walk-forward                                      │ │
│  │  GET  /api/research/features                                          │ │
│  │  GET  /api/research/data-availability                                 │ │
│  └───────────────────────────────────────────────────────────────────────┘ │
│                              │                                              │
│                              ↓                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐ │
│  │                    DATA AVAILABILITY UI                                │ │
│  │  • Show which coins have data                                         │ │
│  │  • Show which timeframes available                                    │ │
│  │  • Show date ranges                                                   │ │
│  │  • Show feature calculation status                                    │ │
│  └───────────────────────────────────────────────────────────────────────┘ │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Data Model

### Raw Candle Data (from Binance)

```go
type Candle struct {
    Symbol          string    `json:"symbol"`
    Timeframe       string    `json:"timeframe"`
    OpenTime        time.Time `json:"open_time"`
    Open            float64   `json:"open"`
    High            float64   `json:"high"`
    Low             float64   `json:"low"`
    Close           float64   `json:"close"`
    Volume          float64   `json:"volume"`           // Base asset volume
    QuoteVolume     float64   `json:"quote_volume"`     // Quote asset (USDT) volume
    TradeCount      int       `json:"trade_count"`      // Number of trades
    TakerBuyVolume  float64   `json:"taker_buy_volume"` // Aggressive buyer volume
    TakerBuyQuote   float64   `json:"taker_buy_quote"`  // Aggressive buyer USDT volume
}
```

### Calculated Features (70+ indicators)

```go
type CandleFeatures struct {
    // Original candle data
    Candle

    // Price Features (15)
    Return1c         float64 `json:"return_1c"`          // 1-candle return %
    Return3c         float64 `json:"return_3c"`          // 3-candle return %
    Return5c         float64 `json:"return_5c"`          // 5-candle return %
    Return10c        float64 `json:"return_10c"`         // 10-candle return %
    Return20c        float64 `json:"return_20c"`         // 20-candle return %
    BodyRatio        float64 `json:"body_ratio"`         // |close-open| / range
    UpperWickRatio   float64 `json:"upper_wick_ratio"`   // upper wick / range
    LowerWickRatio   float64 `json:"lower_wick_ratio"`   // lower wick / range
    PositionInRange  float64 `json:"position_in_range"`  // (close-low) / range
    GapPercent       float64 `json:"gap_percent"`        // gap from prev close
    RangePercent     float64 `json:"range_percent"`      // range / close
    ConsecutiveUp    int     `json:"consecutive_up"`     // bullish streak
    ConsecutiveDown  int     `json:"consecutive_down"`   // bearish streak
    HigherHigh       bool    `json:"higher_high"`        // new high?
    LowerLow         bool    `json:"lower_low"`          // new low?

    // Volume Features (12)
    VolumeRatio5     float64 `json:"volume_ratio_5"`     // vs 5-period avg
    VolumeRatio10    float64 `json:"volume_ratio_10"`    // vs 10-period avg
    VolumeRatio20    float64 `json:"volume_ratio_20"`    // vs 20-period avg
    VolumeTrend5     float64 `json:"volume_trend_5"`     // volume slope
    TakerBuyRatio    float64 `json:"taker_buy_ratio"`    // buy pressure
    TradeCountVal    int     `json:"trade_count_val"`    // number of trades
    AvgTradeSize     float64 `json:"avg_trade_size"`     // volume / trades
    OBV              float64 `json:"obv"`                // On Balance Volume
    OBVTrend         float64 `json:"obv_trend"`          // OBV slope
    VolumePriceTrend float64 `json:"volume_price_trend"` // VPT indicator
    AccumDist        float64 `json:"accum_dist"`         // A/D indicator
    MoneyFlow        float64 `json:"money_flow"`         // MFI base

    // Volatility Features (10)
    ATR7             float64 `json:"atr_7"`              // 7-period ATR
    ATR14            float64 `json:"atr_14"`             // 14-period ATR
    ATR21            float64 `json:"atr_21"`             // 21-period ATR
    ATRPercent       float64 `json:"atr_percent"`        // ATR / close
    BollingerUpper   float64 `json:"bollinger_upper"`    // upper band
    BollingerLower   float64 `json:"bollinger_lower"`    // lower band
    BollingerWidth   float64 `json:"bollinger_width"`    // band width
    BollingerPos     float64 `json:"bollinger_position"` // position in bands
    RangeExpansion   float64 `json:"range_expansion"`    // range vs avg
    VolatilityRatio  float64 `json:"volatility_ratio"`   // current vs historical

    // Momentum Features (15)
    RSI7             float64 `json:"rsi_7"`              // 7-period RSI
    RSI14            float64 `json:"rsi_14"`             // 14-period RSI
    RSI21            float64 `json:"rsi_21"`             // 21-period RSI
    StochK           float64 `json:"stoch_k"`            // Stochastic %K
    StochD           float64 `json:"stoch_d"`            // Stochastic %D
    MACDLine         float64 `json:"macd_line"`          // MACD line
    MACDSignal       float64 `json:"macd_signal"`        // Signal line
    MACDHistogram    float64 `json:"macd_histogram"`     // Histogram
    ROC5             float64 `json:"roc_5"`              // Rate of change
    ROC10            float64 `json:"roc_10"`
    ROC20            float64 `json:"roc_20"`
    Momentum5        float64 `json:"momentum_5"`         // Price momentum
    Momentum10       float64 `json:"momentum_10"`
    WilliamsR        float64 `json:"williams_r"`         // Williams %R

    // Trend Features (12)
    SMA10            float64 `json:"sma_10"`             // Moving averages
    SMA20            float64 `json:"sma_20"`
    SMA50            float64 `json:"sma_50"`
    EMA9             float64 `json:"ema_9"`
    EMA21            float64 `json:"ema_21"`
    PriceVsSMA20     float64 `json:"price_vs_sma_20"`    // % from SMA
    SMACross         int     `json:"sma_cross"`          // 1 = bullish, -1 = bearish
    ADX              float64 `json:"adx"`                // Trend strength
    PlusDI           float64 `json:"plus_di"`            // +DI
    MinusDI          float64 `json:"minus_di"`           // -DI
    TrendStrength    float64 `json:"trend_strength"`     // ADX value
    TrendDirection   int     `json:"trend_direction"`    // 1 = up, -1 = down

    // Time Features (6)
    HourOfDay        int     `json:"hour_of_day"`        // 0-23 UTC
    DayOfWeek        int     `json:"day_of_week"`        // 0=Sun, 6=Sat
    IsWeekend        bool    `json:"is_weekend"`
    Session          string  `json:"session"`            // "asia","europe","us"
    DayOfMonth       int     `json:"day_of_month"`       // 1-31
    IsMonthEnd       bool    `json:"is_month_end"`       // last 3 days
}
```

---

## API Endpoints

### 1. Data Availability

```
GET /api/research/data-availability

Response:
{
  "coins": [
    {
      "symbol": "BTCUSDT",
      "timeframes": {
        "5m":  { "from": "2023-01-01", "to": "2026-01-27", "candles": 315360 },
        "15m": { "from": "2023-01-01", "to": "2026-01-27", "candles": 105120 },
        "1h":  { "from": "2022-01-01", "to": "2026-01-27", "candles": 35064 },
        "4h":  { "from": "2021-01-01", "to": "2026-01-27", "candles": 10950 },
        "1d":  { "from": "2019-01-01", "to": "2026-01-27", "candles": 2583 }
      },
      "features_calculated": true,
      "last_updated": "2026-01-27T10:00:00Z"
    },
    {
      "symbol": "MATICUSDT",
      "timeframes": { ... },
      "features_calculated": true,
      "last_updated": "2026-01-27T10:00:00Z"
    },
    ...
  ],
  "total_coins": 50,
  "total_candles": 12500000,
  "storage_size_mb": 1250
}
```

### 2. Download Data

```
POST /api/research/download-data

Request:
{
  "symbol": "MATICUSDT",
  "timeframes": ["5m", "15m", "1h", "4h", "1d"],
  "from": "2023-01-01",
  "to": "2026-01-27"
}

Response:
{
  "job_id": "dl-12345",
  "status": "started",
  "estimated_candles": 420480
}

GET /api/research/download-status/dl-12345

Response:
{
  "job_id": "dl-12345",
  "status": "in_progress",
  "progress": 65,
  "candles_downloaded": 273312,
  "candles_total": 420480
}
```

### 3. Simple Backtest

```
POST /api/research/backtest

Request:
{
  "coins": ["MATICUSDT", "TIAUSDT", "ATOMUSDT"],
  "timeframe": "15m",
  "from": "2024-01-01",
  "to": "2025-12-31",

  "entry_conditions": [
    { "feature": "return_5c", "operator": "<", "value": -2.0 },
    { "feature": "position_in_range", "operator": "<", "value": 0.3 }
  ],
  "entry_logic": "AND",

  "sl_atr_multiplier": 0.5,
  "tp_atr_multiplier": 2.0,
  "atr_period": 14,
  "max_hold_candles": 50,

  "include_fees": true,
  "fee_percent": 0.04
}

Response:
{
  "summary": {
    "total_trades": 543,
    "wins": 123,
    "losses": 387,
    "timeouts": 33,
    "win_rate": 22.65,
    "expected_atr": 0.066,
    "expected_percent": 0.18,
    "sharpe_ratio": 1.23,
    "profit_factor": 1.27,
    "max_drawdown_percent": -8.5,
    "avg_hold_candles": 12.3
  },
  "by_coin": [
    { "coin": "MATICUSDT", "trades": 187, "win_rate": 23.53, "expected_atr": 0.079 },
    { "coin": "TIAUSDT", "trades": 156, "win_rate": 21.79, "expected_atr": 0.047 },
    { "coin": "ATOMUSDT", "trades": 200, "win_rate": 22.50, "expected_atr": 0.065 }
  ],
  "validation": {
    "p_value": 0.003,
    "significant": true,
    "sample_size_adequate": true
  },
  "execution_time_ms": 45
}
```

### 4. Walk-Forward Backtest

```
POST /api/research/walk-forward

Request:
{
  "coins": ["MATICUSDT", "TIAUSDT", "ATOMUSDT"],
  "timeframe": "15m",
  "from": "2023-01-01",
  "to": "2025-12-31",

  "entry_conditions": [
    { "feature": "return_5c", "operator": "<", "value": -2.0 },
    { "feature": "position_in_range", "operator": "<", "value": 0.3 }
  ],
  "entry_logic": "AND",

  "sl_atr_multiplier": 0.5,
  "tp_atr_multiplier": 2.0,

  "walk_forward_config": {
    "train_months": 6,
    "test_months": 3,
    "step_months": 3,
    "holdout_percent": 20
  }
}

Response:
{
  "periods": [
    {
      "train": { "from": "2023-01-01", "to": "2023-06-30" },
      "test": { "from": "2023-07-01", "to": "2023-09-30" },
      "test_results": {
        "trades": 45,
        "win_rate": 22.2,
        "expected_atr": 0.055
      }
    },
    {
      "train": { "from": "2023-01-01", "to": "2023-09-30" },
      "test": { "from": "2023-10-01", "to": "2023-12-31" },
      "test_results": {
        "trades": 52,
        "win_rate": 23.1,
        "expected_atr": 0.077
      }
    },
    ... more periods ...
  ],

  "aggregate_test_results": {
    "total_trades": 312,
    "win_rate": 22.4,
    "expected_atr": 0.062,
    "consistency": 0.85,
    "worst_period_win_rate": 19.2,
    "best_period_win_rate": 25.3
  },

  "holdout_results": {
    "period": { "from": "2025-07-01", "to": "2025-12-31" },
    "trades": 89,
    "win_rate": 21.3,
    "expected_atr": 0.043,
    "status": "PASS"
  },

  "final_verdict": {
    "profitable": true,
    "consistent": true,
    "holdout_passed": true,
    "recommendation": "VALIDATED - Ready for live trading"
  }
}
```

### 5. Get Available Features

```
GET /api/research/features

Response:
{
  "categories": [
    {
      "name": "Price",
      "features": [
        { "name": "return_5c", "description": "5-candle return %", "type": "float" },
        { "name": "position_in_range", "description": "(close-low)/(high-low)", "type": "float" },
        ...
      ]
    },
    {
      "name": "Volume",
      "features": [
        { "name": "volume_ratio_20", "description": "Volume vs 20-period avg", "type": "float" },
        { "name": "taker_buy_ratio", "description": "Buy pressure ratio", "type": "float" },
        ...
      ]
    },
    ...
  ],
  "total_features": 70
}
```

---

## UI Design: Research Data Page

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  RESEARCH DATA                                          [Download Data]     │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  DATA SUMMARY                                                               │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Total Coins: 50          Total Candles: 12.5M                      │   │
│  │  Storage: 1.25 GB         Last Updated: 2026-01-27 10:00 UTC        │   │
│  │  Features: 70 calculated  Status: ● Up to date                      │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  DATA AVAILABILITY BY COIN                                                  │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ Coin       │ 5m      │ 15m     │ 1h      │ 4h      │ 1d      │ Feat │   │
│  ├────────────┼─────────┼─────────┼─────────┼─────────┼─────────┼──────┤   │
│  │ BTCUSDT    │ 3y ✓    │ 3y ✓    │ 4y ✓    │ 5y ✓    │ 7y ✓    │ ✓    │   │
│  │ ETHUSDT    │ 3y ✓    │ 3y ✓    │ 4y ✓    │ 5y ✓    │ 6y ✓    │ ✓    │   │
│  │ MATICUSDT  │ 2y ✓    │ 2y ✓    │ 3y ✓    │ 3y ✓    │ 3y ✓    │ ✓    │   │
│  │ TIAUSDT    │ 6m ⚠    │ 1y ⚠    │ 1y ⚠    │ 1y ⚠    │ 1y ⚠    │ ✓    │   │
│  │ ATOMUSDT   │ 2y ✓    │ 2y ✓    │ 3y ✓    │ 4y ✓    │ 5y ✓    │ ✓    │   │
│  │ SOLUSDT    │ 2y ✓    │ 2y ✓    │ 3y ✓    │ 4y ✓    │ 4y ✓    │ ✓    │   │
│  │ ...        │         │         │         │         │         │      │   │
│  └────────────┴─────────┴─────────┴─────────┴─────────┴─────────┴──────┘   │
│                                                                             │
│  ✓ = Full coverage (2+ years)  ⚠ = Limited coverage  ✗ = No data          │
│                                                                             │
│  AVAILABLE FEATURES                                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ ▼ Price Features (15)                                               │   │
│  │   return_1c, return_3c, return_5c, return_10c, return_20c          │   │
│  │   body_ratio, upper_wick_ratio, lower_wick_ratio, position_in_range│   │
│  │   gap_percent, range_percent, consecutive_up, consecutive_down      │   │
│  │   higher_high, lower_low                                            │   │
│  │                                                                      │   │
│  │ ▶ Volume Features (12)                                              │   │
│  │ ▶ Volatility Features (10)                                          │   │
│  │ ▶ Momentum Features (15)                                            │   │
│  │ ▶ Trend Features (12)                                               │   │
│  │ ▶ Time Features (6)                                                 │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  [+ Add Coin]  [Refresh All]  [Calculate Features]                         │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Stories

### PART A: Data Infrastructure

#### Story 15.1: Database Schema for Historical Candles
**Priority:** P0
**Estimate:** Small

Create PostgreSQL schema for storing historical candle data.

**Acceptance Criteria:**
- Table: `research_candles` with all Binance kline fields
- Indexes on (symbol, timeframe, open_time)
- Support for efficient range queries
- Partitioning by symbol (optional, for performance)

**Schema:**
```sql
CREATE TABLE research_candles (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(5) NOT NULL,
    open_time TIMESTAMP NOT NULL,
    open DECIMAL(20,8) NOT NULL,
    high DECIMAL(20,8) NOT NULL,
    low DECIMAL(20,8) NOT NULL,
    close DECIMAL(20,8) NOT NULL,
    volume DECIMAL(30,8) NOT NULL,
    quote_volume DECIMAL(30,8) NOT NULL,
    trade_count INTEGER NOT NULL,
    taker_buy_volume DECIMAL(30,8) NOT NULL,
    taker_buy_quote DECIMAL(30,8) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(symbol, timeframe, open_time)
);

CREATE INDEX idx_candles_symbol_tf_time
ON research_candles(symbol, timeframe, open_time DESC);
```

---

#### Story 15.2: Data Download Service
**Priority:** P0
**Estimate:** Medium

Create service to download historical data from Binance API.

**Acceptance Criteria:**
- Download candles for specified symbol/timeframe/date range
- Handle Binance API rate limits (1200 req/min)
- Store in PostgreSQL
- Track download progress
- Resume interrupted downloads
- Background job support

**Files:**
- `internal/research/data_downloader.go`

---

#### Story 15.3: Data Download API Endpoints
**Priority:** P0
**Estimate:** Small

Create API endpoints for data download management.

**Acceptance Criteria:**
- POST /api/research/download-data - Start download job
- GET /api/research/download-status/:job_id - Check progress
- GET /api/research/data-availability - List available data

**Files:**
- `internal/api/handlers_research_data.go`

---

### PART B: Feature Calculation

#### Story 15.4: Feature Calculator Engine
**Priority:** P0
**Estimate:** Large

Create engine to calculate 70+ technical indicators from raw candles.

**Acceptance Criteria:**
- Calculate all features defined in data model
- Efficient batch calculation
- Handle edge cases (insufficient data for lookback)
- Store calculated features in Redis cache

**Feature Categories:**
- Price features (15)
- Volume features (12)
- Volatility features (10)
- Momentum features (15)
- Trend features (12)
- Time features (6)

**Files:**
- `internal/research/feature_calculator.go`
- `internal/research/indicators/` (individual indicator implementations)

---

#### Story 15.5: Feature Cache Management
**Priority:** P0
**Estimate:** Medium

Manage feature storage in Redis with efficient retrieval.

**Acceptance Criteria:**
- Store features as Redis hash per candle
- Batch retrieval for backtest queries
- TTL management for old data
- Feature versioning (for when calculations change)

**Files:**
- `internal/research/feature_cache.go`

---

#### Story 15.6: Feature Calculation Background Job
**Priority:** P1
**Estimate:** Small

Background job to keep features up-to-date.

**Acceptance Criteria:**
- Trigger feature calculation when new candles downloaded
- Periodic recalculation for active research coins
- Progress tracking

---

### PART C: Backtest Engine

#### Story 15.7: Simple Backtest Engine
**Priority:** P0
**Estimate:** Large

Core backtesting engine for testing trading rules.

**Acceptance Criteria:**
- Parse entry conditions (feature, operator, value)
- Support AND/OR logic for multiple conditions
- Simulate trades with SL/TP based on ATR
- Calculate statistics (win rate, expected value, etc.)
- Support multiple coins in single backtest
- Include trading fees

**Files:**
- `internal/research/backtest_engine.go`
- `internal/research/backtest_types.go`

---

#### Story 15.8: Walk-Forward Backtest Engine
**Priority:** P0
**Estimate:** Large

Proper out-of-sample validation with walk-forward methodology.

**Acceptance Criteria:**
- Configure train/test period lengths
- Rolling window testing
- Hold-out period for final validation
- Aggregate results across all test periods
- Calculate consistency metrics

**Files:**
- `internal/research/walk_forward.go`

---

#### Story 15.9: Backtest API Endpoints
**Priority:** P0
**Estimate:** Medium

Create API endpoints for backtesting.

**Acceptance Criteria:**
- POST /api/research/backtest - Simple backtest
- POST /api/research/walk-forward - Walk-forward validation
- GET /api/research/features - List available features

**Files:**
- `internal/api/handlers_research_backtest.go`

---

### PART D: UI

#### Story 15.10: Research Data Availability Page
**Priority:** P0
**Estimate:** Medium

Create UI page showing available research data.

**Acceptance Criteria:**
- Display data coverage matrix (coin × timeframe)
- Show date ranges and candle counts
- Show feature calculation status
- Download data button/form
- Refresh and calculate features buttons
- Expandable feature list with descriptions

**Files:**
- `web/src/pages/ResearchData.tsx`
- `web/src/components/Research/DataAvailabilityTable.tsx`
- `web/src/components/Research/FeatureList.tsx`
- `web/src/components/Research/DownloadDataModal.tsx`

---

#### Story 15.11: Navigation Integration
**Priority:** P0
**Estimate:** Small

Add Research Data page to navigation.

**Acceptance Criteria:**
- Add sidebar menu item
- Route configuration
- Permission controls (admin only?)

---

### PART E: Integration

#### Story 15.12: Main.go Integration
**Priority:** P0
**Estimate:** Small

Wire research services into main application.

**Acceptance Criteria:**
- Initialize research services
- Connect to existing database/Redis
- Register API routes

---

---

## Dependencies

```
Story Dependencies:
  15.1 (Schema) → 15.2 (Downloader) → 15.3 (API)
  15.1 (Schema) → 15.4 (Calculator) → 15.5 (Cache) → 15.6 (Job)
  15.4 (Calculator) + 15.5 (Cache) → 15.7 (Backtest)
  15.7 (Backtest) → 15.8 (Walk-Forward) → 15.9 (API)
  15.3 + 15.9 → 15.10 (UI) → 15.11 (Navigation)
  All → 15.12 (Integration)
```

---

## Package Structure

```
internal/
├── research/
│   ├── data_downloader.go      # Binance data fetching
│   ├── feature_calculator.go   # Main feature engine
│   ├── feature_cache.go        # Redis feature storage
│   ├── backtest_engine.go      # Simple backtesting
│   ├── backtest_types.go       # Types and conditions
│   ├── walk_forward.go         # Walk-forward validation
│   ├── types.go                # Shared types
│   └── indicators/             # Individual indicators
│       ├── rsi.go
│       ├── atr.go
│       ├── bollinger.go
│       ├── macd.go
│       ├── volume.go
│       └── ...
│
├── api/
│   ├── handlers_research_data.go      # Data endpoints
│   └── handlers_research_backtest.go  # Backtest endpoints
│
web/src/
├── pages/
│   └── ResearchData.tsx
└── components/
    └── Research/
        ├── DataAvailabilityTable.tsx
        ├── FeatureList.tsx
        └── DownloadDataModal.tsx
```

---

## Success Metrics

1. **Speed:** Backtest API responds in <100ms for 1-year data
2. **Coverage:** 50+ coins with 2+ years of data each
3. **Features:** 70+ features calculated per candle
4. **Validation:** Walk-forward testing prevents overfitting
5. **Usability:** User can see available data at a glance

---

## What This Enables

Once built, Claude Code (or any researcher) can:

```bash
# Test a hypothesis in seconds:
curl -X POST localhost:8094/api/research/backtest -d '{
  "coins": ["MATICUSDT", "TIAUSDT"],
  "entry_conditions": [{"feature": "rsi_14", "operator": "<", "value": 30}],
  "sl_atr_multiplier": 0.5,
  "tp_atr_multiplier": 2.0
}'

# Result in 50ms instead of 5 minutes!
```

This enables 100x faster research iteration.

---

*Epic created: 2026-01-27*
*Replaces previous Epic 15 (over-engineered agent approach)*
