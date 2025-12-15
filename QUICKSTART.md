# Quick Start Guide

Get your Binance trading bot running in 5 minutes!

## Prerequisites

- Go 1.21+ installed
- Binance account (or Testnet account)

## Step 1: Get API Keys (2 minutes)

### For Testing (Recommended)

1. Visit [Binance Testnet](https://testnet.binance.vision/)
2. Click "Generate HMAC_SHA256 Key"
3. Copy your API Key and Secret Key
4. Note: Testnet uses fake money - perfect for learning!

### For Production (Use Real Money)

1. Login to [Binance.com](https://www.binance.com)
2. Go to Account → API Management
3. Create API Key with "Enable Spot & Margin Trading"
4. ⚠️ Save your Secret Key immediately (shown only once)

## Step 2: Clone and Setup (1 minute)

```bash
# Clone the repository
git clone <your-repo-url>
cd binance-trading-bot

# Install dependencies
go mod download

# Create configuration
cp .env.example .env
```

## Step 3: Configure (1 minute)

Edit `.env` file:

```bash
# For Testnet (Safe - Fake Money)
BINANCE_API_KEY=your_testnet_api_key_here
BINANCE_SECRET_KEY=your_testnet_secret_key_here
BINANCE_BASE_URL=https://testnet.binance.vision
BINANCE_TESTNET=true
```

## Step 4: Run the Bot (1 minute)

```bash
# Start the bot in dry-run mode
go run main.go
```

You should see:
```
2024-11-03 10:15:00 Starting Binance Trading Bot...
2024-11-03 10:15:00 Dry run mode: true
2024-11-03 10:15:00 Trading Bot started
2024-11-03 10:15:01 Screener started
2024-11-03 10:15:02 Market scan completed. Found 15 opportunities
```

## What Happens Next?

The bot will:

1. **Screen the Market** - Scans 100+ crypto pairs every minute
2. **Find Opportunities** - Identifies breakouts, support tests, volume spikes
3. **Generate Signals** - Evaluates strategies on promising coins
4. **Simulate Trades** - Shows what it would do (in dry-run mode)

## Example Output

```
=== TOP OPPORTUNITIES ===
1. SOLUSDT - Price: 145.32 | Change: +5.45% | Volume: $245M | Signals: [BREAKOUT, HIGH_VOLUME]
2. AVAXUSDT - Price: 38.21 | Change: +3.12% | Volume: $123M | Signals: [SUPPORT]
3. LINKUSDT - Price: 15.67 | Change: +2.87% | Volume: $89M | Signals: [STRONG_MOMENTUM]
========================

Signal detected: breakout_high - SOLUSDT - Price 145.32 broke above last candle high 144.80
DRY RUN - Would place BUY order for SOLUSDT at 145.3200
DRY RUN - Stop Loss: 142.41 | Take Profit: 152.59
```

## Understanding the Strategies

### Breakout Strategy
Buys when price breaks **above** previous candle's high
- Entry: Current price
- Stop Loss: 2% below entry
- Take Profit: 5% above entry

### Support Strategy
Buys when price tests **near** previous candle's low
- Entry: Current price
- Stop Loss: 2% below entry
- Take Profit: 5% above entry

## Customization Quick Tips

### Change Trading Pairs

Edit `main.go`:
```go
breakoutStrategy := strategy.NewBreakoutStrategy(&strategy.BreakoutConfig{
    Symbol: "SOLUSDT",  // Change to any pair: ETHUSDT, BNBUSDT, etc.
    Interval: "15m",    // Change timeframe: 5m, 15m, 1h, 4h
    // ...
})
```

### Adjust Risk Settings

Edit `config.json`:
```json
{
  "trading": {
    "max_open_positions": 3,     // Max concurrent trades
    "max_risk_per_trade": 1.0,   // 1% risk per trade
    "dry_run": true              // Keep true until confident!
  }
}
```

### Change Screening Filters

Edit `config.json`:
```json
{
  "screener": {
    "min_volume": 500000,        // Only coins with $500k+ volume
    "min_price_change": 3.0,     // Only coins with +3% change
    "screening_interval": 120    // Scan every 2 minutes
  }
}
```

## Common Commands

```bash
# Run bot
go run main.go

# Build binary
make build

# Run tests
make test

# Clean up
make clean

# Generate config
make config
```

## Safety Checklist

Before going live with real money:

- ✅ Test on Testnet for at least 1 week
- ✅ Understand each strategy
- ✅ Set appropriate stop losses
- ✅ Start with small position sizes
- ✅ Monitor the bot actively
- ✅ Have a backup plan
- ✅ Never invest more than you can afford to lose

## Next Steps

1. **Let it run** - Observe for 24 hours in dry-run mode
2. **Check signals** - See what trades it wants to make
3. **Adjust strategies** - Fine-tune parameters
4. **Read full README** - Learn about advanced features
5. **Create custom strategies** - Build your own logic

## Troubleshooting

### "Invalid API key"
- Double-check your API key in .env
- Make sure testnet keys are used with testnet URL

### "No signals detected"
- Market might be quiet - wait longer
- Lower `min_price_change` in config
- Try different symbols/timeframes

### Bot stops unexpectedly
- Check console for errors
- Verify internet connection
- Ensure API keys have correct permissions

## Getting Help

- 📖 Read the full [README.md](README.md)
- 🐛 Check [GitHub Issues](https://github.com/yourrepo/issues)
- 💬 Join our Discord/Telegram (if available)
- 📚 [Binance API Docs](https://binance-docs.github.io/apidocs/spot/en/)

## Important Reminders

- 🧪 **Always test on Testnet first**
- 💰 **Start small with real money**
- 📊 **Monitor actively**
- 🚨 **Trading is risky**
- 🧠 **Learn continuously**

---

**Happy Trading! 🚀**

Remember: This bot is a tool. Success requires good strategies, risk management, and continuous learning!
