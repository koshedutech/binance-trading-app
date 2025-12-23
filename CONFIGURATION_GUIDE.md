# Binance Trading Bot - Configuration Guide

Complete reference for configuring the bot for different trading styles and risk profiles.

## Quick Reference: Configuration Profiles

| Profile | Risk Level | Max Position | Daily Loss | Autopilot | Leverage | Best For |
|---------|-----------|--------------|------------|-----------|----------|----------|
| **Beginner** | Low | $50 | $20 | Manual | 2x | Learning only |
| **Conservative** | Low-Moderate | $100 | $50 | Passive | 3x | Paper trading |
| **Intermediate** | Moderate | $200-300 | $100 | Active | 5-10x | Small live account |
| **Aggressive** | High | $500+ | $200+ | Full Auto | 10-20x | Experienced traders |
| **Ultra-Aggressive** | Very High | $1000+ | $500+ | Full Multi-Mode | 20x | Professional traders |

---

## Part 1: Beginner Profile (Learning Phase)

**Timeframe:** Weeks 1-2
**Capital:** Testnet (paper money)
**Objective:** Understand how bot works, manual trading only

### Complete .env Configuration

```env
# ============================================================================
# BEGINNER PROFILE - LEARNING PHASE
# ============================================================================
# This profile is designed for learning how the bot works
# All trading is MANUAL - autopilot is DISABLED
# Use testnet to practice with paper money

# ============================================================================
# LICENSE & BINANCE API
# ============================================================================
LICENSE_KEY=
BINANCE_API_KEY=testnet_api_key_here
BINANCE_SECRET_KEY=testnet_secret_key_here
BINANCE_TESTNET=true
BINANCE_BASE_URL=https://testnet.binance.vision

# ============================================================================
# DATABASE
# ============================================================================
DB_HOST=localhost
DB_PORT=5432
DB_USER=trading_bot
DB_PASSWORD=secure_password_123
DB_NAME=trading_bot
DB_SSLMODE=disable

# ============================================================================
# WEB SERVER
# ============================================================================
WEB_PORT=8090
WEB_HOST=0.0.0.0

# ============================================================================
# AI/LLM (for signal generation, not used for autopilot yet)
# ============================================================================
AI_ENABLED=true
AI_LLM_ENABLED=true
AI_ML_ENABLED=false       # Disable for learning
AI_SENTIMENT_ENABLED=false # Disable for learning
AI_LLM_PROVIDER=claude
AI_CLAUDE_API_KEY=sk-ant-xxxxxxxxxxxxx
AI_MIN_CONFIDENCE=0.75    # High bar (only strong signals)

# ============================================================================
# FUTURES TRADING - CONSERVATIVE SETTINGS
# ============================================================================
FUTURES_ENABLED=true
FUTURES_TESTNET=true      # Use testnet
FUTURES_DEFAULT_LEVERAGE=2 # Low leverage
FUTURES_DEFAULT_MARGIN_TYPE=CROSSED
FUTURES_POSITION_MODE=ONE_WAY
FUTURES_MAX_LEVERAGE=2    # Prevent accidents

# ============================================================================
# FUTURES AUTOPILOT - DISABLED (Manual Trading Only)
# ============================================================================
FUTURES_AUTOPILOT_ENABLED=false  # MANUAL MODE
FUTURES_AUTOPILOT_RISK_LEVEL=low
FUTURES_AUTOPILOT_MAX_DAILY_LOSS=20.0
FUTURES_AUTOPILOT_MAX_POSITION_SIZE=50.0
FUTURES_AUTOPILOT_MIN_CONFIDENCE=0.75
FUTURES_AUTOPILOT_DEFAULT_LEVERAGE=2
FUTURES_AUTOPILOT_ALLOW_SHORTS=false # No shorts for beginners

# ============================================================================
# SCALPING & CIRCUIT BREAKER
# ============================================================================
SCALPING_ENABLED=false    # Disable for now
CIRCUIT_BREAKER_ENABLED=true
CIRCUIT_MAX_LOSS_PER_HOUR=50.0
CIRCUIT_MAX_CONSECUTIVE_LOSSES=3
CIRCUIT_COOLDOWN_MINUTES=30

# ============================================================================
# LOGGING
# ============================================================================
LOG_LEVEL=INFO
LOG_JSON=false
LOG_INCLUDE_FILE=true
```

