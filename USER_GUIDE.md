# Binance Trading Bot - User Guide

A practical guide for setting up and running the Binance Trading Bot for different user types and trading styles.

## Quick Navigation

- [New to Trading? Start Here](#new-to-trading--start-here)
- [5-Minute Quick Start](#5-minute-quick-start)
- [Configuration Profiles](#configuration-profiles)
- [Common Setup Mistakes](#common-setup-mistakes)
- [Security Checklist](#security-checklist)
- [Step-by-Step API Setup](#step-by-step-api-setup)
- [First Trade Checklist](#first-trade-checklist)

---

## New to Trading? Start Here

**This bot is designed for experienced traders.** Before using this bot, you should:

1. ✅ Understand what futures trading is (leverage, short positions, liquidation)
2. ✅ Know how to read candlestick charts
3. ✅ Understand risk management and position sizing
4. ✅ Have experience with Binance platform
5. ✅ Start with paper trading (testnet) first

**If any of the above is unclear**, spend a few weeks on Binance testnet learning basics before using this bot.

---

## 5-Minute Quick Start

**Goal**: Get the bot running on testnet in 5 minutes

### 1. Prepare Your System (1 minute)

```bash
# Install Docker (if not already installed)
# Windows: Download from https://www.docker.com/products/docker-desktop
# Mac: brew install docker
# Linux: apt-get install docker.io docker-compose

# Clone or download the bot
git clone https://github.com/your-repo/binance-trading-bot.git
cd binance-trading-bot
```

### 2. Configure Environment (2 minutes)

```bash
# Copy the example configuration
cp .env.example .env

# Edit with your Binance testnet API keys
# macOS/Linux: nano .env
# Windows: Use Notepad or VS Code
```

**Minimum required settings:**
```env
BINANCE_API_KEY=your_testnet_api_key
BINANCE_SECRET_KEY=your_testnet_secret_key
BINANCE_TESTNET=true
AI_CLAUDE_API_KEY=your_claude_api_key
LICENSE_KEY=  # Leave empty for 7-day trial
```

### 3. Start the Bot (1 minute)

```bash
# Start all services (database, backend, frontend)
docker-compose up -d

# Check that it's running
docker-compose logs trading-bot

# Expected output: "Server listening on http://0.0.0.0:8090"
```

### 4. Access Dashboard (1 minute)

1. Open browser: **http://localhost:8090**
2. You should see the dashboard with balance and trading UI
3. Verify testnet mode is enabled (should say "Testnet" in UI)

**Done!** The bot is now running on testnet. Proceed to configuration.

---

## Configuration Profiles

Choose the profile that matches your trading experience and risk tolerance.

### Profile 1: Beginner (Ultra-Conservative)

**Use case**: Learning the bot, paper trading only
- Risk level: `low`
- Max daily loss: $10 USD
- Position size: $50 max per position
- Autopilot: Disabled (manual trading only)
- Leverage: 2x

```env
FUTURES_AUTOPILOT_ENABLED=false
FUTURES_AUTOPILOT_RISK_LEVEL=low
FUTURES_AUTOPILOT_MAX_DAILY_LOSS=10.0
FUTURES_AUTOPILOT_MAX_POSITION_SIZE=50.0
FUTURES_DEFAULT_LEVERAGE=2
BINANCE_TESTNET=true
```

**What this means:**
- You manually click "Place Order" in the UI
- Bot calculates profit/loss but doesn't auto-trade
- Daily losses capped at $10
- Each position max $50
- Easy to reverse positions if needed

**Next step**: Paper trade for 2 weeks, then move to Profile 2

---

### Profile 2: Intermediate (Conservative)

**Use case**: Scalping mode, moderate positions, live trading ready
- Risk level: `moderate`
- Max daily loss: $50 USD
- Position size: $200 max per position
- Scalping mode: Enabled (quick 0.5-1% profits)
- Leverage: 5x
- Confidence threshold: 65%

```env
FUTURES_AUTOPILOT_ENABLED=true
FUTURES_AUTOPILOT_RISK_LEVEL=moderate
FUTURES_AUTOPILOT_MAX_DAILY_LOSS=50.0
FUTURES_AUTOPILOT_MAX_POSITION_SIZE=200.0
FUTURES_AUTOPILOT_MIN_CONFIDENCE=0.65
FUTURES_DEFAULT_LEVERAGE=5
SCALPING_ENABLED=true
SCALPING_MIN_PROFIT=0.5
BINANCE_TESTNET=true  # Change to false when ready for live
```

**What this means:**
- Bot auto-trades based on AI signals
- Quick exits for 0.5-1% profits (scalping)
- Positions sized conservatively
- Won't trade unless AI confidence > 65%
- Daily loss limit protects from bad days

**Timeline:**
1. Paper trade for 2 weeks on testnet
2. Move to live trading with $500-1000 capital
3. Monitor for 1 week before increasing position size

---

### Profile 3: Advanced (Aggressive)

**Use case**: Full autopilot with multiple trading modes, experienced traders only
- Risk level: `aggressive`
- Max daily loss: $200 USD
- Position size: $500 max per position
- Multi-mode trading: Ultra-fast, Scalp, Swing, Position modes
- Leverage: 10-20x (depending on mode)
- Confidence threshold: 35%

```env
FUTURES_AUTOPILOT_ENABLED=true
FUTURES_AUTOPILOT_RISK_LEVEL=aggressive
FUTURES_AUTOPILOT_MAX_DAILY_LOSS=200.0
FUTURES_AUTOPILOT_MAX_POSITION_SIZE=500.0
FUTURES_AUTOPILOT_MIN_CONFIDENCE=0.35
FUTURES_DEFAULT_LEVERAGE=10
FUTURES_AUTOPILOT_ALLOW_SHORTS=true
SCALPING_ENABLED=true
BINANCE_TESTNET=false  # Live trading
```

**Additional settings for aggressive mode:**
```env
# Mode allocation
# Ultra-Fast Scalp: 30%, Scalp: 30%, Swing: 25%, Position: 15%

# Circuit breaker (safety net)
CIRCUIT_BREAKER_ENABLED=true
CIRCUIT_MAX_LOSS_PER_HOUR=100.0
CIRCUIT_MAX_CONSECUTIVE_LOSSES=10
```

**Prerequisites:**
- ✅ 3+ months of profitable paper trading
- ✅ Understand circuit breaker and safety limits
- ✅ Experience with leverage and liquidations
- ✅ Can afford to lose $200/day without emotional impact

---

## Common Setup Mistakes

### ❌ Mistake 1: Wrong API Keys (Testnet vs Production)

**What happens:**
- API key is from production, but config says `BINANCE_TESTNET=true`
- You get `API Error: -2015 Invalid API-key`

**How to fix:**
1. Testnet keys: https://testnet.binance.vision/
2. Production keys: https://www.binance.com/api/settings/api-management
3. Don't mix them!

**How to verify:**
```bash
# Check if you have the right keys
curl -H "X-MBX-APIKEY: YOUR_KEY" "https://testnet.binance.vision/api/v3/account"
# Should return your account info if testnet key is correct

curl -H "X-MBX-APIKEY: YOUR_KEY" "https://api.binance.com/api/v3/account"
# Should return your account info if production key is correct
```

---

### ❌ Mistake 2: IP Not Whitelisted

**What happens:**
- API requests fail immediately: `API Error: -2015 Invalid API-key`
- Actually, it's an IP whitelist issue, not the key

**How to fix:**
1. Find your server IP:
   ```bash
   curl ifconfig.me  # Linux/Mac
   # Or: Search "what is my IP" in Google (Windows)
   ```
2. Go to Binance: Settings → API Management → Edit restrictions
3. Enable "Restrict to trusted IPs only"
4. Add your server IP
5. Wait 2-5 minutes for Binance to apply changes

**For dynamic IPs (home internet):**
- Use a VPS with static IP instead
- Or use Binance's "IP Access Restriction" with wildcard (not recommended for security)

---

### ❌ Mistake 3: Database Not Running

**What happens:**
- Bot starts but dashboard says "Connection Failed"
- Logs show: `error connecting to database`

**How to fix:**
```bash
# Check if PostgreSQL is running
docker-compose ps

# Should show:
# NAME                STATUS              PORTS
# postgres            Up 2 seconds        5432/tcp
# redis               Up 2 seconds        6379/tcp
# trading-bot         Up 2 seconds        0.0.0.0:8090->8090/tcp

# If postgres is not running:
docker-compose restart postgres
```

---

### ❌ Mistake 4: AI API Key Missing or Invalid

**What happens:**
- Bot runs but no trade signals are generated
- Logs show: `LLM error: unauthorized`

**How to fix:**
1. Verify you have the correct API key
2. Check provider URL is correct:
   - Claude: https://api.anthropic.com
   - OpenAI: https://api.openai.com
   - DeepSeek: https://api.deepseek.com
3. Verify key has correct permissions (can make API requests)

**Quick test:**
```bash
# Test Claude API
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: YOUR_CLAUDE_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"claude-opus-4-1","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}'

# Should return a response, not an error
```

---

### ❌ Mistake 5: Testnet→Live Transition Too Fast

**What happens:**
- Paper trading looks great (paper money risk-free)
- Switch to live, and strategy doesn't work (market conditions differ)
- Lost real money

**Best practice:**
1. Paper trade for minimum 2-4 weeks
2. See profit consistently
3. Paper trade through a complete market cycle (up and down)
4. Start live with 10% of intended capital
5. Scale up slowly after 2 weeks of live profits

---

## Security Checklist

**Before going live with real money, complete this checklist:**

### API Key Security

- [ ] ✅ API key has **"Spot & Margin Trading"** enabled (not "Withdrawal")
- [ ] ✅ API key has **"Futures"** enabled
- [ ] ✅ API key has **IP whitelist** enabled with your server's static IP
- [ ] ✅ **"Enable Withdrawals"** is DISABLED on the API key
- [ ] ✅ API key is in `.env` file (NOT in source code)
- [ ] ✅ `.env` file is listed in `.gitignore` (don't commit it)
- [ ] ✅ Verify key every month (rotate every 3 months in production)

```bash
# Verify API key permissions on server
curl -H "X-MBX-APIKEY: YOUR_KEY" "https://api.binance.com/api/v3/account"
# Should work without errors
```

### Server Security

- [ ] ✅ Database password is strong (20+ characters, mixed case, numbers, symbols)
- [ ] ✅ Database is NOT accessible from internet (no port forwarding)
- [ ] ✅ Web dashboard is behind authentication (enable AUTH_ENABLED=true for production)
- [ ] ✅ Use HTTPS for dashboard (not HTTP) in production
- [ ] ✅ Firewall only opens ports: 22 (SSH), 443 (HTTPS), 80 (HTTP redirect)
- [ ] ✅ Regular backups of PostgreSQL database (daily minimum)

### Trading Security

- [ ] ✅ Start with testnet mode (`BINANCE_TESTNET=true`)
- [ ] ✅ Autopilot is DISABLED initially (`FUTURES_AUTOPILOT_ENABLED=false`)
- [ ] ✅ Position size is small first ($50-100 max per position)
- [ ] ✅ Daily loss limit is set conservatively (`FUTURES_AUTOPILOT_MAX_DAILY_LOSS`)
- [ ] ✅ Circuit breaker is ENABLED (`CIRCUIT_BREAKER_ENABLED=true`)
- [ ] ✅ Have a panic button ready (restart bot, or close all positions manually)

### Operational Security

- [ ] ✅ Monitor logs daily for unusual activity
- [ ] ✅ Check bot health endpoint every morning
- [ ] ✅ Have a rollback plan (how to stop trading quickly if needed)
- [ ] ✅ Monitor P&L daily (don't just set and forget)
- [ ] ✅ Keep documentation of all settings changes

---

## Step-by-Step API Setup

### For Binance Production Trading

#### Step 1: Create API Key (5 minutes)

1. Go to: https://www.binance.com/en/account/settings/api-management
2. Click "Create API"
3. Name it: "Trading Bot - [Your Name] - [Date]"
4. Click "Create"
5. Complete security verification (phone/email)

#### Step 2: Configure Permissions (5 minutes)

1. Under your API key, click "Edit restrictions"
2. **Enable these:**
   - ✅ Enable Reading (allow bot to check balance)
   - ✅ Enable Spot & Margin Trading (for spot mode)
   - ✅ Enable Futures (for futures mode)

3. **Disable these:**
   - ❌ Enable Withdrawals (critical for security!)
   - ❌ Enable IP Restriction (until you have static IP)

4. Click "Save"

#### Step 3: Set IP Whitelist (5 minutes)

1. Find your server's IP:
   ```bash
   # On the server:
   curl ifconfig.me

   # Or from home (if server is on your home network):
   curl ifconfig.me  # Get public IP
   ```

2. On Binance API page:
   - Find "Restrict access to trusted IPs only"
   - Click "Add IP"
   - Enter your server's IP (e.g., 203.0.113.45)
   - Click "Add"

3. **Test connection:**
   ```bash
   # Wait 5 minutes for Binance to apply changes
   curl -H "X-MBX-APIKEY: YOUR_KEY" "https://api.binance.com/api/v3/account"
   ```
   Should return your account info without error

#### Step 4: Add to Bot Configuration (2 minutes)

```bash
# Edit .env file
nano .env

# Find these lines:
BINANCE_API_KEY=your_api_key_here
BINANCE_SECRET_KEY=your_secret_key_here

# Replace with your production keys:
BINANCE_API_KEY=zmxxxxxxxxxxxxxxxxxxx
BINANCE_SECRET_KEY=xxxxxxxxxxxxxxxxxx

# Set to production (not testnet):
BINANCE_TESTNET=false
```

#### Step 5: Restart Bot

```bash
# Restart the bot to apply new keys
docker-compose restart trading-bot

# Verify connection:
docker-compose logs trading-bot | grep -i "binance\|connected"
# Should see: "Connected to Binance Futures" or similar
```

---

### For Testnet (Paper Trading)

#### Step 1: Get Testnet API Keys

1. Go to: https://testnet.binancefuture.com
2. API Management → Create API Key
3. Copy the key and secret

**Note:** Testnet keys are different from production keys!

#### Step 2: Configure Bot for Testnet

```bash
# Edit .env
nano .env

# Set testnet keys:
BINANCE_API_KEY=testnet_key_here
BINANCE_SECRET_KEY=testnet_secret_here

# Enable testnet mode:
BINANCE_TESTNET=true
BINANCE_BASE_URL=https://testnet.binance.vision

# Can disable IP whitelist for testnet (more flexible)
```

#### Step 3: Verify Testnet Connection

```bash
# Check logs
docker-compose logs trading-bot | grep -i "testnet\|connected"

# Should see: "Using testnet: true"
```

---

## First Trade Checklist

**Before enabling autopilot for real trading:**

### Week 1: Paper Trading Setup

- [ ] ✅ Bot is running on testnet
- [ ] ✅ Dashboard is accessible and showing correct balances
- [ ] ✅ LLM provider is working (Claude/OpenAI/DeepSeek)
- [ ] ✅ Autopilot is DISABLED
- [ ] ✅ Place at least 3 manual test orders to verify execution
- [ ] ✅ Verify orders show up on Binance testnet dashboard

### Week 2-3: Autopilot Paper Trading

- [ ] ✅ Enable autopilot with conservative settings:
  ```env
  FUTURES_AUTOPILOT_ENABLED=true
  FUTURES_AUTOPILOT_RISK_LEVEL=low
  FUTURES_AUTOPILOT_MAX_DAILY_LOSS=20.0
  FUTURES_AUTOPILOT_MAX_POSITION_SIZE=100.0
  FUTURES_AUTOPILOT_MIN_CONFIDENCE=0.75  # Very high bar
  ```

- [ ] ✅ Monitor bot for 48 hours continuously (at least)
- [ ] ✅ Verify autopilot is generating and executing trades
- [ ] ✅ Check P&L shows realistic numbers
- [ ] ✅ Monitor logs for errors or warnings
- [ ] ✅ Test "pause" and "resume" functionality
- [ ] ✅ Test "close all positions" in emergency scenario

### Week 4: Transition to Live (if profitable on paper)

**Only proceed if:**
- ✅ Paper trading was profitable (made money consistently)
- ✅ Win rate > 40% over 20+ trades
- ✅ No API errors or connection issues
- ✅ Understand all settings and what they control

**Preparation:**
1. Backup `.env` file (important!)
2. Create backup of PostgreSQL database:
   ```bash
   docker-compose exec postgres pg_dump -U trading_bot trading_bot > backup.sql
   ```
3. Update API keys to production
4. Set `BINANCE_TESTNET=false`
5. Keep position size SMALL initially

```env
# Conservative live trading
BINANCE_TESTNET=false
FUTURES_AUTOPILOT_MAX_DAILY_LOSS=50.0
FUTURES_AUTOPILOT_MAX_POSITION_SIZE=200.0
FUTURES_AUTOPILOT_MIN_CONFIDENCE=0.65
FUTURES_DEFAULT_LEVERAGE=3  # Lower than paper
```

### Week 5+: Monitor and Scale

- [ ] ✅ Monitor every day for first week of live trading
- [ ] ✅ Check P&L and verify numbers match expectations
- [ ] ✅ Monitor logs for any errors
- [ ] ✅ After 1 week of profitable trading, consider increasing position size
- [ ] ✅ After 2 weeks of consistent profits, enable more aggressive settings

---

## Troubleshooting Quick Reference

| Problem | Check First | Solution |
|---------|------------|----------|
| API Error: -2015 | IP whitelist | Add server IP to Binance API whitelist |
| API Error: -1021 | Server time | Run `ntpdate pool.ntp.org` to sync time |
| Database connection failed | PostgreSQL running? | `docker-compose restart postgres` |
| No trade signals | AI API key | Verify `AI_CLAUDE_API_KEY` is set and valid |
| Autopilot running but no trades | Confidence threshold | Lower `MIN_CONFIDENCE` or verify signals |
| High CPU usage | Too many symbols | Reduce `FUTURES_AUTOPILOT_ALLOWED_SYMBOLS` |
| Dashboard not loading | Frontend built? | `cd web && npm install && npm run build` |
| Orphan orders on Binance | Position cleanup | Manual cleanup or auto-cleanup via API |

---

## Getting Help

1. **Check logs first:**
   ```bash
   docker-compose logs trading-bot | tail -50
   ```

2. **Common issues:**
   - See SETUP.md Troubleshooting section
   - Check `.env` file for typos or missing values

3. **Community/Support:**
   - GitHub Issues: [your-repo/issues](https://github.com/your-repo/issues)
   - Email: support@your-site.com
   - Documentation: [docs.your-site.com](https://docs.your-site.com)

---

## Next Steps

1. **New users**: Follow "5-Minute Quick Start" above
2. **Choose your profile**: Beginner, Intermediate, or Advanced
3. **Run through security checklist** before going live
4. **Paper trade for 2-4 weeks** minimum
5. **Monitor daily** for first week of live trading

Good luck! 🚀
