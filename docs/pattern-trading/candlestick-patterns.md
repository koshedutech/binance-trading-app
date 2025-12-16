# Candlestick Pattern Trading Guide

## Overview

Candlestick patterns are visual representations of price action that signal potential market reversals or continuations. This guide covers all 17 patterns implemented in the trading bot.

---

## Reversal Patterns (11 Total)

### 1. Morning Star ⭐ (Bullish Reversal)

**Formation:** 3-candle pattern
**Win Rate:** 65-70%
**Best Timeframe:** 1H, 4H, Daily

**Structure:**
- Candle 1: Long bearish (red) candle
- Candle 2: Small body (indecision) - can be bullish or bearish
- Candle 3: Long bullish (green) candle closing above C1 midpoint

**Signal:** Strong bullish reversal after downtrend

**Trading:**
- Entry: Close of 3rd candle or pullback
- Stop: Below candle 2 low
- Target: Previous resistance

---

### 2. Evening Star 🌙 (Bearish Reversal)

**Formation:** 3-candle pattern
**Win Rate:** 60-65%
**Best Timeframe:** 1H, 4H, Daily

**Structure:**
- Candle 1: Long bullish candle
- Candle 2: Small body (indecision)
- Candle 3: Long bearish candle closing below C1 midpoint

**Signal:** Strong bearish reversal after uptrend

---

### 3. Bullish Engulfing 🟢 (Bullish Reversal)

**Formation:** 2-candle pattern
**Win Rate:** 70-75%
**Best Timeframe:** 4H, Daily

**Structure:**
- Candle 1: Bearish candle
- Candle 2: Bullish candle that completely engulfs C1 body

**Signal:** Bulls overpowering bears

**Why it works:** Shows shift in momentum from sellers to buyers

---

### 4. Bearish Engulfing 🔴 (Bearish Reversal)

**Formation:** 2-candle pattern
**Win Rate:** 70-75%

**Structure:**
- Candle 1: Bullish candle
- Candle 2: Bearish candle that completely engulfs C1 body

**Signal:** Bears overpowering bulls

---

### 5. Hammer 🔨 (Bullish Reversal)

**Formation:** 1-candle pattern
**Win Rate:** 55-60%
**Best Timeframe:** Daily

**Structure:**
- Small body at top of candle
- Long lower wick (at least 2x body size)
- Little to no upper wick
- Appears after downtrend

**Signal:** Rejection of lower prices, potential bounce

---

### 6. Shooting Star ☄️ (Bearish Reversal)

**Formation:** 1-candle pattern
**Win Rate:** 60-65%

**Structure:**
- Small body at bottom of candle
- Long upper wick (at least 2x body size)
- Little to no lower wick
- Appears after uptrend

**Signal:** Rejection of higher prices, potential pullback

---

### 7. Hanging Man 🪢 (Bearish Reversal)

**Formation:** 1-candle pattern
**Win Rate:** 55-60%

**Structure:**
- Same shape as Hammer
- Appears at TOP of uptrend (not bottom)
- Long lower wick
- Small body

**Signal:** Distribution, sellers testing lower prices

---

### 8. Doji ➕ (Indecision)

**Formation:** 1-candle pattern
**Win Rate:** 50-55% (low reliability alone)

**Structure:**
- Open and close nearly identical
- Small body (< 10% of total range)
- Can have long wicks

**Signal:** Market indecision, potential reversal

**Important:** Only trade Doji with strong confluence

---

### 9. Dragonfly Doji 🪰 (Bullish)

**Formation:** 1-candle pattern
**Win Rate:** 60-65%

**Structure:**
- Open/close at the high
- Long lower wick
- No upper wick

**Signal:** Strong rejection of lower prices

---

### 10. Gravestone Doji 🪦 (Bearish)

**Formation:** 1-candle pattern
**Win Rate:** 60-65%

**Structure:**
- Open/close at the low
- Long upper wick
- No lower wick

**Signal:** Strong rejection of higher prices

---

### 11. Bullish/Bearish Harami 🤰 (Reversal)

**Formation:** 2-candle pattern
**Win Rate:** 60-65%

**Structure:**
- Candle 1: Large body (bullish or bearish)
- Candle 2: Small opposite-color candle INSIDE C1 body

**Signal:** Momentum weakening, potential reversal

---

## Continuation Patterns (4 Total)

### 12. Bullish Flag 🚩 (Continuation)