### Step-by-Step Usage

1. **Start the bot:**
   ```bash
   docker-compose up -d
   ```

2. **Access dashboard:** http://localhost:8090

3. **Manual workflow:**
   - Look at AI signals (generated automatically)
   - Manually click "Place Order" for positions you like
   - Monitor position P&L
   - Manually close when satisfied with profit or want to cut loss

4. **After 10-20 manual trades:**
   - Review: Which trades were winners? Why?
   - Adjust: Confidence levels, stop-loss, take-profit
   - Move to Conservative Profile

---

## Part 2: Conservative Profile (Validation Phase)

**Timeframe:** Weeks 3-6
**Capital:** $500-1000 testnet, then $500-1000 live
**Objective:** Validate strategy, enable autopilot with safe limits

### Complete .env Configuration

```env
# ============================================================================
# CONSERVATIVE PROFILE - VALIDATION PHASE
# ============================================================================
# Autopilot ENABLED but with very conservative limits
# Start on TESTNET, move to LIVE after 2 weeks of profits

# ============================================================================
# LICENSE & BINANCE API
# ============================================================================
LICENSE_KEY=
# Use TESTNET first:
BINANCE_API_KEY=testnet_api_key_here
BINANCE_SECRET_KEY=testnet_secret_key_here
BINANCE_TESTNET=true
BINANCE_BASE_URL=https://testnet.binance.vision

# After 2 weeks, switch to LIVE:
# BINANCE_API_KEY=production_api_key_here
# BINANCE_SECRET_KEY=production_secret_key_here
# BINANCE_TESTNET=false

# ============================================================================
# DATABASE
# ============================================================================
DB_HOST=localhost
DB_PORT=5432
DB_USER=trading_bot
DB_PASSWORD=change_to_strong_password
DB_NAME=trading_bot
DB_SSLMODE=disable

# ============================================================================
# WEB SERVER & SECURITY
# ============================================================================
WEB_PORT=8090
WEB_HOST=0.0.0.0
AUTH_ENABLED=true         # Enable authentication
JWT_SECRET=very_long_random_secret_minimum_32_chars_here
JWT_EXPIRY=24h

# ============================================================================
# AI/LLM CONFIGURATION
# ============================================================================
AI_ENABLED=true
AI_LLM_ENABLED=true
AI_ML_ENABLED=true        # Enable ML analysis
AI_SENTIMENT_ENABLED=false # Disable for now
AI_LLM_PROVIDER=claude
AI_CLAUDE_API_KEY=sk-ant-xxxxxxxxxxxxx
AI_MIN_CONFIDENCE=0.65    # Medium bar

# ============================================================================
# FUTURES TRADING
# ============================================================================
FUTURES_ENABLED=true
FUTURES_TESTNET=true      # Start with testnet
FUTURES_DEFAULT_LEVERAGE=3
FUTURES_DEFAULT_MARGIN_TYPE=CROSSED
FUTURES_POSITION_MODE=ONE_WAY
FUTURES_MAX_LEVERAGE=5    # Allow up to 5x

# ============================================================================
# FUTURES AUTOPILOT - ENABLED (Conservative)
# ============================================================================
FUTURES_AUTOPILOT_ENABLED=true       # AUTOPILOT ON
FUTURES_AUTOPILOT_RISK_LEVEL=low
FUTURES_AUTOPILOT_MAX_DAILY_LOSS=50.0
FUTURES_AUTOPILOT_MAX_POSITION_SIZE=100.0
FUTURES_AUTOPILOT_MIN_CONFIDENCE=0.65
FUTURES_AUTOPILOT_DEFAULT_LEVERAGE=3
FUTURES_AUTOPILOT_MAX_LEVERAGE=5
FUTURES_AUTOPILOT_ALLOW_SHORTS=false # No shorts
FUTURES_AUTOPILOT_TAKE_PROFIT=1.5    # Quick 1.5% exits
FUTURES_AUTOPILOT_STOP_LOSS=1.0      # 1% stop loss
FUTURES_AUTOPILOT_TRAILING_STOP_ENABLED=true
FUTURES_AUTOPILOT_ALLOW_SHORTS=false

# Limit symbols to start
FUTURES_AUTOPILOT_ALLOWED_SYMBOLS=BTCUSDT,ETHUSDT,BNBUSDT

# ============================================================================
# SCALPING MODE - Conservative
# ============================================================================
SCALPING_ENABLED=true
SCALPING_MIN_PROFIT=0.5   # 0.5% minimum profit
SCALPING_MAX_LOSS=0.2     # 0.2% max loss
SCALPING_MAX_HOLD_SECONDS=300 # Hold for 5 min max

# ============================================================================
# CIRCUIT BREAKER - IMPORTANT SAFETY
# ============================================================================
CIRCUIT_BREAKER_ENABLED=true
CIRCUIT_MAX_LOSS_PER_HOUR=50.0        # Stop if lose $50/hour
CIRCUIT_MAX_CONSECUTIVE_LOSSES=5      # Stop after 5 losses
CIRCUIT_COOLDOWN_MINUTES=30           # Pause for 30 min

# ============================================================================
# LOGGING
# ============================================================================
LOG_LEVEL=INFO
LOG_JSON=true
LOG_INCLUDE_FILE=true
```

