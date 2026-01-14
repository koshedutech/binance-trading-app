# Story 10.1: Position Management & Efficiency Exit System

## Story Overview

**Epic**: Epic 10 - Position Management & Optimization
**Story ID:** POS-10.1
**Story Type**: Feature Enhancement
**Priority**: P1 (High)
**Complexity**: High
**Status**: Ready for Implementation
**Created:** 2026-01-14
**Last Updated:** 2026-01-14

---

## Executive Summary

This story implements a comprehensive position management system with:
1. **Simplified Efficiency Tracking** - Track profit efficiency without complex rate calculations
2. **Trend-Based Exit Priority** - Trend reversal triggers immediate exit
3. **Dynamic SL/TP Management** - Both updated on Binance for profit protection
4. **Redis-First Architecture** - All real-time data in Redis for millisecond decisions
5. **Integration with Position Optimization** - Works with or without staged TPs

---

## Problem Statement

### Current Issues

1. **Positions held too long** - No efficiency tracking leads to diminishing returns
2. **Trailing stop is software-only** - Binance SL order not updated, profits not protected
3. **No trend-based exit** - System waits for SL instead of exiting on trend reversal
4. **Complex calculations** - Rate-per-unit formulas are overcomplicated
5. **Database latency** - Decision-making slowed by DB queries during active trades

### Data Analysis Results

| Hold Duration | Avg ROI | Trades | Observation |
|---------------|---------|--------|-------------|
| < 15 min | **1.72%** | 45% | Highest efficiency |
| 15-30 min | 0.45% | 25% | Declining |
| 30-60 min | 0.15% | 18% | Poor |
| > 60 min | **0.02%** | 12% | Very poor |

**Key Insight:** Fast exits with high efficiency are better than holding for small additional gains.

---

## Solution: Simplified Efficiency Model

### The Core Formula

```
EFFICIENCY = currentProfit / peakProfit

THRESHOLD = average(exit_efficiency) from last 4-8 hours of closed trades

EXIT when efficiency < threshold
```

**That's it!** No rate-per-unit, no time units, no complex formulas.

### Why This Works

- **Peak Profit**: The highest profit % achieved since entry (only goes up)
- **Current Profit**: The current profit %
- **Efficiency**: How much of your best moment are you still capturing?
- **Threshold**: What's the historical average efficiency at exit?

### Example

```
Entry: $100.00

Price Movement:
  $100.50 → Profit 0.50% → Peak 0.50% → Efficiency 100%
  $100.80 → Profit 0.80% → Peak 0.80% → Efficiency 100% (new peak!)
  $100.60 → Profit 0.60% → Peak 0.80% → Efficiency 75%
  $100.40 → Profit 0.40% → Peak 0.80% → Efficiency 50%
  $100.30 → Profit 0.30% → Peak 0.80% → Efficiency 37.5%

If historical threshold = 40%:
  At 37.5% efficiency → EXIT (below threshold)
  Captured: 0.30% profit instead of waiting for potential reversal
```

---

## Part 1: Position Lifecycle Stages

### Stage Definitions

```
┌─────────────────────────────────────────────────────────────────┐
│ STAGE 1: RISK_ZONE                                              │
│ ═══════════════════                                             │
│ • Position is below breakeven                                   │
│ • Capital is at risk                                            │
│ • Initial SL/TP from mode config active                        │
│ • Early Warning System monitors trend                          │
│                                                                 │
│ Exit Conditions:                                                │
│   - Trend reversal confirmed → EXIT IMMEDIATELY                │
│   - Fixed SL hit → EXIT (worst case)                           │
│   - Price reaches breakeven → ADVANCE TO STAGE 2               │
├─────────────────────────────────────────────────────────────────┤
│ STAGE 2: BREAKEVEN_ACHIEVED                                     │
│ ═══════════════════════════                                     │
│ • Price has reached entry + fees + buffer                      │
│ • Move SL to breakeven price                                   │
│ • Position is now "FREE" (no capital risk)                     │
│ • Initialize efficiency tracking                               │
│                                                                 │
│ Next Step:                                                      │
│   - If Position Optimization ON → ADVANCE TO TP1_PENDING       │
│   - If Position Optimization OFF → ADVANCE TO EFFICIENCY       │
├─────────────────────────────────────────────────────────────────┤
│ STAGE 3A: TP1_PENDING (If Position Optimization enabled)       │
│ ════════════════════════════════════════════════════           │
│ • Waiting for TP1 to hit                                       │
│ • Efficiency tracking NOT active yet                           │
│                                                                 │
│ Exit Conditions:                                                │
│   - Trend reversal confirmed → EXIT IMMEDIATELY                │
│   - TP1 hit → Sell configured %, ADVANCE TO EFFICIENCY         │
├─────────────────────────────────────────────────────────────────┤
│ STAGE 3B: EFFICIENCY_TRACKING (Main operating stage)           │
│ ════════════════════════════════════════════════════           │
│ • Efficiency tracking is ACTIVE                                │
│ • Dynamic SL/TP updates on Binance                             │
│ • Trend monitoring continuous                                  │
│                                                                 │
│ Exit Conditions (Priority Order):                              │
│   1. Trend reversal confirmed → EXIT IMMEDIATELY               │
│   2. Efficiency < threshold → EXIT                             │
│   3. Trailing SL hit → EXIT (Binance order)                    │
│   4. Dynamic TP hit → EXIT (best case)                         │
└─────────────────────────────────────────────────────────────────┘
```

