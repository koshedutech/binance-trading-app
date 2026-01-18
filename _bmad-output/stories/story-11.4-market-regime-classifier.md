# Story 11.4: Market Regime Classifier

**Epic:** 11 - Position Decision Engine
**Priority:** P1
**Status:** done
**Created:** 2026-01-18

## Goal

Implement automatic market regime detection for each coin. The classifier analyzes technical indicators (ADX, ATR) to determine the current market condition and recommend appropriate trading strategies.

## Acceptance Criteria

- [x] Real-time regime classification based on ADX and ATR
- [x] Regime stored in Redis state (via CoinState.Regime)
- [x] Regime change triggers strategy re-evaluation
- [x] Historical regime tracking for analysis
- [x] Configurable thresholds per timeframe

## Market Regimes

| Regime | Characteristics | Preferred Strategy |
|--------|-----------------|-------------------|
| TRENDING | ADX > 25, directional movement | Trend Following |
| RANGING | ADX < 20, price oscillating | Mean Reversion |
| VOLATILE | High ATR, wide swings | Breakout |
| CONSOLIDATING | Low ATR, tight range | Range Trading |

## Implementation Tasks

### Task 1: Create Regime Classifier (regime_classifier.go)
- Create RegimeClassifier struct with configuration and history tracking
- Implement RegimeConfig for configurable thresholds
- Implement RegimeChange struct for historical tracking
- Implement RegimeIndicators struct for indicator snapshots

### Task 2: Implement Classification Logic
- Classify method to determine regime from CoinState
- Handle ADX thresholds for trending vs ranging
- Handle ATR comparison for volatile vs consolidating
- Minimum regime duration before allowing change

### Task 3: Implement State Integration
- UpdateRegime method to update CoinState via StateManager
- GetHistory method for regime change analysis
- GetCurrentStreak method for current regime duration

### Task 4: Write Unit Tests (regime_classifier_test.go)
- TestRegimeClassifier_TrendingADX
- TestRegimeClassifier_RangingADX
- TestRegimeClassifier_Volatile
- TestRegimeClassifier_Consolidating
- TestRegimeClassifier_History
- TestRegimeClassifier_ConfigurableThresholds

## Technical Design

### Classification Algorithm

```
1. If ADX >= ADXTrendingThreshold (25): TRENDING
2. Else if ADX < ADXRangingThreshold (20):
   - If ATR > ATRAverage * ATRVolatileMultiplier (1.5): VOLATILE
   - Else if ATR < ATRAverage * ATRConsolidatingDivisor (0.5): CONSOLIDATING
   - Else: RANGING
3. Default: RANGING
```

### Default Configuration

- ADXTrendingThreshold: 25
- ADXRangingThreshold: 20
- ATRVolatileMultiplier: 1.5
- ATRConsolidatingDivisor: 0.5
- MinRegimeDuration: 3 candles
- HistoryMaxSize: 100 changes

### Preferred Strategies by Regime

| Regime | Strategy |
|--------|----------|
| TRENDING | trend_following |
| RANGING | mean_reversion |
| VOLATILE | breakout |
| CONSOLIDATING | range_trading |

## Files to Create

1. `internal/decision/regime_classifier.go`
2. `internal/decision/regime_classifier_test.go`

## Dependencies

- `internal/decision/coin_state.go` - CoinState struct and MarketRegime enum
- `internal/decision/state_manager.go` - StateManager for Redis operations

## Notes

- Use existing MarketRegime constants from coin_state.go
- Thread-safe history tracking with sync.RWMutex
- History is per-symbol, stored in memory (not Redis)

---

## Change Log

| Date | Status | Notes |
|------|--------|-------|
| 2026-01-18 | in-progress | Story created and implementation started |
| 2026-01-18 | done | Implementation complete - all tests pass |
