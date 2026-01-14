# Epic 10: Position Management & Optimization

## Epic Overview

**Epic ID:** EPIC-10
**Status:** Ready for Implementation
**Created:** 2026-01-14
**Last Updated:** 2026-01-14
**Priority:** High

---

## Vision

Optimize position management with:
1. **Simplified Efficiency Tracking** - Exit when profit efficiency declines
2. **Trend-Based Exit Priority** - Exit immediately on trend reversal
3. **Dynamic SL/TP on Binance** - Active profit protection
4. **Redis-First Architecture** - Millisecond decision latency

---

## Problem Statement

Current position management issues:

1. **Positions held too long** - No efficiency tracking leads to diminishing returns
2. **Trailing stop is software-only** - Binance SL not updated, profits not protected
3. **No trend-based exit** - System waits for SL instead of exiting on reversal
4. **Decision latency** - Database queries slow down critical decisions

### Data Insights (from analysis)

| Hold Duration | Avg ROI | Observation |
|---------------|---------|-------------|
| < 15 min | **1.72%** | Highest efficiency |
| 15-30 min | 0.45% | Declining |
| 30-60 min | 0.15% | Poor |
| > 60 min | **0.02%** | Very poor |

**Conclusion:** Fast exits with high efficiency are better than holding for small additional gains.

---

## Core Concept: Simplified Efficiency

```
EFFICIENCY = currentProfit / peakProfit

THRESHOLD = average(exit_efficiency) from last 4-8 hours

EXIT when efficiency < threshold
```

No complex rate-per-unit calculations. Just simple profit comparison.

---

## Stories

### Story 10.1: Position Management & Efficiency Exit System
**Priority:** P1
**Status:** Ready for Implementation
**File:** `story-10.1-position-management-efficiency-exit.md`

Complete position management system including:
- Simplified efficiency tracking (every tick, not candle-based)
- Trend-based exit priority (trend reversal = immediate exit)
- Dynamic SL/TP updated on Binance
- Redis-first architecture for millisecond decisions
- Integration with Position Optimization (TP1/TP2/TP3)
- Historical baseline from average exit efficiency
- UI display with expandable position cards

### Story 10.2: Position Analytics Dashboard
**Priority:** P2
**Status:** Planning

UI and analytics for position efficiency:
- Historical efficiency analysis
- Trade categorization charts
- Performance metrics by mode
- Export capabilities

---

## Architecture Overview

### Data Flow

```
BINANCE WEBSOCKET          REDIS                    POSTGRESQL
     │                       │                           │
     │ Price ticks           │ Position state           │
     │ ─────────────────────>│ Efficiency               │
     │ Candle data           │ Trend cache              │
     │                       │ Market data              │
     │                       │                           │
     │                       │ (On trade close only)    │
     │                       │ ─────────────────────────>│ Trade records
     │                       │                           │ Efficiency metrics
```

### Exit Priority

| Priority | Condition | Action |
|----------|-----------|--------|
| 1 | Trend Reversal | EXIT IMMEDIATELY |
| 2 | Efficiency < Threshold | EXIT |
| 3 | Trailing SL Hit | Binance handles |
| 4 | Dynamic TP Hit | Binance handles |

### Position Stages

```
ENTRY → RISK_ZONE → BREAKEVEN → [TP1] → EFFICIENCY_TRACKING → EXIT
```

---

## Key Simplifications

| Aspect | Old Approach | New Approach |
|--------|--------------|--------------|
| Efficiency | Rate per time unit | currentProfit / peakProfit |
| Threshold | Complex formula | Average exit efficiency |
| Checking | At candle boundaries | Every tick |
| Historical | Rate calculations | Just exit_efficiency |

---

## Success Metrics

| Metric | Target |
|--------|--------|
| Decision latency | < 3ms |
| Profit protection | 90% of peak captured |
| Average hold time | Reduce by 30% |
| Success rate | Maintain or improve |

---

## Dependencies

| Dependency | Status | Notes |
|------------|--------|-------|
| Redis infrastructure | Existing | Already in use |
| Binance WebSocket | Existing | Already subscribed |
| Position Optimization | Existing | Will integrate |

---

## References

- Story 10.1: Position Management & Efficiency Exit System
- Analysis session: 48-hour trade history showing optimal hold times
- Discussion: Party mode session 2026-01-14