### Breakeven Calculation

```go
// Breakeven = Entry + Fees + Small Buffer
func calculateBreakevenPrice(entryPrice float64, side string, feePercent float64) float64 {
    // Total fees: entry (0.05%) + exit (0.05%) = 0.10%
    // Buffer: 0.05% (to cover slippage and ensure small profit)
    totalBuffer := feePercent + 0.05  // e.g., 0.10 + 0.05 = 0.15%

    if side == "LONG" {
        return entryPrice * (1 + totalBuffer/100)  // e.g., $100 * 1.0015 = $100.15
    }
    return entryPrice * (1 - totalBuffer/100)  // For SHORT
}
```

---

## Part 2: Exit Priority - TREND IS KING

### The Golden Rule

```
┌─────────────────────────────────────────────────────────────────┐
│                        TREND IS KING                            │
│                                                                 │
│   Trend UP      → HOLD (no matter what)                        │
│   Trend DOWN    → EXIT IMMEDIATELY (don't wait for SL)         │
│   Trend SIDEWAYS → Check efficiency, tighten SL                │
└─────────────────────────────────────────────────────────────────┘
```

### Exit Priority Order

| Priority | Condition | Action | Speed |
|----------|-----------|--------|-------|
| **1** | Trend Reversal Confirmed | EXIT NOW | Immediate (market order) |
| **2** | Efficiency < Threshold | EXIT | Normal (limit with offset) |
| **3** | Trailing SL Hit | EXIT | Binance handles |
| **4** | Dynamic TP Hit | EXIT | Binance handles |

### Trend Reversal Detection

```go
type TrendAnalysis struct {
    Direction        string  // "UP", "DOWN", "SIDEWAYS"
    Strength         float64 // 0.0 - 1.0
    Confidence       float64 // 0.0 - 1.0
    ReversalDetected bool
    RecoveryLikely   bool

    // Indicators
    ADX              float64
    RSI              float64
    ATRPercent       float64
    MACDSignal       string  // "bullish", "bearish", "neutral"
}

// Trend reversal confirmation requirements
type TrendReversalConfirmation struct {
    MinConsecutiveSignals int     // At least 2 consecutive bearish signals
    MinConfidence         float64 // At least 0.75 (75%)
    RequireVolumeConfirm  bool    // Volume should support reversal
}
```

### Trend-Based Exit Execution

```go
func (ga *GinieAutopilot) executeTrendReversalExit(pos *PositionRuntimeState) error {
    // STEP 1: Try to update SL to current price (lock whatever profit)
    newSL := pos.CurrentPrice * 0.999  // 0.1% below current
    err := ga.updateBinanceStopLoss(pos, newSL)

    if err != nil {
        // STEP 2: If SL update fails, wait briefly and retry
        time.Sleep(2 * time.Second)
        err = ga.updateBinanceStopLoss(pos, pos.CurrentPrice * 0.998)
    }

    if err != nil {
        // STEP 3: If still failing, place MARKET ORDER immediately
        return ga.closePositionMarket(pos, "TREND_REVERSAL_EMERGENCY")
    }

    return nil
}
```

---

## Part 3: Dynamic SL/TP Management

### Why Both SL and TP on Binance

```
If our system crashes, Binance protects us:
  - SL on Binance → Limits loss if price drops
  - TP on Binance → Captures profit if price spikes

Both must be updated dynamically as price moves.
```

### Dynamic SL Calculation

```go
func (ga *GinieAutopilot) calculateDynamicSL(pos *PositionRuntimeState) float64 {
    // Get market data from Redis cache
    atr := ga.redis.GetIndicator(pos.Symbol, "atr_pct")  // e.g., 0.8%
    adx := ga.redis.GetIndicator(pos.Symbol, "adx")       // e.g., 25

    // BASE: 1.5x ATR from highest price
    baseTrailing := atr * 1.5

    // ADJUST for trend strength
    if adx > 30 {
        baseTrailing *= 0.8   // Strong trend → tighter SL (20% tighter)
    } else if adx < 20 {
        baseTrailing *= 1.3   // Weak trend → wider SL (30% wider)
    }

    // ADJUST for profit level
    if pos.CurrentProfit > 1.0 {
        baseTrailing *= 0.9   // Good profit → protect more (10% tighter)
    }

    // ADJUST for efficiency decline
    if pos.Efficiency < 0.50 {
        baseTrailing *= 0.85  // Efficiency dropping → trail tighter (15% tighter)
    }

    // CLAMP to reasonable bounds
    baseTrailing = clamp(baseTrailing, 0.3, 3.0)

    // Calculate actual SL price
    if pos.Side == "LONG" {
        return pos.PeakPrice * (1 - baseTrailing/100)
    }
    return pos.PeakPrice * (1 + baseTrailing/100)  // For SHORT
}
```

### Dynamic TP Calculation

