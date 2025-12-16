# Backtesting Guide

## Why Backtest?

Backtesting validates your trading strategies against historical data **before risking real money**. It answers critical questions:

- Does this pattern actually work?
- What's the real win rate?
- How much can I expect to lose during drawdowns?
- Is this strategy profitable long-term?

**Rule:** Never trade a strategy live without backtesting it first.

---

## How Our Backtesting Engine Works

### Core Components

1. **Historical Data:** Candlestick data from past market conditions
2. **Strategy Function:** Your trading logic (entry/exit rules)
3. **Simulation:** Execute trades as if trading live
4. **Metrics:** Calculate performance statistics

### Key Metrics Explained

#### Win Rate
`Win Rate = (Winning Trades / Total Trades) × 100%`

- **Good:** 55-65%
- **Excellent:** 65%+
- **Warning:** <50% means losing strategy

#### Return on Investment (ROI)
`ROI = (Net Profit / Initial Capital) × 100%`

- Measures overall profitability
- **Example:** $10,000 → $12,500 = 25% ROI

#### Profit Factor
`Profit Factor = Total Profit / Total Loss`

- **Above 2.0:** Excellent strategy
- **1.5-2.0:** Good strategy
- **1.0-1.5:** Marginal strategy
- **Below 1.0:** Losing strategy

#### Maximum Drawdown
- Largest peak-to-valley equity drop
- **Example:** Account goes from $12,000 → $9,000 = 25% drawdown
- **Acceptable:** <20%
- **Warning:** >30%

#### Sharpe Ratio
- Risk-adjusted returns
- **Above 2.0:** Excellent
- **1.0-2.0:** Good
- **Below 1.0:** Poor risk/reward

---

## Using the Backtest Engine

### Basic Example

```go
package main

import (
    "binance-trading-bot/internal/backtest"
    "binance-trading-bot/internal/patterns"
    "time"
)

func main() {
    // 1. Set backtest parameters
    startDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
    endDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
    initialCapital := 10000.0  // $10,000
    commission := 0.001        // 0.1% trading fee

    // 2. Create backtest engine
    engine := backtest.NewBacktestEngine(startDate, endDate, initialCapital, commission)

    // 3. Load historical candles
    candles := loadHistoricalData("BTCUSDT", "1h") // Your data source

    // 4. Define strategy
    strategy := func(candles []binance.Kline, currentIndex int) (*backtest.Signal, error) {
        // Your trading logic here
        detector := patterns.NewPatternDetector(0.5)

        // Detect morning star
        recentCandles := candles[currentIndex-50:currentIndex+1]
        morningStars := detector.DetectPatterns("BTCUSDT", "1h", recentCandles)

        for _, pattern := range morningStars {
            if pattern.Type == patterns.MorningStar && pattern.Confidence > 0.7 {
                currentPrice := candles[currentIndex].Close
                return &backtest.Signal{
                    Action:     "BUY",
                    Price:      currentPrice,
                    StopLoss:   currentPrice * 0.98,  // 2% stop loss
                    TakeProfit: currentPrice * 1.05,  // 5% take profit
                    Pattern:    pattern.Type,
                }, nil
            }
        }

        return nil, nil
    }

    // 5. Run backtest
    result, err := engine.RunBacktest(candles, strategy)
    if err != nil {
        panic(err)
    }

    // 6. Print results
    engine.PrintResults(result)
}
```

### Output Example

```
=== BACKTEST RESULTS ===
Total Trades: 87
Winning Trades: 56 (64.4%)
Losing Trades: 31
Net Profit: $3,245.67
ROI: 32.46%
Profit Factor: 2.15
Max Drawdown: 12.3%
Average Win: $125.40
Average Loss: $62.18
Sharpe Ratio: 1.87

=== PATTERN PERFORMANCE ===
morning_star: 45 trades, 68.9% win rate, Net: $2,145.32
bullish_engulfing: 25 trades, 72.0% win rate, Net: $1,523.45
hammer: 17 trades, 52.9% win rate, Net: -$423.10
```

---

## Interpreting Results

### This Strategy is GOOD ✅

- Win Rate: 64.4% (above 60%)
- ROI: 32.46% (profitable)
- Profit Factor: 2.15 (excellent - above 2.0)
- Max Drawdown: 12.3% (acceptable - below 20%)
- Sharpe Ratio: 1.87 (good risk/reward)

### Pattern Insights

**Morning Star:**
- Most trades (45)
- High win rate (68.9%)
- Best profit ($2,145)
- **Action:** Focus on this pattern

**Hammer:**
- Low win rate (52.9%)
- Lost money (-$423)
- **Action:** Remove from strategy or add more filters

---