### Monitoring During Conservative Phase

**Daily checklist:**
```bash
# Check bot status
curl http://localhost:8090/api/health

# Check recent trades
curl http://localhost:8090/api/futures/orders

# Monitor P&L
curl http://localhost:8090/api/futures/positions

# Check for errors
docker-compose logs trading-bot | grep -i "error" | tail -20

# Monitor resource usage
docker stats
```

**Weekly analysis:**
- Win rate: Should be > 40%
- Average profit per trade: > 0.3%
- Daily drawdown: < max loss limit
- Any API errors or circuit breaker triggers?

**Decision criteria to move to live or next profile:**
- ✅ 2 weeks of consistent trading
- ✅ Win rate > 40%
- ✅ Positive cumulative P&L
- ✅ No API or connection issues
- ✅ Comfortable with bot behavior

---

## Part 3: Intermediate Profile (Scaling Phase)

**Timeframe:** Weeks 6-12
**Capital:** $1000-5000 live
**Objective:** Scale positions, add more symbols, enable aggressive features

### Complete .env Configuration

```env
# ============================================================================
# INTERMEDIATE PROFILE - SCALING PHASE
# ============================================================================
# Running on LIVE with moderate-aggressive settings
# Multiple trading modes and symbols

# ============================================================================
# LICENSE & BINANCE API (PRODUCTION)
# ============================================================================
LICENSE_KEY=PRO-XXXX-XXXX-XXXX  # Get production license
BINANCE_API_KEY=production_api_key_here
BINANCE_SECRET_KEY=production_secret_key_here
BINANCE_TESTNET=false           # LIVE TRADING
BINANCE_BASE_URL=https://api.binance.com

# ============================================================================
# DATABASE
# ============================================================================
DB_HOST=localhost
DB_PORT=5432
DB_USER=trading_bot
DB_PASSWORD=very_strong_password_32_chars_min
DB_NAME=trading_bot
DB_SSLMODE=disable

# ============================================================================
# WEB SERVER & SECURITY
# ============================================================================
WEB_PORT=8090
WEB_HOST=0.0.0.0
AUTH_ENABLED=true
JWT_SECRET=very_long_random_secret_minimum_64_chars_for_production
JWT_EXPIRY=24h

# ============================================================================
# AI/LLM CONFIGURATION
# ============================================================================
AI_ENABLED=true
AI_LLM_ENABLED=true
AI_ML_ENABLED=true
AI_SENTIMENT_ENABLED=true        # Enable sentiment analysis
AI_LLM_PROVIDER=claude
AI_CLAUDE_API_KEY=sk-ant-xxxxxxxxxxxxx
AI_MIN_CONFIDENCE=0.60           # Lower bar

# ============================================================================
# FUTURES TRADING - LIVE
# ============================================================================
FUTURES_ENABLED=true
FUTURES_TESTNET=false
FUTURES_DEFAULT_LEVERAGE=5
FUTURES_DEFAULT_MARGIN_TYPE=CROSSED
FUTURES_POSITION_MODE=HEDGE       # Support both long and short
FUTURES_MAX_LEVERAGE=10

# ============================================================================
# FUTURES AUTOPILOT - ACTIVE
# ============================================================================
FUTURES_AUTOPILOT_ENABLED=true
FUTURES_AUTOPILOT_RISK_LEVEL=moderate
FUTURES_AUTOPILOT_MAX_DAILY_LOSS=100.0
FUTURES_AUTOPILOT_MAX_POSITION_SIZE=250.0
FUTURES_AUTOPILOT_MIN_CONFIDENCE=0.60
FUTURES_AUTOPILOT_DEFAULT_LEVERAGE=5
FUTURES_AUTOPILOT_MAX_LEVERAGE=10
FUTURES_AUTOPILOT_ALLOW_SHORTS=true      # Allow shorts now
FUTURES_AUTOPILOT_TAKE_PROFIT=2.0        # 2% targets
FUTURES_AUTOPILOT_STOP_LOSS=1.5          # 1.5% stop loss
FUTURES_AUTOPILOT_TRAILING_STOP_ENABLED=true
FUTURES_AUTOPILOT_TRAILING_STOP_PERCENT=0.5

# More symbols for diversification
FUTURES_AUTOPILOT_ALLOWED_SYMBOLS=BTCUSDT,ETHUSDT,BNBUSDT,ADAUSDT,LINKUSDT,XRPUSDT

# ============================================================================
# SCALPING MODE - ENABLED
# ============================================================================
SCALPING_ENABLED=true
SCALPING_MIN_PROFIT=0.75         # 0.75% minimum profit
SCALPING_MAX_LOSS=0.3            # 0.3% max loss
SCALPING_MAX_HOLD_SECONDS=600    # Hold for 10 min max

# ============================================================================
# CIRCUIT BREAKER - SAFETY NET
# ============================================================================
CIRCUIT_BREAKER_ENABLED=true
CIRCUIT_MAX_LOSS_PER_HOUR=100.0
CIRCUIT_MAX_CONSECUTIVE_LOSSES=8
CIRCUIT_COOLDOWN_MINUTES=60      # 1 hour pause

# ============================================================================
# LOGGING & MONITORING
# ============================================================================
LOG_LEVEL=INFO
LOG_JSON=true
LOG_INCLUDE_FILE=true
NOTIFICATIONS_ENABLED=true
TELEGRAM_ENABLED=true
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
TELEGRAM_CHAT_ID=your_telegram_chat_id
```