```go
func (ga *GinieAutopilot) calculateDynamicTP(pos *PositionRuntimeState) float64 {
    atr := ga.redis.GetIndicator(pos.Symbol, "atr_pct")
    adx := ga.redis.GetIndicator(pos.Symbol, "adx")

    // BASE: 3x ATR above highest price
    baseTP := atr * 3.0

    // Strong trend = aim higher
    if adx > 35 {
        baseTP *= 1.5
    }

    // Weak trend = more conservative
    if adx < 20 {
        baseTP *= 0.7
    }

    // CLAMP to reasonable bounds
    baseTP = clamp(baseTP, 1.5, 8.0)

    // Calculate actual TP price (trails upward only)
    if pos.Side == "LONG" {
        return pos.PeakPrice * (1 + baseTP/100)
    }
    return pos.PeakPrice * (1 - baseTP/100)  // For SHORT
}
```

### Update Logic

```go
func (ga *GinieAutopilot) updateDynamicLevels(pos *PositionRuntimeState) error {
    // Update SL (only if improvement)
    newSL := ga.calculateDynamicSL(pos)
    if pos.Side == "LONG" && newSL > pos.SLPrice {
        ga.updateBinanceStopLoss(pos, newSL)
        pos.SLPrice = newSL
    }

    // Update TP (only if improvement)
    newTP := ga.calculateDynamicTP(pos)
    if pos.Side == "LONG" && newTP > pos.TPPrice {
        ga.updateBinanceTakeProfit(pos, newTP)
        pos.TPPrice = newTP
    }

    return nil
}
```

---

## Part 4: Historical Baseline (Simplified)

### What We Store When Trade Closes

```go
type TradeEfficiencyRecord struct {
    TradeID        int64   `db:"trade_id"`
    UserID         string  `db:"user_id"`
    Symbol         string  `db:"symbol"`
    Mode           string  `db:"mode"`

    // Efficiency at exit (SIMPLE!)
    PeakProfit     float64 `db:"peak_profit"`       // Highest profit % achieved
    ExitProfit     float64 `db:"exit_profit"`       // Profit % at exit
    ExitEfficiency float64 `db:"exit_efficiency"`   // exit_profit / peak_profit

    // Metadata
    ExitReason     string  `db:"exit_reason"`       // "efficiency", "trend", "sl", "tp"
    Category       int     `db:"category"`          // 1=loss, 2=breakeven, 3=success
    CreatedAt      int64   `db:"created_at"`
}
```

### Historical Baseline Calculation

```go
// Runs every 1 hour via background job
func (ga *GinieAutopilot) refreshHistoricalBaseline(userID, mode string) error {
    windowHours := getWindowHours(mode)  // 4-8 hours based on mode

    // Query last N hours of closed trades
    records, err := ga.db.Query(`
        SELECT exit_efficiency, category
        FROM trade_efficiency_metrics
        WHERE user_id = $1 AND mode = $2
        AND created_at >= NOW() - INTERVAL '$3 hours'
    `, userID, mode, windowHours)

    if err != nil || len(records) == 0 {
        // Use default threshold if no history
        return ga.redis.SaveBaseline(userID, mode, HistoricalBaseline{
            AvgExitEfficiency: 0.50,  // Default 50%
            TradeCount: 0,
        })
    }

    // Simple average of all exit efficiencies!
    totalEfficiency := 0.0
    for _, record := range records {
        totalEfficiency += record.ExitEfficiency
    }
    avgEfficiency := totalEfficiency / float64(len(records))

    baseline := HistoricalBaseline{
        Mode:              mode,
        AvgExitEfficiency: avgEfficiency,
        TradeCount:        len(records),
        WindowHours:       windowHours,
        LastUpdated:       time.Now().Unix(),
    }

    return ga.redis.SaveBaseline(userID, mode, baseline)
}

func getWindowHours(mode string) int {
    switch mode {
    case "ultra_fast": return 4
    case "scalp":      return 6
    case "swing":      return 8
    case "position":   return 12
    default:           return 6
    }
}
```

### Redis Storage

```go
// Key: baseline:{user_id}:{mode}
type HistoricalBaseline struct {
    Mode              string  `json:"mode"`
    AvgExitEfficiency float64 `json:"avg_eff"`     // This is the threshold!
    TradeCount        int     `json:"count"`
    WindowHours       int     `json:"window"`
    LastUpdated       int64   `json:"updated_ts"`
}
```

---

## Part 5: Redis-First Architecture

### Design Principle

```
┌─────────────────────────────────────────────────────────────────┐
│                    DATA ARCHITECTURE                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  REDIS (All real-time data):                                   │
│  ════════════════════════════                                   │
│  • Position state (updated every tick)                         │
│  • Efficiency tracking                                          │
│  • Peak profit, current profit                                 │
│  • SL/TP levels                                                 │
│  • Trend analysis results                                       │
│  • Market data cache (candles, indicators)                     │
│  • Historical baseline (cached)                                │
│                                                                 │
│  POSTGRESQL (Permanent records only):                          │
│  ════════════════════════════════════                          │
│  • Closed trade records                                         │
│  • Efficiency metrics (for baseline calculation)               │
│  • User configurations                                          │
│                                                                 │
│  RULE: NO PostgreSQL queries during active position!           │
│        All decision data must be in Redis.                     │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Redis Key Structure

```
Position State:
  pos:{user_id}:{symbol}:state     → PositionRuntimeState (JSON)

Historical Baseline:
  baseline:{user_id}:{mode}        → HistoricalBaseline (JSON)

