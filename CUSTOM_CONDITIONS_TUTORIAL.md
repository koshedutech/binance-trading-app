# Creating Custom Trading Conditions - Complete Tutorial

This tutorial will guide you through creating custom trading conditions for the Binance Trading Bot, from simple to advanced scenarios.

## Table of Contents
1. [Basic Custom Condition](#basic-custom-condition)
2. [Multi-Condition Strategy](#multi-condition-strategy)
3. [Time-Based Conditions](#time-based-conditions)
4. [Price Pattern Recognition](#price-pattern-recognition)
5. [Volume Analysis Conditions](#volume-analysis-conditions)
6. [Advanced Technical Indicators](#advanced-technical-indicators)
7. [Combining Multiple Signals](#combining-multiple-signals)
8. [Dynamic Order Modification](#dynamic-order-modification)

---

## 1. Basic Custom Condition

Let's start with a simple condition: Buy when price is above 50-period SMA and volume increases.

```go
package strategy

import (
    "binance-trading-bot/internal/binance"
    "fmt"
    "time"
)

type SimpleTrendConfig struct {
    Symbol       string
    Interval     string
    MAPeriod     int
    VolumeMultiplier float64
    PositionSize float64
    StopLoss     float64
    TakeProfit   float64
}

type SimpleTrendStrategy struct {
    config *SimpleTrendConfig
}

func NewSimpleTrendStrategy(config *SimpleTrendConfig) *SimpleTrendStrategy {
    return &SimpleTrendStrategy{config: config}
}

func (s *SimpleTrendStrategy) Name() string {
    return fmt.Sprintf("SimpleTrend-%s", s.config.Symbol)
}

func (s *SimpleTrendStrategy) GetSymbol() string {
    return s.config.Symbol
}

func (s *SimpleTrendStrategy) GetInterval() string {
    return s.config.Interval
}

func (s *SimpleTrendStrategy) Evaluate(klines []binance.Kline, currentPrice float64) (*Signal, error) {
    if len(klines) < s.config.MAPeriod {
        return &Signal{Type: SignalNone}, nil
    }

    // Calculate 50-period SMA
    sma := calculateSMA(klines, s.config.MAPeriod)
    
    // Calculate average volume
    avgVolume := calculateAverageVolume(klines[:len(klines)-1], 20)
    lastVolume := klines[len(klines)-2].Volume

    // Condition 1: Price above SMA
    aboveSMA := currentPrice > sma
    
    // Condition 2: Volume increase
    volumeIncrease := lastVolume > avgVolume * s.config.VolumeMultiplier

    if aboveSMA && volumeIncrease {
        return &Signal{
            Type:       SignalBuy,
            Symbol:     s.config.Symbol,
            EntryPrice: currentPrice,
            StopLoss:   currentPrice * (1 - s.config.StopLoss),
            TakeProfit: currentPrice * (1 + s.config.TakeProfit),
            OrderType:  "LIMIT",
            Side:       "BUY",
            Reason:     fmt.Sprintf("Price %.2f above SMA %.2f with high volume", currentPrice, sma),
            Timestamp:  time.Now(),
        }, nil
    }

    return &Signal{Type: SignalNone}, nil
}

// Helper functions
func calculateSMA(klines []binance.Kline, period int) float64 {
    sum := 0.0
    start := len(klines) - period
    for i := start; i < len(klines); i++ {
        sum += klines[i].Close
    }
    return sum / float64(period)
}

func calculateAverageVolume(klines []binance.Kline, period int) float64 {
    if period > len(klines) {
        period = len(klines)
    }
    sum := 0.0
    start := len(klines) - period
    for i := start; i < len(klines); i++ {
        sum += klines[i].Volume
    }
    return sum / float64(period)
}
```

**Usage:**
```go
strategy := NewSimpleTrendStrategy(&SimpleTrendConfig{
    Symbol:           "BTCUSDT",
    Interval:         "15m",
    MAPeriod:         50,
    VolumeMultiplier: 1.5,
    PositionSize:     0.01,
    StopLoss:         0.02,
    TakeProfit:       0.05,
})
```

---

## 2. Multi-Condition Strategy

Now let's create a strategy that requires ALL of multiple conditions to be true.

```go
type MultiConditionConfig struct {
    Symbol     string
    Interval   string
    Conditions []ConditionChecker
    StopLoss   float64
    TakeProfit float64
}

type ConditionChecker interface {
    Check(klines []binance.Kline, currentPrice float64) (bool, string)
}

// Example Condition: Price above moving average
type PriceAboveMACondition struct {
    Period int
}

func (c *PriceAboveMACondition) Check(klines []binance.Kline, currentPrice float64) (bool, string) {
    if len(klines) < c.Period {
        return false, "insufficient data"
    }
    
    ma := calculateSMA(klines, c.Period)
    if currentPrice > ma {
        return true, fmt.Sprintf("Price %.2f > MA%d %.2f", currentPrice, c.Period, ma)
    }
    return false, fmt.Sprintf("Price %.2f < MA%d %.2f", currentPrice, c.Period, ma)
}

// Example Condition: RSI in range
type RSIRangeCondition struct {
    Period int
    Min    float64
    Max    float64
}

func (c *RSIRangeCondition) Check(klines []binance.Kline, currentPrice float64) (bool, string) {
    rsi := calculateRSI(klines, c.Period)
    if rsi >= c.Min && rsi <= c.Max {
        return true, fmt.Sprintf("RSI %.2f in range [%.0f, %.0f]", rsi, c.Min, c.Max)
    }
    return false, fmt.Sprintf("RSI %.2f outside range", rsi)
}

// Example Condition: Volume spike
type VolumeSpikeCondition struct {
    Multiplier float64
    Period     int
}

func (c *VolumeSpikeCondition) Check(klines []binance.Kline, currentPrice float64) (bool, string) {
    if len(klines) < c.Period+1 {
        return false, "insufficient data"
    }
    
    lastCandle := klines[len(klines)-2]
    avgVol := calculateAverageVolume(klines[:len(klines)-1], c.Period)
    
    if lastCandle.Volume > avgVol*c.Multiplier {
        return true, fmt.Sprintf("Volume spike: %.0f (%.1fx avg)", lastCandle.Volume, lastCandle.Volume/avgVol)
    }
    return false, "no volume spike"
}

type MultiConditionStrategy struct {
    config *MultiConditionConfig
}

func (s *MultiConditionStrategy) Evaluate(klines []binance.Kline, currentPrice float64) (*Signal, error) {
    reasons := make([]string, 0)
    allMet := true
    
    // Check all conditions
    for _, condition := range s.config.Conditions {
        met, reason := condition.Check(klines, currentPrice)
        reasons = append(reasons, reason)
        if !met {
            allMet = false
        }
    }
    
    if allMet {
        return &Signal{
            Type:       SignalBuy,
            Symbol:     s.config.Symbol,
            EntryPrice: currentPrice,
            StopLoss:   currentPrice * (1 - s.config.StopLoss),
            TakeProfit: currentPrice * (1 + s.config.TakeProfit),
            OrderType:  "LIMIT",
            Side:       "BUY",
            Reason:     fmt.Sprintf("All conditions met: %v", reasons),
            Timestamp:  time.Now(),
        }, nil
    }
    
    return &Signal{Type: SignalNone}, nil
}
```

**Usage:**
```go
strategy := NewMultiConditionStrategy(&MultiConditionConfig{
    Symbol:   "ETHUSDT",
    Interval: "15m",
    Conditions: []ConditionChecker{
        &PriceAboveMACondition{Period: 50},
        &RSIRangeCondition{Period: 14, Min: 50, Max: 70},
        &VolumeSpikeCondition{Multiplier: 1.5, Period: 20},
    },
    StopLoss:   0.02,
    TakeProfit: 0.05,
})
```

---

## 3. Time-Based Conditions

Buy only during specific hours or days.

```go
type TimeBasedConfig struct {
    Symbol      string
    Interval    string
    AllowedHours []int // e.g., [9, 10, 11, 14, 15, 16] for specific hours
    AllowedDays  []time.Weekday
    Timezone     string // e.g., "America/New_York"
    BaseStrategy Strategy // Wrap another strategy
}

type TimeBasedStrategy struct {
    config   *TimeBasedConfig
    location *time.Location
}

func NewTimeBasedStrategy(config *TimeBasedConfig) (*TimeBasedStrategy, error) {
    loc, err := time.LoadLocation(config.Timezone)
    if err != nil {
        return nil, err
    }
    
    return &TimeBasedStrategy{
        config:   config,
        location: loc,
    }, nil
}

func (s *TimeBasedStrategy) Evaluate(klines []binance.Kline, currentPrice float64) (*Signal, error) {
    now := time.Now().In(s.location)
    
    // Check if current hour is allowed
    hourAllowed := false
    for _, hour := range s.config.AllowedHours {
        if now.Hour() == hour {
            hourAllowed = true
            break
        }
    }
    
    if !hourAllowed {
        return &Signal{Type: SignalNone}, nil
    }
    
    // Check if current day is allowed
    dayAllowed := false
    for _, day := range s.config.AllowedDays {
        if now.Weekday() == day {
            dayAllowed = true
            break
        }
    }
    
    if !dayAllowed {
        return &Signal{Type: SignalNone}, nil
    }
    
    // If time conditions met, evaluate base strategy
    return s.config.BaseStrategy.Evaluate(klines, currentPrice)
}
```

**Usage:**
```go
baseStrategy := NewBreakoutStrategy(breakoutConfig)

timeStrategy, _ := NewTimeBasedStrategy(&TimeBasedConfig{
    Symbol:   "BTCUSDT",
    Interval: "15m",
    AllowedHours: []int{9, 10, 11, 13, 14, 15}, // Trading hours
    AllowedDays: []time.Weekday{
        time.Monday,
        time.Tuesday,
        time.Wednesday,
        time.Thursday,
        time.Friday,
    },
    Timezone:     "America/New_York",
    BaseStrategy: baseStrategy,
})
```

---

## 4. Price Pattern Recognition

Detect specific candlestick patterns like hammer, doji, engulfing, etc.

```go
type PatternType string

const (
    PatternHammer     PatternType = "HAMMER"
    PatternDoji       PatternType = "DOJI"
    PatternEngulfing  PatternType = "ENGULFING"
    PatternMorningStar PatternType = "MORNING_STAR"
)

type PatternConfig struct {
    Symbol       string
    Interval     string
    Patterns     []PatternType
    StopLoss     float64
    TakeProfit   float64
}

type PatternStrategy struct {
    config *PatternConfig
}

func (s *PatternStrategy) Evaluate(klines []binance.Kline, currentPrice float64) (*Signal, error) {
    if len(klines) < 3 {
        return &Signal{Type: SignalNone}, nil
    }
    
    for _, pattern := range s.config.Patterns {
        detected, reason := s.detectPattern(klines, pattern)
        if detected {
            return &Signal{
                Type:       SignalBuy,
                Symbol:     s.config.Symbol,
                EntryPrice: currentPrice,
                StopLoss:   currentPrice * (1 - s.config.StopLoss),
                TakeProfit: currentPrice * (1 + s.config.TakeProfit),
                OrderType:  "LIMIT",
                Side:       "BUY",
                Reason:     reason,
                Timestamp:  time.Now(),
            }, nil
        }
    }
    
    return &Signal{Type: SignalNone}, nil
}

func (s *PatternStrategy) detectPattern(klines []binance.Kline, pattern PatternType) (bool, string) {
    lastCandle := klines[len(klines)-2]
    prevCandle := klines[len(klines)-3]
    
    switch pattern {
    case PatternHammer:
        return s.isHammer(lastCandle), "Hammer pattern detected"
        
    case PatternDoji:
        return s.isDoji(lastCandle), "Doji pattern detected"
        
    case PatternEngulfing:
        return s.isBullishEngulfing(prevCandle, lastCandle), "Bullish engulfing pattern"
        
    case PatternMorningStar:
        if len(klines) < 4 {
            return false, ""
        }
        candle1 := klines[len(klines)-4]
        candle2 := klines[len(klines)-3]
        candle3 := klines[len(klines)-2]
        return s.isMorningStar(candle1, candle2, candle3), "Morning star pattern"
    }
    
    return false, ""
}

func (s *PatternStrategy) isHammer(candle binance.Kline) bool {
    body := candle.Close - candle.Open
    if body < 0 {
        body = -body
    }
    
    upperShadow := candle.High - maxFloat(candle.Open, candle.Close)
    lowerShadow := minFloat(candle.Open, candle.Close) - candle.Low
    
    // Hammer: small body, long lower shadow, small upper shadow
    return lowerShadow > body*2 && upperShadow < body*0.5
}

func (s *PatternStrategy) isDoji(candle binance.Kline) bool {
    body := candle.Close - candle.Open
    if body < 0 {
        body = -body
    }
    
    totalRange := candle.High - candle.Low
    
    // Doji: body is very small compared to total range
    return body < totalRange*0.1
}

func (s *PatternStrategy) isBullishEngulfing(prev, current binance.Kline) bool {
    // Previous candle is bearish
    prevBearish := prev.Close < prev.Open
    
    // Current candle is bullish
    currentBullish := current.Close > current.Open
    
    // Current candle engulfs previous
    engulfs := current.Open < prev.Close && current.Close > prev.Open
    
    return prevBearish && currentBullish && engulfs
}

func (s *PatternStrategy) isMorningStar(c1, c2, c3 binance.Kline) bool {
    // First candle: bearish
    bearish := c1.Close < c1.Open
    
    // Second candle: small body (star)
    starBody := c2.Close - c2.Open
    if starBody < 0 {
        starBody = -starBody
    }
    smallStar := starBody < (c1.Open-c1.Close)*0.3
    
    // Third candle: bullish and closes above first candle midpoint
    bullish := c3.Close > c3.Open
    midpoint := (c1.Open + c1.Close) / 2
    aboveMid := c3.Close > midpoint
    
    return bearish && smallStar && bullish && aboveMid
}

func maxFloat(a, b float64) float64 {
    if a > b {
        return a
    }
    return b
}

func minFloat(a, b float64) float64 {
    if a < b {
        return a
    }
    return b
}
```

---

## 5. Volume Analysis Conditions

Advanced volume analysis including volume profile and OBV.

```go
type VolumeAnalysisConfig struct {
    Symbol     string
    Interval   string
    OBVPeriod  int
    VWAPPeriod int
    StopLoss   float64
    TakeProfit float64
}

type VolumeAnalysisStrategy struct {
    config *VolumeAnalysisConfig
}

func (s *VolumeAnalysisStrategy) Evaluate(klines []binance.Kline, currentPrice float64) (*Signal, error) {
    if len(klines) < s.config.OBVPeriod {
        return &Signal{Type: SignalNone}, nil
    }
    
    // Calculate On-Balance Volume
    obv := s.calculateOBV(klines)
    obvMA := s.calculateOBVMA(klines, s.config.OBVPeriod)
    
    // Calculate Volume Weighted Average Price
    vwap := s.calculateVWAP(klines, s.config.VWAPPeriod)
    
    // Conditions:
    // 1. OBV rising (above its MA)
    // 2. Price above VWAP
    // 3. Volume increasing
    
    obvRising := obv > obvMA
    aboveVWAP := currentPrice > vwap
    
    lastVolume := klines[len(klines)-2].Volume
    avgVolume := calculateAverageVolume(klines[:len(klines)-1], 20)
    volumeIncreasing := lastVolume > avgVolume
    
    if obvRising && aboveVWAP && volumeIncreasing {
        return &Signal{
            Type:       SignalBuy,
            Symbol:     s.config.Symbol,
            EntryPrice: currentPrice,
            StopLoss:   currentPrice * (1 - s.config.StopLoss),
            TakeProfit: currentPrice * (1 - s.config.TakeProfit),
            OrderType:  "LIMIT",
            Side:       "BUY",
            Reason:     fmt.Sprintf("Volume analysis: OBV rising, price %.2f > VWAP %.2f", currentPrice, vwap),
            Timestamp:  time.Now(),
        }, nil
    }
    
    return &Signal{Type: SignalNone}, nil
}

func (s *VolumeAnalysisStrategy) calculateOBV(klines []binance.Kline) float64 {
    obv := 0.0
    for i := 1; i < len(klines); i++ {
        if klines[i].Close > klines[i-1].Close {
            obv += klines[i].Volume
        } else if klines[i].Close < klines[i-1].Close {
            obv -= klines[i].Volume
        }
    }
    return obv
}

func (s *VolumeAnalysisStrategy) calculateOBVMA(klines []binance.Kline, period int) float64 {
    if len(klines) < period {
        return 0
    }
    
    sum := 0.0
    for i := len(klines) - period; i < len(klines); i++ {
        obv := 0.0
        for j := 1; j <= i; j++ {
            if klines[j].Close > klines[j-1].Close {
                obv += klines[j].Volume
            } else if klines[j].Close < klines[j-1].Close {
                obv -= klines[j].Volume
            }
        }
        sum += obv
    }
    
    return sum / float64(period)
}

func (s *VolumeAnalysisStrategy) calculateVWAP(klines []binance.Kline, period int) float64 {
    if len(klines) < period {
        period = len(klines)
    }
    
    sumPV := 0.0
    sumV := 0.0
    
    start := len(klines) - period
    for i := start; i < len(klines); i++ {
        typical := (klines[i].High + klines[i].Low + klines[i].Close) / 3
        sumPV += typical * klines[i].Volume
        sumV += klines[i].Volume
    }
    
    if sumV == 0 {
        return 0
    }
    
    return sumPV / sumV
}
```

---

## 6. Combining Multiple Signals

Create a composite strategy that combines signals from multiple sub-strategies.

```go
type CompositeConfig struct {
    Symbol       string
    Interval     string
    Strategies   []Strategy
    MinSignals   int // Minimum number of strategies that must agree
    StopLoss     float64
    TakeProfit   float64
}

type CompositeStrategy struct {
    config *CompositeConfig
}

func (s *CompositeStrategy) Evaluate(klines []binance.Kline, currentPrice float64) (*Signal, error) {
    signals := make([]*Signal, 0)
    reasons := make([]string, 0)
    
    // Evaluate all sub-strategies
    for _, strategy := range s.config.Strategies {
        signal, err := strategy.Evaluate(klines, currentPrice)
        if err != nil {
            continue
        }
        
        if signal.Type != SignalNone {
            signals = append(signals, signal)
            reasons = append(reasons, fmt.Sprintf("%s: %s", strategy.Name(), signal.Reason))
        }
    }
    
    // Check if minimum signals threshold is met
    if len(signals) >= s.config.MinSignals {
        return &Signal{
            Type:       SignalBuy,
            Symbol:     s.config.Symbol,
            EntryPrice: currentPrice,
            StopLoss:   currentPrice * (1 - s.config.StopLoss),
            TakeProfit: currentPrice * (1 + s.config.TakeProfit),
            OrderType:  "LIMIT",
            Side:       "BUY",
            Reason:     fmt.Sprintf("%d strategies agree: %v", len(signals), reasons),
            Timestamp:  time.Now(),
        }, nil
    }
    
    return &Signal{Type: SignalNone}, nil
}
```

**Usage:**
```go
composite := NewCompositeStrategy(&CompositeConfig{
    Symbol:   "BTCUSDT",
    Interval: "15m",
    Strategies: []Strategy{
        NewBreakoutStrategy(breakoutConfig),
        NewRSIStrategy(rsiConfig),
        NewMAStrategy(maConfig),
        NewVolumeStrategy(volumeConfig),
    },
    MinSignals: 3, // Need at least 3 strategies to agree
    StopLoss:   0.02,
    TakeProfit: 0.06,
})
```

---

## Summary

You now have templates for:
- ✅ Basic single-condition strategies
- ✅ Multi-condition strategies with modular conditions
- ✅ Time-based filtering
- ✅ Candlestick pattern recognition
- ✅ Advanced volume analysis
- ✅ Composite strategies combining multiple signals

Mix and match these patterns to create your perfect trading strategy!