### Per-Mode Capital Allocation

```json
{
  "mode_allocation": {
    "scalp_percent": 40,
    "swing_percent": 40,
    "position_percent": 20,

    "max_scalp_positions": 4,
    "max_swing_positions": 3,
    "max_position_positions": 2,

    "max_scalp_usd_per_position": 300,
    "max_swing_usd_per_position": 400,
    "max_position_usd_per_position": 600
  }
}
```

### Monitoring During Intermediate Phase

**Daily:**
- Check P&L across all modes
- Verify no circuit breaker triggers
- Monitor Telegram alerts
- Check logs for errors

**Weekly:**
- Win rate analysis by symbol
- Win rate analysis by trading mode
- Profitability by AI confidence levels
- Adjust confidence thresholds if needed

**Monthly:**
- Portfolio review
- Risk/reward ratio analysis
- Compare results to settings
- Plan next scaling step

---

## Part 4: Advanced Profile (Full Automation)

**Timeframe:** 3+ months
**Capital:** $5000+
**Objective:** Multi-mode trading, maximum automation, aggressive growth

### Complete .env Configuration

```env
# ============================================================================
# ADVANCED PROFILE - FULL AUTOMATION
# ============================================================================
# Aggressive settings with ALL features enabled
# For experienced traders only

LICENSE_KEY=ENTERPRISE-XXXX-XXXX-XXXX

# Production API Keys
BINANCE_API_KEY=production_api_key_here
BINANCE_SECRET_KEY=production_secret_key_here
BINANCE_TESTNET=false
BINANCE_BASE_URL=https://api.binance.com

# ============================================================================
# DATABASE
# ============================================================================
DB_HOST=localhost
DB_PORT=5432
DB_USER=trading_bot
DB_PASSWORD=extremely_secure_password_64_chars_minimum
DB_NAME=trading_bot
DB_SSLMODE=require  # Require SSL for security

# ============================================================================
# WEB SERVER & SECURITY
# ============================================================================
WEB_PORT=8090
WEB_HOST=127.0.0.1  # Only localhost in production with nginx proxy
AUTH_ENABLED=true
JWT_SECRET=extremely_long_random_secret_64_chars_minimum
JWT_EXPIRY=12h      # Shorter expiry for security
REQUIRE_EMAIL_VERIFICATION=true

# ============================================================================
# AI/LLM - FULL STACK
# ============================================================================
AI_ENABLED=true
AI_LLM_ENABLED=true
AI_ML_ENABLED=true
AI_SENTIMENT_ENABLED=true
AI_LLM_PROVIDER=claude
AI_CLAUDE_API_KEY=sk-ant-xxxxxxxxxxxxx
AI_MIN_CONFIDENCE=0.40  # Low bar - trust the algorithm

# ============================================================================
# FUTURES - AGGRESSIVE
# ============================================================================
FUTURES_ENABLED=true
FUTURES_TESTNET=false
FUTURES_DEFAULT_LEVERAGE=10
FUTURES_DEFAULT_MARGIN_TYPE=ISOLATED  # Isolated for risk management
FUTURES_POSITION_MODE=HEDGE            # Both long and short
FUTURES_MAX_LEVERAGE=20

# ============================================================================
# FUTURES AUTOPILOT - AGGRESSIVE
# ============================================================================
FUTURES_AUTOPILOT_ENABLED=true
FUTURES_AUTOPILOT_RISK_LEVEL=aggressive
FUTURES_AUTOPILOT_MAX_DAILY_LOSS=300.0
FUTURES_AUTOPILOT_MAX_POSITION_SIZE=500.0
FUTURES_AUTOPILOT_MIN_CONFIDENCE=0.40
FUTURES_AUTOPILOT_DEFAULT_LEVERAGE=10
FUTURES_AUTOPILOT_MAX_LEVERAGE=20
FUTURES_AUTOPILOT_ALLOW_SHORTS=true
FUTURES_AUTOPILOT_TAKE_PROFIT=2.5     # 2.5% targets
FUTURES_AUTOPILOT_STOP_LOSS=1.2       # 1.2% stop loss
FUTURES_AUTOPILOT_TRAILING_STOP_ENABLED=true
FUTURES_AUTOPILOT_TRAILING_STOP_PERCENT=0.3

# All symbols
FUTURES_AUTOPILOT_ALLOWED_SYMBOLS=BTCUSDT,ETHUSDT,BNBUSDT,ADAUSDT,LINKUSDT,XRPUSDT,DOGEUSDT,MATICUSDT,LTCUSDT,AVAXUSDT

# ============================================================================
# SCALPING MODE - AGGRESSIVE
# ============================================================================
SCALPING_ENABLED=true
SCALPING_MIN_PROFIT=0.3            # Only 0.3%
SCALPING_MAX_LOSS=0.2              # Very tight stops
SCALPING_MAX_HOLD_SECONDS=60       # Hold 1 minute max

# ============================================================================
# ULTRA-FAST SCALPING (if implemented)
# ============================================================================
ULTRA_FAST_ENABLED=true
ULTRA_FAST_SCAN_INTERVAL=5000      # Scan every 5 seconds
ULTRA_FAST_MONITOR_INTERVAL=500    # Check exits every 500ms
ULTRA_FAST_MAX_POSITIONS=3
ULTRA_FAST_MAX_USD_PER_POS=200
ULTRA_FAST_MIN_CONFIDENCE=50       # AI confidence 50%

# ============================================================================
# CIRCUIT BREAKER - SAFETY FIRST
# ============================================================================
CIRCUIT_BREAKER_ENABLED=true
CIRCUIT_MAX_LOSS_PER_HOUR=200.0
CIRCUIT_MAX_CONSECUTIVE_LOSSES=12
CIRCUIT_COOLDOWN_MINUTES=120       # 2 hour pause

# ============================================================================
# BIG CANDLE DETECTION (for momentum plays)
# ============================================================================
BIG_CANDLE_ENABLED=true
BIG_CANDLE_SIZE_MULTIPLIER=1.5
BIG_CANDLE_VOLUME_CONFIRMATION=true

# ============================================================================
# NOTIFICATIONS - FULL ALERTS
# ============================================================================
NOTIFICATIONS_ENABLED=true
TELEGRAM_ENABLED=true
TELEGRAM_BOT_TOKEN=your_token
TELEGRAM_CHAT_ID=your_chat_id
DISCORD_ENABLED=true
DISCORD_WEBHOOK_URL=your_webhook_url

# ============================================================================
# LOGGING - DETAILED
# ============================================================================
LOG_LEVEL=INFO
LOG_JSON=true
LOG_INCLUDE_FILE=true
LOG_OUTPUT=/var/log/trading-bot/app.log
```