Market Data Cache:
  kline:{symbol}:{interval}        → Sorted Set of candles
  kline:{symbol}:{interval}:current → Current forming candle
  ind:{symbol}:rsi                 → Latest RSI value
  ind:{symbol}:adx                 → Latest ADX value
  ind:{symbol}:atr                 → Latest ATR value
  price:{symbol}                   → Current price + timestamp

Active Positions Index:
  active:{user_id}                 → Set of active symbols

Mode Config Cache:
  config:{user_id}:{mode}          → ModeConfigCache (JSON)
```

### Position Runtime State (Redis)

```go
type PositionRuntimeState struct {
    // Identity
    PositionID    string  `json:"pid"`
    Symbol        string  `json:"sym"`
    Side          string  `json:"side"`       // "LONG" or "SHORT"
    Mode          string  `json:"mode"`
    UserID        string  `json:"uid"`
    EntryPrice    float64 `json:"entry"`
    EntryTime     int64   `json:"entry_ts"`

    // Current State
    CurrentPrice  float64 `json:"price"`
    CurrentQty    float64 `json:"qty"`
    LastUpdate    int64   `json:"upd_ts"`

    // Orders on Binance
    SLPrice       float64 `json:"sl"`
    SLOrderID     string  `json:"sl_oid"`
    TPPrice       float64 `json:"tp"`
    TPOrderID     string  `json:"tp_oid"`
    BEPrice       float64 `json:"be"`          // Breakeven price

    // Stage Tracking
    Stage         string  `json:"stage"`       // RISK_ZONE, TP1_DONE, EFFICIENCY
    BEAchieved    bool    `json:"be_done"`
    BETime        int64   `json:"be_ts"`
    TP1Done       bool    `json:"tp1_done"`
    TP1Time       int64   `json:"tp1_ts"`
    TP1Qty        float64 `json:"tp1_qty"`
    EffActive     bool    `json:"eff_active"`  // Efficiency tracking active

    // Efficiency Tracking (SIMPLIFIED!)
    PeakProfit    float64 `json:"peak_pft"`    // Highest profit % achieved
    PeakPrice     float64 `json:"peak_px"`     // Price at peak
    PeakTime      int64   `json:"peak_ts"`     // When peak was achieved
    CurrentProfit float64 `json:"cur_pft"`     // Current profit %
    Efficiency    float64 `json:"eff"`         // currentProfit / peakProfit

    // Trend Analysis (from AI, cached)
    TrendDir      string  `json:"trend"`       // "UP", "DOWN", "SIDEWAYS"
    TrendStrength float64 `json:"trend_str"`
    Reversal      bool    `json:"reversal"`
    ADX           float64 `json:"adx"`
    RSI           float64 `json:"rsi"`
    ATRPct        float64 `json:"atr_pct"`
    TrendTime     int64   `json:"trend_ts"`

    // Exit Decision
    ShouldExit    bool    `json:"exit"`
    ExitReason    string  `json:"exit_reason"`
    ExitUrgency   string  `json:"exit_urg"`    // "immediate", "normal"
}
```

### Data Flow Timeline

```
ORDER FILLED
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Create PositionRuntimeState                                  │
│ 2. Save to Redis: pos:{user_id}:{symbol}:state                 │
│ 3. Add to active set: SADD active:{user_id} {symbol}           │
│ 4. Place SL/TP orders on Binance                               │
│                                                                 │
│ NO PostgreSQL writes!                                           │
└─────────────────────────────────────────────────────────────────┘
     │
     ▼
┌─────────────────────────────────────────────────────────────────┐
│ MONITORING LOOP (Every price tick ~100ms)                      │
│                                                                 │
│ 1. Receive price from Binance WebSocket                        │
│ 2. GET position from Redis                              < 1ms  │
│ 3. Update: price, profit, peak, efficiency              < 0.1ms│
│ 4. Check exit conditions                                < 0.1ms│
│ 5. Update dynamic SL/TP if needed                              │
│ 6. SAVE to Redis                                        < 1ms  │
│                                                                 │
│ Total latency: < 3ms per tick                                  │
│ NO PostgreSQL queries!                                          │
└─────────────────────────────────────────────────────────────────┘
     │
     ▼ (When exit condition met)
┌─────────────────────────────────────────────────────────────────┐
│ TRADE CLOSE                                                     │
│                                                                 │
│ 1. Execute exit on Binance                                     │
│ 2. Write to PostgreSQL: futures_trades                         │
│ 3. Write to PostgreSQL: trade_efficiency_metrics               │
│ 4. Delete from Redis (or set TTL)                              │
│ 5. SREM active:{user_id} {symbol}                              │
│                                                                 │
│ PostgreSQL writes only at close!                               │
└─────────────────────────────────────────────────────────────────┘
```

---

## Part 6: Market Data Caching

### Data Sources

```
┌─────────────────────────────────────────────────────────────────┐
│                 BINANCE DATA SOURCES                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  WEBSOCKET (Real-time, FREE, No Rate Limit):                   │
│  ═══════════════════════════════════════════                   │
│  • Price ticks              → Every ~100ms                     │
│  • Kline streams            → 1m, 5m, 15m, 1h candles         │
│  • Order updates            → Fill notifications               │
│  • Position updates         → Quantity changes                 │
│                                                                 │
│  We subscribe to:                                               │
│    {symbol}@kline_1m                                           │
│    {symbol}@kline_5m                                           │
│    {symbol}@kline_15m                                          │
│    {symbol}@kline_1h                                           │
│    {symbol}@markPrice                                          │
│                                                                 │
│  REST API (Rate Limited - Use Sparingly):                      │
│  ════════════════════════════════════════                      │
│  • Initial historical candles (once at startup)                │
│  • Order placement/modification                                 │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Candle Caching in Redis

