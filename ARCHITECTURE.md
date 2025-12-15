# Binance Trading Bot - Project Structure

## Complete File Structure

```
binance-trading-bot/
│
├── main.go                           # Application entry point
│
├── config/
│   └── config.go                     # Configuration management
│
├── internal/
│   ├── binance/
│   │   └── client.go                 # Binance API wrapper
│   │
│   ├── bot/
│   │   └── bot.go                    # Main trading bot orchestrator
│   │
│   ├── strategy/
│   │   ├── strategy.go               # Base strategies (Breakout, Support)
│   │   └── advanced_strategies.go    # Advanced strategies (RSI, MA, Volume)
│   │
│   ├── screener/
│   │   └── screener.go               # Market screening engine
│   │
│   └── order/
│       └── manager.go                # Advanced order management
│
├── examples/
│   └── comprehensive_examples.go     # Complete usage examples
│
├── config.json.example               # Sample configuration
├── .env.example                      # Environment variables template
├── .gitignore                        # Git ignore rules
├── Dockerfile                        # Docker configuration
├── docker-compose.yml                # Docker Compose setup
├── Makefile                          # Build automation
├── go.mod                            # Go module definition
├── README.md                         # Complete documentation
└── QUICKSTART.md                     # Quick start guide
```

## Component Overview

### Core Components

#### 1. Main Application (`main.go`)
- Entry point
- Strategy registration
- Bot initialization and startup
- Signal handling

#### 2. Configuration (`config/`)
- JSON-based configuration
- Environment variable support
- Flexible parameter management
- Sample config generation

#### 3. Binance Client (`internal/binance/`)
**Features:**
- REST API wrapper
- Kline (candlestick) data fetching
- 24hr ticker data
- Order placement and cancellation
- Price fetching
- HMAC-SHA256 signature authentication

**Key Functions:**
```go
GetKlines(symbol, interval string, limit int) ([]Kline, error)
Get24hrTickers() ([]Ticker24hr, error)
PlaceOrder(params map[string]string) (*OrderResponse, error)
CancelOrder(symbol string, orderId int64) error
GetCurrentPrice(symbol string) (float64, error)
```

#### 4. Trading Bot (`internal/bot/`)
**Responsibilities:**
- Strategy orchestration
- Position management
- Order execution
- Risk management
- Monitoring and logging

**Features:**
- Multiple concurrent strategies
- Dry-run mode
- Position limits
- Automatic stop-loss/take-profit
- Real-time P&L tracking

#### 5. Strategy System (`internal/strategy/`)

**Base Strategies:**
1. **BreakoutStrategy**
   - Triggers when price breaks above previous candle high
   - Configurable entry, stop loss, take profit
   - Volume filtering

2. **SupportStrategy**
   - Triggers when price tests previous candle low
   - Configurable touch distance
   - Support bounce detection

**Advanced Strategies:**
3. **RSIStrategy**
   - Relative Strength Index based
   - Oversold/overbought detection
   - Configurable RSI periods and levels

4. **MovingAverageCrossoverStrategy**
   - Fast/slow MA crossover
   - Bullish and bearish signals
   - Trend following

5. **VolumeSpikeStrategy**
   - Volume anomaly detection
   - Price action confirmation
   - Momentum trading

**Strategy Interface:**
```go
type Strategy interface {
    Name() string
    Evaluate(klines []Kline, currentPrice float64) (*Signal, error)
    GetSymbol() string
    GetInterval() string
}
```

#### 6. Market Screener (`internal/screener/`)
**Features:**
- Scans all crypto pairs
- Volume filtering
- Price change filtering
- Multi-condition analysis
- Automatic signal detection

**Detection:**
- Breakouts
- Support tests
- Volume spikes
- Strong momentum
- High-quality opportunities

**Configuration:**
```go
ScreenerConfig {
    MinVolume:         100000,  // $100k minimum
    MinPriceChange:    2.0,     // 2% minimum
    QuoteCurrency:     "USDT",
    MaxSymbols:        50,
    ScreeningInterval: 60,      // seconds
}
```

#### 7. Order Manager (`internal/order/`)
**Advanced Features:**
- Trailing stop loss
- Time-based modifications
- Price action rules
- Automatic order adjustments

**Modification Rules:**
1. **Time-Based Rules:**
   - Auto-cancel after timeout
   - Scheduled price adjustments
   - Convert to market order

2. **Price Action Rules:**
   - Chase price movements
   - Distance-based triggers
   - Volume-based adjustments
   - Momentum detection

**Example Usage:**
```go
// Enable trailing stop
om.EnableTrailingStop(orderID, 0.01) // 1%

// Add timeout rule
timeRule := TimeBasedRule{
    Name:        "30min_timeout",
    TriggerTime: time.Now().Add(30 * time.Minute),
    Action:      "CANCEL",
}

// Add price action rule
priceRule := PriceActionRule{
    Name:      "chase_price",
    Condition: "PRICE_DISTANCE",
    Threshold: 2.0,
    Action:    "MODIFY_TO_MARKET",
}
```

## Data Flow

```
User/Config
    ↓
Main Application
    ↓
Trading Bot ←→ Binance Client ←→ Binance API
    ↓              ↓
Strategies    Market Screener
    ↓              ↓
Signal Generation
    ↓
Order Manager
    ↓
Order Execution/Modification
    ↓
Position Monitoring
```

## Strategy Evaluation Flow