### Advanced Settings - Per-Mode Allocation

```json
{
  "mode_allocation": {
    "ultra_fast_scalp_percent": 30,
    "scalp_percent": 35,
    "swing_percent": 25,
    "position_percent": 10,

    "max_ultra_fast_positions": 5,
    "max_scalp_positions": 5,
    "max_swing_positions": 4,
    "max_position_positions": 2,

    "max_ultra_fast_usd_per_position": 300,
    "max_scalp_usd_per_position": 400,
    "max_swing_usd_per_position": 500,
    "max_position_usd_per_position": 800,

    "allow_dynamic_rebalance": true,
    "rebalance_threshold_pct": 15
  },

  "mode_safety_ultra_fast": {
    "max_trades_per_minute": 5,
    "max_trades_per_hour": 30,
    "max_trades_per_day": 100,
    "enable_profit_monitor": true,
    "profit_window_minutes": 10,
    "max_loss_percent_in_window": -1.5,
    "enable_win_rate_monitor": true,
    "win_rate_sample_size": 15,
    "min_win_rate_threshold": 55.0
  }
}
```

---

## Scaling Checklist

### Moving from Conservative to Intermediate
- [ ] ✅ 2+ weeks of paper trading completed
- [ ] ✅ Win rate > 40% consistently
- [ ] ✅ No circuit breaker triggers in past week
- [ ] ✅ Comfortable with bot automation
- [ ] ✅ Have emergency plan to stop trading
- [ ] ✅ Understand all configuration options