```go
// Store candle when it closes (from WebSocket)
func (r *MarketDataCache) OnCandleClose(symbol, interval string, candle Kline) error {
    key := fmt.Sprintf("kline:%s:%s", symbol, interval)

    // Store in Sorted Set (score = timestamp for ordering)
    data, _ := json.Marshal(candle)
    r.client.ZAdd(ctx, key, redis.Z{
        Score:  float64(candle.CloseTime),
        Member: data,
    })

    // Keep rolling window (last 24 hours)
    maxCandles := getMaxCandles(interval)
    r.client.ZRemRangeByRank(ctx, key, 0, -maxCandles-1)

    // Recalculate indicators
    r.updateIndicators(symbol, interval)

    return nil
}

func getMaxCandles(interval string) int {
    switch interval {
    case "1m":  return 1440  // 24 hours
    case "5m":  return 288   // 24 hours
    case "15m": return 96    // 24 hours
    case "1h":  return 24    // 24 hours
    case "4h":  return 6     // 24 hours
    default:    return 100
    }
}

// Get candles instantly from Redis
func (r *MarketDataCache) GetCandles(symbol, interval string, count int) ([]Kline, error) {
    key := fmt.Sprintf("kline:%s:%s", symbol, interval)
    results, _ := r.client.ZRevRange(ctx, key, 0, int64(count-1)).Result()

    candles := make([]Kline, len(results))
    for i, data := range results {
        json.Unmarshal([]byte(data), &candles[i])
    }
    return candles, nil
}
```

### Indicator Updates

```go
// Called when candle closes
func (r *MarketDataCache) updateIndicators(symbol, interval string) {
    candles, _ := r.GetCandles(symbol, interval, 50)

    // Calculate and cache indicators
    rsi := calculateRSI(candles, 14)
    adx := calculateADX(candles, 14)
    atr := calculateATR(candles, 14)
    atrPct := (atr / candles[0].Close) * 100

    // Store in Redis for instant access
    r.client.Set(ctx, fmt.Sprintf("ind:%s:rsi", symbol), rsi, 0)
    r.client.Set(ctx, fmt.Sprintf("ind:%s:adx", symbol), adx, 0)
    r.client.Set(ctx, fmt.Sprintf("ind:%s:atr", symbol), atr, 0)
    r.client.Set(ctx, fmt.Sprintf("ind:%s:atr_pct", symbol), atrPct, 0)
}
```

---

## Part 7: Integration with Position Optimization

### Feature Interaction

```
┌─────────────────────────────────────────────────────────────────┐
│ FEATURE: Position Optimization (Existing - TP1/TP2/TP3)        │
│ FEATURE: Efficiency Exit (Story 10.1 - This Story)             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│ SCENARIO 1: Only Efficiency Exit enabled                       │
│ ═══════════════════════════════════════                        │
│ Entry → Breakeven → Efficiency Tracking → Exit                 │
│                                                                 │
│ SCENARIO 2: Only Position Optimization enabled                 │
│ ═════════════════════════════════════════════                  │
│ Entry → TP1 → TP2 → TP3 → Trail remaining                     │
│ (Existing behavior, unchanged)                                  │
│                                                                 │
│ SCENARIO 3: BOTH enabled (Recommended)                         │
│ ═════════════════════════════════════════                      │
│ Entry → Breakeven → TP1 hits (sell 30%) → Efficiency Tracking  │
│                                                                 │
│ After TP1:                                                      │
│   - We have room to grow (30% profit already booked)           │
│   - Efficiency tracking takes over for remaining 70%           │
│   - Dynamic SL/TP for profit protection                        │
│   - Exit on efficiency decline or trend reversal               │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Configuration

```go
type PositionManagementConfig struct {
    // Position Optimization (staged TPs)
    PositionOptimizationEnabled bool    `json:"pos_opt_enabled"`
    TP1Percent                  float64 `json:"tp1_pct"`        // e.g., 0.4%
    TP1SellPercent              float64 `json:"tp1_sell_pct"`   // e.g., 30%
    // TP2 and TP3 can be 0 to disable

    // Efficiency Exit (Story 10.1)
    EfficiencyExitEnabled       bool    `json:"eff_exit_enabled"`

    // When both enabled, TP1 hits first, then efficiency tracking
    // If only TP1 configured (TP2=0, TP3=0), efficiency takes over after TP1
}
```

### Stage Flow with Both Enabled

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│  Entry (100 units @ $100)                                      │
│       │                                                         │
│       ▼                                                         │
│  RISK_ZONE                                                      │
│       │ Price rises to $100.15 (breakeven)                     │
│       ▼                                                         │
│  BREAKEVEN_ACHIEVED                                             │
│       │ Move SL to $100.15                                     │
│       ▼                                                         │
│  TP1_PENDING (waiting for TP1 at 0.4% = $100.40)               │
│       │ Price hits $100.40                                     │
│       │ Sell 30 units (30%) → Book ~$12 profit                 │
│       ▼                                                         │
│  EFFICIENCY_TRACKING (70 units remaining)                      │
│       │ Peak profit tracked from this point                    │
│       │ Dynamic SL/TP active                                   │
│       │ Trend monitoring active                                │
│       │                                                         │
│       ├── Trend reversal? → EXIT IMMEDIATELY                   │
│       ├── Efficiency < threshold? → EXIT                       │
│       ├── Dynamic SL hit? → EXIT                               │
│       └── Dynamic TP hit? → EXIT (best case)                   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Part 8: Efficiency Calculation Logic

### Core Algorithm

```go
// Called on EVERY price tick
func (ga *GinieAutopilot) updateEfficiency(state *PositionRuntimeState, currentPrice float64) {
    // 1. Calculate current profit %
    if state.Side == "LONG" {
        state.CurrentProfit = (currentPrice - state.EntryPrice) / state.EntryPrice * 100
    } else {
        state.CurrentProfit = (state.EntryPrice - currentPrice) / state.EntryPrice * 100
    }

    // 2. Update peak profit (only goes UP, never down)
    if state.CurrentProfit > state.PeakProfit {
        state.PeakProfit = state.CurrentProfit
        state.PeakPrice = currentPrice
        state.PeakTime = time.Now().Unix()
    }

    // 3. Calculate efficiency (simple division!)
    if state.PeakProfit > 0 {
        state.Efficiency = state.CurrentProfit / state.PeakProfit
    } else {
        state.Efficiency = 1.0  // At entry or below
    }
}