## Backtest Validation Checklist

### ✅ Data Quality
- [ ] At least 6-12 months of data
- [ ] Multiple market conditions (bull, bear, sideways)
- [ ] Clean data (no gaps or errors)

### ✅ Realistic Assumptions
- [ ] Include trading fees (0.1-0.2%)
- [ ] Account for slippage
- [ ] Use realistic position sizes
- [ ] Don't peek into the future (no lookahead bias)

### ✅ Statistical Significance
- [ ] Minimum 100 trades
- [ ] Test across multiple symbols
- [ ] Test different timeframes
- [ ] Verify results with forward testing

---

## Common Backtesting Mistakes

### 1. Overfitting (Curve Fitting)
**Problem:** Strategy works perfectly on historical data but fails live

**Example:**
- Optimize parameters to get 90% win rate on 2023 data
- Strategy fails in 2024 because it was tailored to past conditions

**Solution:**
- Use simple strategies
- Test on out-of-sample data
- Avoid excessive parameter optimization

### 2. Lookahead Bias
**Problem:** Using future information in past decisions

**Example:**
```go
// WRONG - uses future candle data
if candles[i+1].Close > candles[i].Close {
    // Enter trade based on NEXT candle
}

// CORRECT - only uses past data
if candles[i].Close > candles[i-1].Close {
    // Enter trade based on PREVIOUS candle
}
```

### 3. Ignoring Costs
**Problem:** Forgetting trading fees and slippage

**Reality:**
- Buy at $100
- Sell at $105
- **Gross Profit:** $5 (5%)
- **After 0.1% fees:** $4.80 (4.8%)
- **After slippage:** $4.50 (4.5%)

### 4. Cherry-Picking Data
**Problem:** Only testing on favorable market conditions

**Solution:** Test on:
- Bull markets (2020-2021)
- Bear markets (2022)
- Sideways markets (2019)

---

## Advanced Backtesting

### Walk-Forward Analysis

Instead of one backtest, do multiple:

1. **Train Period:** Optimize on 2022 data
2. **Test Period:** Validate on 2023 data
3. **Train Period:** Optimize on 2023 data
4. **Test Period:** Validate on 2024 data

This prevents overfitting to a single time period.

### Monte Carlo Simulation

Run backtest 1,000 times with random trade sequences to understand:
- Worst-case scenarios
- Probability of ruin
- Confidence intervals

---

## Pattern-Specific Backtesting

### Test Each Pattern Individually

```go
// Backtest ONLY Morning Star
patterns := detector.DetectReversalPatterns(symbol, timeframe, candles)
morningStars := filterByType(patterns, patterns.MorningStar)

// Backtest ONLY Bullish Engulfing
engulfing := filterByType(patterns, patterns.BullishEngulfing)
```

**Goal:** Identify which patterns work best for your trading style

### Confluence Backtesting

Test pattern + FVG combinations:

```go
if morningStarDetected && priceInFVG && highVolume {
    // Triple confluence - backtest this specific combination
}
```

---

## Real-World Backtest Example

### Strategy: FVG + Morning Star + Volume

**Rules:**
1. Identify bullish FVG on 4H chart
2. Wait for price to enter FVG
3. Look for Morning Star pattern
4. Confirm volume >2x average
5. Enter long with 2% stop, 5% target

**Backtest Results (BTC 2023):**
- Trades: 23
- Win Rate: 78.3%
- Net Profit: $4,567
- Max Drawdown: 8.9%
- Sharpe: 2.34

**Conclusion:** High-quality, low-frequency strategy ✅

---

## Next Steps

1. **Run Your First Backtest**
   - Start simple (single pattern)
   - Use 6 months of data
   - Analyze results

2. **Iterate and Improve**
   - Remove losing patterns
   - Add confluence filters
   - Optimize stop/target levels

3. **Forward Test**
   - Run strategy on NEW data (not used in backtest)
   - Verify results hold up

4. **Paper Trade**
   - Test live with fake money
   - Verify execution matches backtest

5. **Go Live (Small)**
   - Start with tiny positions
   - Scale up gradually as confidence builds

---

## Warning Signs

### 🚨 Don't Trade Live If:
- Win rate <50%
- Profit factor <1.5
- Max drawdown >30%
- Less than 50 backtested trades
- Results don't match forward test
- You don't understand why it works

---

**Remember:** Backtesting is a tool, not a guarantee. Past performance doesn't guarantee future results. But it's WAY better than guessing!

---

**Related Documentation:**
- [Candlestick Patterns Guide](./candlestick-patterns.md)
- [FVG Trading Guide](./fvg-guide.md)
- [Multi-Timeframe Analysis](./multi-timeframe-guide.md)