### Moving from Intermediate to Advanced
- [ ] ✅ 4+ weeks of live trading completed
- [ ] ✅ Win rate > 50%
- [ ] ✅ Consistent positive P&L
- [ ] ✅ Survived at least one drawdown period
- [ ] ✅ Made manual adjustments and saw results
- [ ] ✅ Ready for higher leverage and more symbols

---

## Configuration Parameter Reference

### Risk Parameters

| Parameter | Beginner | Conservative | Intermediate | Advanced |
|-----------|----------|--------------|--------------|----------|
| `FUTURES_DEFAULT_LEVERAGE` | 2x | 3x | 5x | 10x |
| `FUTURES_AUTOPILOT_MAX_POSITION_SIZE` | $50 | $100 | $250 | $500 |
| `FUTURES_AUTOPILOT_MAX_DAILY_LOSS` | $20 | $50 | $100 | $300 |
| `CIRCUIT_MAX_LOSS_PER_HOUR` | $50 | $50 | $100 | $200 |
| `CIRCUIT_MAX_CONSECUTIVE_LOSSES` | 3 | 5 | 8 | 12 |
| `CIRCUIT_COOLDOWN_MINUTES` | 30 | 30 | 60 | 120 |

### Signal Quality Parameters

| Parameter | Conservative | Intermediate | Advanced |
|-----------|--------------|--------------|----------|
| `AI_MIN_CONFIDENCE` | 0.75 | 0.60 | 0.40 |
| `FUTURES_AUTOPILOT_MIN_CONFIDENCE` | 0.65 | 0.60 | 0.40 |
| Confidence Rule | Only strong signals | Mix of strong and medium | Most signals accepted |