// Check if should exit on efficiency
func (ga *GinieAutopilot) shouldExitOnEfficiency(state *PositionRuntimeState) bool {
    // Only check if efficiency tracking is active
    if !state.EffActive || !state.BEAchieved {
        return false
    }

    // Get threshold from historical baseline (cached in Redis)
    baseline, _ := ga.redis.GetBaseline(state.UserID, state.Mode)

    // Simple comparison!
    return state.Efficiency < baseline.AvgExitEfficiency
}
```

### Complete Decision Engine

```go
func (ga *GinieAutopilot) processPositionTick(symbol string, price float64) error {
    // 1. Get position state from Redis (< 1ms)
    state, err := ga.redis.GetPositionState(ga.userID, symbol)
    if err != nil {
        return err
    }

    // 2. Update live data
    state.CurrentPrice = price
    state.LastUpdate = time.Now().Unix()

    // 3. Update high/low
    if price > state.PeakPrice {
        state.PeakPrice = price
        state.PeakTime = time.Now().Unix()
    }

    // 4. Update efficiency
    ga.updateEfficiency(state, price)

    // 5. Decision engine based on stage
    switch state.Stage {

    case "RISK_ZONE":
        // Check trend reversal (priority 1)
        if state.Reversal && state.TrendStrength > 0.75 {
            state.ShouldExit = true
            state.ExitReason = "TREND_REVERSAL_RISK_ZONE"
            state.ExitUrgency = "immediate"
            break
        }

        // Check if breakeven achieved
        if state.Side == "LONG" && price >= state.BEPrice {
            state.BEAchieved = true
            state.BETime = time.Now().Unix()
            ga.moveSLToBreakeven(state)

            if ga.config.PositionOptimizationEnabled {
                state.Stage = "TP1_PENDING"
            } else {
                state.Stage = "EFFICIENCY"
                state.EffActive = true
            }
        }

    case "TP1_PENDING":
        // Check trend reversal (priority 1)
        if state.Reversal && state.TrendStrength > 0.75 {
            state.ShouldExit = true
            state.ExitReason = "TREND_REVERSAL_TP1_PENDING"
            state.ExitUrgency = "immediate"
            break
        }

        // Check if TP1 hit
        tp1Price := state.EntryPrice * (1 + ga.config.TP1Percent/100)
        if state.Side == "LONG" && price >= tp1Price {
            ga.executeTP1(state)
            state.Stage = "EFFICIENCY"
            state.EffActive = true
            ga.updateDynamicLevels(state)  // Set new SL/TP
        }

    case "EFFICIENCY":
        // PRIORITY 1: Trend reversal
        if state.Reversal && state.TrendStrength > 0.75 {
            state.ShouldExit = true
            state.ExitReason = "TREND_REVERSAL"
            state.ExitUrgency = "immediate"
            break
        }

        // PRIORITY 2: Efficiency check
        if ga.shouldExitOnEfficiency(state) {
            state.ShouldExit = true
            state.ExitReason = fmt.Sprintf("EFFICIENCY_%.1f%%_BELOW_THRESHOLD", state.Efficiency*100)
            state.ExitUrgency = "normal"
            break
        }

        // Update dynamic SL/TP
        ga.updateDynamicLevels(state)
    }

    // 6. Save updated state to Redis
    ga.redis.SavePositionState(state)

    // 7. Execute exit if needed
    if state.ShouldExit {
        return ga.executeExit(state)
    }

    return nil
}
```

---

## Part 9: UI Display - Position Stages

### Expandable Position Card

```
┌─────────────────────────────────────────────────────────────────┐
│ COLLAPSED VIEW (Default)                                        │
├─────────────────────────────────────────────────────────────────┤
│ BTCUSDT LONG       +1.25% ($125)                    [Expand]   │
│ Entry: $100,000 │ Current: $101,250 │ SL: $100,500            │
│ ████████████████░░░░░░░░░░░░░  Efficiency: 74%                │
│ Stage: PROFIT ZONE (Efficiency Tracking)                       │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ EXPANDED VIEW                                                   │
├─────────────────────────────────────────────────────────────────┤
│ BTCUSDT LONG       +1.25% ($125)                  [Collapse]   │
├─────────────────────────────────────────────────────────────────┤
│ PRICE LEVELS                                                    │
│ ───────────────────────────────────────────────────────────────│
│ $102,500 ─── Take Profit (Dynamic)                             │
│     ↑                                                           │
│ $101,250 ─── Current Price                                     │
│     │                                                           │
│ $100,500 ─── Stop Loss (Trailing)                              │
│     │                                                           │
│ $100,150 ─── Breakeven (Achieved)                              │
│     │                                                           │
│ $100,000 ─── Entry Price                                       │
├─────────────────────────────────────────────────────────────────┤
│ POSITION STAGES                                                 │
│ ───────────────────────────────────────────────────────────────│
│ [✓] Stage 1: RISK ZONE          Completed 10:15               │
│     └─ Initial SL at $98,000                                   │
│                                                                 │
│ [✓] Stage 2: BREAKEVEN          Achieved 10:23                │
│     └─ SL moved to $100,150                                    │
│                                                                 │
│ [✓] Stage 3: TP1 HIT            Completed 10:35               │
│     └─ Sold 30% at $100,400 (+$120)                           │
│                                                                 │
│ [→] Stage 4: EFFICIENCY         Active (Current)              │
│     └─ Tracking remaining 70%                                  │
│     └─ Dynamic SL: $100,500                                    │
│     └─ Dynamic TP: $102,500                                    │
├─────────────────────────────────────────────────────────────────┤
│ EFFICIENCY METRICS                                              │
│ ───────────────────────────────────────────────────────────────│
│ Peak Profit:    1.70% (at $101,700)                            │
│ Current Profit: 1.25%                                           │
│ Efficiency:     ████████████████░░░░░░░░  74%                  │
│ Threshold:      48% (from last 6 hours)                        │
│ Status:         HOLDING - Above threshold                      │
├─────────────────────────────────────────────────────────────────┤
│ TREND ANALYSIS                                                  │
│ ───────────────────────────────────────────────────────────────│
│ Direction:      UP (Bullish)                                   │
│ Strength:       ████████████░░░░░░  ADX: 32                    │
│ RSI: 58 │ MACD: Bullish │ ATR: 0.8%                           │
├─────────────────────────────────────────────────────────────────┤
│ [Close Position]  [Adjust SL/TP]  [View History]              │
└─────────────────────────────────────────────────────────────────┘
```

### Stage Indicators

| Stage | Icon | Color | Description |
|-------|------|-------|-------------|
| RISK_ZONE | ⚠️ | Red | Below breakeven, capital at risk |
| BREAKEVEN | ✅ | Green | Just achieved breakeven |
| TP1_PENDING | 🎯 | Blue | Waiting for TP1 |
| TP1_HIT | 💰 | Gold | TP1 completed, profit booked |
| EFFICIENCY | 📈 | Purple | Efficiency tracking active |
| TREND_WARNING | 🔶 | Orange | Trend weakening |
| EXITING | 🔴 | Red | Exit in progress |

---

## Part 10: Database Schema

### Table: trade_efficiency_metrics

```sql
CREATE TABLE trade_efficiency_metrics (
    id SERIAL PRIMARY KEY,
    futures_trade_id INTEGER REFERENCES futures_trades(id),
    user_id UUID REFERENCES users(id),
    symbol VARCHAR(20) NOT NULL,
    mode VARCHAR(20) NOT NULL,

    -- Entry/Exit Data
    entry_price DECIMAL(20,8) NOT NULL,
    exit_price DECIMAL(20,8) NOT NULL,
    entry_time TIMESTAMP NOT NULL,
    exit_time TIMESTAMP NOT NULL,

    -- Quantity
    original_qty DECIMAL(20,8) NOT NULL,
    exit_qty DECIMAL(20,8) NOT NULL,

    -- Efficiency Data (SIMPLIFIED!)
    peak_profit DECIMAL(10,6) NOT NULL,       -- Highest profit % achieved
    exit_profit DECIMAL(10,6) NOT NULL,       -- Profit % at exit
    exit_efficiency DECIMAL(10,6) NOT NULL,   -- exit_profit / peak_profit

    -- Exit Details
    exit_reason VARCHAR(50) NOT NULL,         -- 'efficiency', 'trend', 'sl', 'tp'
    exit_urgency VARCHAR(20),                 -- 'immediate', 'normal'

    -- Stage Data
    breakeven_achieved BOOLEAN DEFAULT FALSE,
    tp1_hit BOOLEAN DEFAULT FALSE,
    tp1_profit DECIMAL(20,8),

    -- Trend at Exit
    trend_direction VARCHAR(20),
    trend_strength DECIMAL(10,6),

    -- Category
    trade_category INTEGER NOT NULL,          -- 1=loss, 2=breakeven, 3=success

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for baseline query
CREATE INDEX idx_eff_user_mode_time ON trade_efficiency_metrics(user_id, mode, created_at);
CREATE INDEX idx_eff_category ON trade_efficiency_metrics(trade_category);
```

---

## Part 11: Implementation Tasks

### Task 1: Redis Infrastructure
- [ ] Add Redis keys for position state
- [ ] Add Redis keys for historical baseline
- [ ] Add Redis keys for market data cache
- [ ] Implement atomic updates with Lua scripts

### Task 2: Position Runtime State
- [ ] Create PositionRuntimeState struct
- [ ] Implement Redis save/load methods
- [ ] Add active positions index management

### Task 3: Simplified Efficiency Tracking
- [ ] Implement peak profit tracking (every tick)
- [ ] Implement efficiency calculation
- [ ] Remove all rate-per-unit code

### Task 4: Historical Baseline (Simplified)
- [ ] Store exit_efficiency on trade close
- [ ] Implement hourly baseline refresh
- [ ] Calculate average exit efficiency as threshold

### Task 5: Dynamic SL/TP
- [ ] Implement calculateDynamicSL()
- [ ] Implement calculateDynamicTP()
- [ ] Update Binance orders on improvement

### Task 6: Trend-Based Exit
- [ ] Implement trend reversal detection
- [ ] Add immediate exit on confirmed reversal
- [ ] Add trend data to position state

### Task 7: Stage Management
- [ ] Implement stage transitions
- [ ] Integrate with Position Optimization
- [ ] Handle TP1 → Efficiency handoff

### Task 8: Market Data Caching
- [ ] Cache candles in Redis Sorted Sets
- [ ] Implement rolling window cleanup
- [ ] Cache indicators on candle close

### Task 9: UI Updates
- [ ] Add expandable position card
- [ ] Display stage information
- [ ] Show efficiency metrics

### Task 10: Database Changes
- [ ] Create trade_efficiency_metrics table
- [ ] Add migration script
- [ ] Implement repository methods

### Task 11: Testing
- [ ] Unit tests for efficiency calculation
- [ ] Unit tests for baseline calculation
- [ ] Integration tests with Redis
- [ ] End-to-end position lifecycle test

---

## Part 12: Acceptance Criteria

### AC10.1.1: Simplified Efficiency Tracking
- [ ] Peak profit tracked every tick (not candle-based)
- [ ] Efficiency = currentProfit / peakProfit
- [ ] No rate-per-unit calculations
- [ ] Updates in < 1ms

### AC10.1.2: Historical Baseline
- [ ] Exit efficiency stored on trade close
- [ ] Threshold = average exit_efficiency from last 4-8 hours
- [ ] Refreshed every 1 hour
- [ ] Stored in Redis for instant access

### AC10.1.3: Trend-Based Exit
- [ ] Trend reversal triggers immediate exit
- [ ] Exit before SL when trend confirms reversal
- [ ] Market order for emergency exits
- [ ] Trend data cached in Redis

### AC10.1.4: Dynamic SL/TP
- [ ] SL calculated from ATR + trend + efficiency
- [ ] TP trails upward with peak price
- [ ] Both updated on Binance (not just internal)
- [ ] Only moves in favorable direction

### AC10.1.5: Redis-First Architecture
- [ ] All decision data in Redis
- [ ] No PostgreSQL queries during active position
- [ ] Position state updated every tick
- [ ] Total decision latency < 3ms

### AC10.1.6: Position Optimization Integration
- [ ] Works with Position Optimization disabled
- [ ] Works with Position Optimization enabled
- [ ] Handoff from TP1 to efficiency tracking
- [ ] Stage transitions tracked

### AC10.1.7: UI Display
- [ ] Expandable position card
- [ ] Stage progress visible
- [ ] Efficiency metrics displayed
- [ ] Trend status shown

---

## Summary

### The Complete System

```
ENTRY → RISK_ZONE → BREAKEVEN → [TP1] → EFFICIENCY_TRACKING → EXIT

Exit Priority:
  1. Trend Reversal → IMMEDIATE EXIT
  2. Efficiency < Threshold → NORMAL EXIT
  3. Trailing SL Hit → Binance handles
  4. Dynamic TP Hit → Binance handles

Core Formula:
  EFFICIENCY = currentProfit / peakProfit
  THRESHOLD = average(exit_efficiency) from history
  EXIT when efficiency < threshold

Architecture:
  - Redis for ALL real-time data
  - PostgreSQL only for closed trades
  - WebSocket for price data (no rate limits)
  - < 3ms decision latency
```

### Key Simplifications from Original Story

| Aspect | Original | Simplified |
|--------|----------|------------|
| Efficiency | Rate per time unit | Profit / Peak Profit |
| Threshold | Complex formula with fees | Average exit efficiency |
| Candle dependency | Check at candle close | Check every tick |
| Historical data | Rate calculations | Just exit_efficiency |
| Code complexity | High | Low |

### Files to Modify

| File | Changes |
|------|---------|
| `internal/autopilot/ginie_types.go` | Add PositionRuntimeState |
| `internal/autopilot/ginie_autopilot.go` | Add efficiency tracking |
| `internal/autopilot/position_redis.go` | New file - Redis operations |
| `internal/autopilot/market_cache.go` | New file - Market data cache |
| `internal/db/migrations/` | Add trade_efficiency_metrics table |
| `web/src/components/PositionCard.tsx` | Add expandable view |

---

**This story is ready for implementation.**