**Formation:** Multi-candle pattern
**Win Rate:** 65-70%
**Best Timeframe:** 1H, 4H

**Structure:**
- Strong upward move (pole)
- Brief downward consolidation (flag)
- Breakout continuation upward

**Target:** Pole height projected from breakout

---

### 13. Bearish Flag 🏴 (Continuation)

**Formation:** Multi-candle pattern
**Win Rate:** 65-70%

**Structure:**
- Strong downward move (pole)
- Brief upward consolidation (flag)
- Breakdown continuation downward

---

### 14. Ascending Triangle △ (Bullish Continuation)

**Formation:** 10+ candle pattern
**Win Rate:** 70-75%

**Structure:**
- Flat resistance (horizontal line at top)
- Rising support (higher lows)
- Breakout upward

**Signal:** Buyers gaining strength

---

### 15. Descending Triangle ▽ (Bearish Continuation)

**Formation:** 10+ candle pattern
**Win Rate:** 70-75%

**Structure:**
- Flat support (horizontal line at bottom)
- Descending resistance (lower highs)
- Breakdown downward

**Signal:** Sellers gaining strength

---

## Pattern Reliability Tiers

### Tier 1 (Highest Reliability - 70%+)
- Bullish/Bearish Engulfing
- Ascending/Descending Triangles
- Bullish/Bearish Flags

### Tier 2 (Good Reliability - 60-70%)
- Morning/Evening Star
- Dragonfly/Gravestone Doji
- Shooting Star
- Harami patterns

### Tier 3 (Moderate Reliability - 55-60%)
- Hammer
- Hanging Man

### Tier 4 (Low Reliability - 50-55%)
- Regular Doji (needs strong confluence)

---

## Multi-Timeframe Pattern Strategy

### Setup Example:

**Daily Chart:**
- Identify trend: Bullish
- Pattern: Morning Star detected

**4H Chart:**
- Confirm: Price at support zone
- FVG: Bullish FVG nearby

**1H Chart:**
- Volume: High volume on reversal candle
- Entry timing: Precise entry on pullback

**Confluence Score: 85% (Grade: A)**

---

## Best Practices

### ✅ Do's
1. **Always consider trend:** Trade reversals IN trend direction (pullbacks)
2. **Wait for confirmation:** Don't enter on pattern alone
3. **Check volume:** Patterns with high volume are more reliable
4. **Use stop losses:** Place stops just beyond pattern extremes
5. **Combine patterns:** FVG + Pattern + Volume = high probability

### ❌ Don'ts
1. **Don't trade against strong trends:** Reversal patterns in strong trends often fail
2. **Don't ignore context:** A hammer at resistance is weak
3. **Don't chase:** Wait for pullback after pattern completes
4. **Don't overtrade:** Only take high-confidence setups
5. **Don't use Doji alone:** Too unreliable without confluence

---

## Pattern + FVG Combination

**Highest Win Rate Setup (75-80%):**

1. Identify FVG zone
2. Wait for price to return to FVG
3. Look for reversal pattern INSIDE FVG
4. Confirm with volume spike
5. Enter with tight stop below/above FVG

**Example:**
- Bullish FVG at $3,100-$3,150
- Price returns to $3,120
- Morning Star forms at $3,125
- Volume 3x average
- **ENTRY:** Long at $3,130
- **STOP:** $3,095 (below FVG)
- **TARGET:** $3,300

---

## Bot Configuration

```go
// Enable pattern detection
detector := patterns.NewPatternDetector(0.5) // 0.5% min body size

// Detect all patterns
reversalPatterns := detector.DetectReversalPatterns("BTCUSDT", "1h", candles)
continuationPatterns := detector.DetectContinuationPatterns("BTCUSDT", "1h", candles)

// Filter by confidence
for _, pattern := range reversalPatterns {
    if pattern.Confidence > 0.70 {
        // High confidence pattern - consider trading
    }
}
```

---

## Next Steps

1. Study patterns on historical charts (100+ examples each)
2. Practice identifying patterns manually
3. Backtest patterns on your favorite pairs
4. Enable pattern detection in bot (dry-run first)
5. Track performance per pattern type
6. Refine strategy based on results

---

**Related Documentation:**
- [FVG Trading Guide](./fvg-guide.md)
- [Volume Analysis](./volume-guide.md)
- [Backtesting Guide](./backtesting-guide.md)
- [Multi-Timeframe Analysis](./multi-timeframe-guide.md)