```
1. Market Screener identifies opportunities
   ↓
2. Bot fetches klines for strategy symbols
   ↓
3. Strategy evaluates conditions
   ↓
4. Signal generated (if conditions met)
   ↓
5. Risk management checks
   ↓
6. Order placement
   ↓
7. Order manager takes control
   ↓
8. Continuous monitoring & modification
   ↓
9. Position closed (TP/SL hit or manual)
```

## Configuration Hierarchy

```
Environment Variables (.env)
    ↓ (if not set)
Configuration File (config.json)
    ↓ (if not found)
Default Values
```

## Key Design Patterns

### 1. Strategy Pattern
Different trading strategies implement common interface:
```go
type Strategy interface {
    Evaluate(klines []Kline, currentPrice float64) (*Signal, error)
}
```

### 2. Factory Pattern
Strategy creation with configuration:
```go
NewBreakoutStrategy(config *BreakoutConfig) *BreakoutStrategy
```

### 3. Observer Pattern
Bot monitors multiple strategies and positions concurrently

### 4. Singleton Pattern
Single Binance client instance shared across components

## Extending the System

### Adding a New Strategy

1. **Create Strategy File:**
```go
// internal/strategy/my_strategy.go

type MyStrategy struct {
    config *MyStrategyConfig
}

func (s *MyStrategy) Evaluate(klines []binance.Kline, currentPrice float64) (*Signal, error) {
    // Your logic here
}
```

2. **Register in main.go:**
```go
myStrat := strategy.NewMyStrategy(config)
bot.RegisterStrategy("my_strategy", myStrat)
```

### Adding New Order Conditions

1. **Define Rule:**
```go
rule := order.PriceActionRule{
    Name:      "my_condition",
    Condition: "MY_CONDITION",
    Threshold: value,
    Action:    "MY_ACTION",
}
```

2. **Add to Order Manager:**
```go
om.AddPriceActionRule(orderID, rule)
```

## Testing Strategy

### Unit Tests
```bash
go test ./internal/strategy/...
go test ./internal/binance/...
```

### Integration Tests
```bash
BINANCE_TESTNET=true go test ./...
```

### Dry Run Testing
```bash
# Set dry_run: true in config
go run main.go
```

## Deployment Options

### 1. Direct Execution
```bash
go build -o trading-bot
./trading-bot
```

### 2. Docker
```bash
docker build -t trading-bot .
docker run --env-file .env trading-bot
```

### 3. Docker Compose
```bash
docker-compose up -d
```

### 4. Systemd Service (Linux)
```ini
[Unit]
Description=Binance Trading Bot
After=network.target

[Service]
Type=simple
User=trader
WorkingDirectory=/opt/trading-bot
ExecStart=/opt/trading-bot/trading-bot
Restart=always

[Install]
WantedBy=multi-user.target
```

## Monitoring & Logging

### Log Levels
- INFO: General operations
- WARN: Important notices
- ERROR: Errors and failures

### Key Metrics to Monitor
- Signals generated
- Orders placed
- Orders modified
- Positions opened/closed
- P&L per position
- API rate limits
- Error rates

## Security Best Practices

1. **Never commit API keys**
2. **Use testnet for development**
3. **Set IP restrictions on API keys**
4. **Enable 2FA on Binance account**
5. **Start with small position sizes**
6. **Monitor bot constantly initially**
7. **Set maximum loss limits**
8. **Regular security audits**

## Performance Optimization

1. **Concurrent Processing:** Multiple strategies run in parallel
2. **Efficient Polling:** Configurable intervals
3. **Smart Caching:** Avoid redundant API calls
4. **Rate Limit Handling:** Built-in rate limiting
5. **Memory Management:** Cleanup of old data

## Common Modifications

### Change Trading Pairs
```go
// main.go
strategy := NewBreakoutStrategy(&BreakoutConfig{
    Symbol: "ADAUSDT", // Change this
})
```

### Adjust Timeframes
```go
strategy := NewBreakoutStrategy(&BreakoutConfig{
    Interval: "5m", // 1m, 5m, 15m, 1h, 4h, 1d
})
```

### Modify Risk Parameters
```go
strategy := NewBreakoutStrategy(&BreakoutConfig{
    StopLoss:   0.01,  // 1%
    TakeProfit: 0.03,  // 3%
})
```

### Screen More Coins
```json
{
  "screener": {
    "max_symbols": 100,
    "min_volume": 50000
  }
}
```

## Troubleshooting Guide

### Issue: No signals generated
**Solutions:**
- Lower min_price_change threshold
- Check market volatility
- Verify strategy conditions
- Increase max_symbols in screener

### Issue: Orders not executing
**Solutions:**
- Check API permissions
- Verify account balance
- Check order size limits
- Review price filters

### Issue: High API usage
**Solutions:**
- Increase polling intervals
- Reduce number of strategies
- Implement better caching
- Use WebSocket for prices

## Future Enhancements

- [ ] WebSocket support for real-time data
- [ ] Machine learning signal generation
- [ ] Backtesting engine
- [ ] Web dashboard
- [ ] Telegram notifications
- [ ] Database for trade history
- [ ] Portfolio rebalancing
- [ ] Multi-exchange support
- [ ] Advanced risk management
- [ ] Genetic algorithm optimization

## Resources

- [Binance API Docs](https://binance-docs.github.io/apidocs/spot/en/)
- [Go Documentation](https://golang.org/doc/)
- [Technical Analysis](https://www.investopedia.com/technical-analysis-4689657)
- [Trading Strategies](https://www.investopedia.com/trading-4427765)

---

**Version:** 1.0.0  
**Last Updated:** November 2024  
**License:** MIT