### Position Exit Parameters

| Parameter | Conservative | Intermediate | Advanced |
|-----------|--------------|--------------|----------|
| `FUTURES_AUTOPILOT_TAKE_PROFIT` | 1.5% | 2.0% | 2.5% |
| `FUTURES_AUTOPILOT_STOP_LOSS` | 1.0% | 1.5% | 1.2% |
| `SCALPING_MIN_PROFIT` | 0.5% | 0.75% | 0.3% |
| `SCALPING_MAX_LOSS` | 0.2% | 0.3% | 0.2% |

---

## Troubleshooting Configuration Issues

### Q: Bot runs but generates no trades
```env
# Check these in order:
1. FUTURES_AUTOPILOT_ENABLED=true                    # Is autopilot on?
2. AI_CLAUDE_API_KEY=not_empty                       # Do you have API key?
3. FUTURES_AUTOPILOT_ALLOWED_SYMBOLS=something       # Are symbols configured?
4. FUTURES_AUTOPILOT_MIN_CONFIDENCE < 0.75           # Is bar too high?
5. Check logs: "confidence: 0.X" messages
```

### Q: Too many trades, bot is overtrading
```env
# Reduce trade frequency:
1. FUTURES_AUTOPILOT_MIN_CONFIDENCE=higher           # Increase bar
2. CIRCUIT_MAX_CONSECUTIVE_LOSSES=lower              # Stop sooner
3. SCALPING_MIN_PROFIT=higher                        # Only big moves
4. Reduce FUTURES_AUTOPILOT_ALLOWED_SYMBOLS          # Fewer symbols
```

### Q: Losses are high, want to reduce risk
```env
# Immediately:
1. FUTURES_AUTOPILOT_MAX_DAILY_LOSS=lower            # Lower daily limit
2. FUTURES_AUTOPILOT_MAX_POSITION_SIZE=lower         # Smaller positions
3. FUTURES_DEFAULT_LEVERAGE=lower                    # Reduce leverage

# Short term:
4. CIRCUIT_BREAKER_ENABLED=true                      # Make sure enabled
5. Increase AI_MIN_CONFIDENCE                        # Only strong signals

# Medium term:
6. Review P&L by symbol - disable worst performers
7. Review settings after 1 week of changes
```

---

## Environment Variable Validation

**Before running bot, verify:**
```bash
#!/bin/bash
# config-check.sh

echo "=== Configuration Validation ==="

# Check required fields
[[ -z "$BINANCE_API_KEY" ]] && echo "❌ BINANCE_API_KEY is empty" || echo "✅ BINANCE_API_KEY set"
[[ -z "$BINANCE_SECRET_KEY" ]] && echo "❌ BINANCE_SECRET_KEY is empty" || echo "✅ BINANCE_SECRET_KEY set"
[[ -z "$AI_CLAUDE_API_KEY" ]] && echo "❌ AI_CLAUDE_API_KEY is empty" || echo "✅ AI_CLAUDE_API_KEY set"

# Check incompatible settings
if [[ "$BINANCE_TESTNET" == "false" && "$TRADING_DRY_RUN" == "true" ]]; then
  echo "❌ ERROR: TESTNET=false but DRY_RUN=true (contradictory)"
fi

# Check leverage limits
if (( $(echo "$FUTURES_DEFAULT_LEVERAGE > $FUTURES_MAX_LEVERAGE" | bc -l) )); then
  echo "❌ ERROR: DEFAULT_LEVERAGE > MAX_LEVERAGE"
fi

echo "=== Validation Complete ==="
```

---

**Remember:** Configuration is critical. Spend time getting it right before enabling autopilot with real money.
