# Binance Trading Bot - Setup Guide

This guide covers the setup and deployment of the Binance Trading Bot for self-hosted use.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Quick Start with Docker](#quick-start-with-docker)
3. [Manual Installation](#manual-installation)
4. [Configuration](#configuration)
5. [Binance API Setup](#binance-api-setup)
6. [AI/LLM Configuration](#aillm-configuration)
7. [Running the Bot](#running-the-bot)
8. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Hardware Requirements
- **CPU**: 2+ cores recommended
- **RAM**: 2GB minimum, 4GB recommended
- **Storage**: 10GB minimum
- **Network**: Stable internet connection with static IP (for Binance API)

### Software Requirements
- Docker & Docker Compose (recommended)
- OR: Go 1.21+, Node.js 18+, PostgreSQL 15+

---

## Quick Start with Docker

The fastest way to get started is using Docker Compose.

### Step 1: Download and Extract

```bash
# Download the latest release
wget https://github.com/your-repo/releases/latest/download/trading-bot.tar.gz

# Extract
tar -xzf trading-bot.tar.gz
cd trading-bot
```

### Step 2: Configure Environment

```bash
# Copy example environment file
cp .env.example .env

# Edit with your settings
nano .env
```

**Required settings:**
```env
# Your license key (or leave empty for 7-day trial)
LICENSE_KEY=PRO-XXXX-XXXX-XXXX

# Binance API (get from binance.com)
BINANCE_API_KEY=your_api_key
BINANCE_SECRET_KEY=your_secret_key
BINANCE_TESTNET=true  # Start with testnet!

# AI Provider (at least one required)
AI_LLM_PROVIDER=claude
AI_CLAUDE_API_KEY=your_claude_key
```

### Step 3: Start Services

```bash
# Start all services
docker-compose up -d

# Check logs
docker-compose logs -f trading-bot
```

### Step 4: Access the Dashboard

Open your browser and navigate to:
- **Local**: http://localhost:8090
- **Remote**: http://your-server-ip:8090

---

## Manual Installation

### Step 1: Install Dependencies

**Ubuntu/Debian:**
```bash
# Install Go
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Install Node.js
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt-get install -y nodejs

# Install PostgreSQL
sudo apt-get install -y postgresql postgresql-contrib
```

**macOS:**
```bash
brew install go node postgresql@15
brew services start postgresql@15
```

### Step 2: Setup Database

```bash
# Create database and user
sudo -u postgres psql << EOF
CREATE USER trading_bot WITH PASSWORD 'your_secure_password';
CREATE DATABASE trading_bot OWNER trading_bot;
GRANT ALL PRIVILEGES ON DATABASE trading_bot TO trading_bot;
EOF
```

### Step 3: Build the Application

```bash
# Clone or download the source
cd /opt/trading-bot

# Build frontend
cd web
npm install
npm run build
cd ..

# Build backend
go build -o trading-bot .
```

### Step 4: Configure and Run

```bash
# Copy and edit config
cp .env.example .env
nano .env

# Run
./trading-bot
```

---

## Configuration

### License Key

Your license key determines which features are available:

| License Type | Max Symbols | Features |
|-------------|-------------|----------|
| Trial (free) | 3 | Spot trading, Basic signals |
| Personal | 10 | + Futures, AI analysis |
| Pro | 50 | + Ginie autopilot, Advanced signals |
| Enterprise | Unlimited | + API access, White label |

Set your license key in `.env`:
```env
LICENSE_KEY=PRO-XXXX-XXXX-XXXX
```

### Database Configuration

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=trading_bot
DB_PASSWORD=your_secure_password
DB_NAME=trading_bot
DB_SSLMODE=disable
```

### Server Configuration

```env
WEB_PORT=8090
WEB_HOST=0.0.0.0
```

---

## Binance API Setup

### Step 1: Create API Key

1. Log in to [Binance](https://www.binance.com)
2. Go to **Settings** → **API Management**
3. Click **Create API**
4. Complete security verification

### Step 2: Configure Permissions

Enable these permissions:
- ✅ Enable Reading
- ✅ Enable Spot & Margin Trading
- ✅ Enable Futures

**IMPORTANT**: Disable "Enable Withdrawals" for security!

### Step 3: Whitelist Your IP

1. Click **Restrict access to trusted IPs only**
2. Add your server's static IP address
3. Save changes

> ⚠️ **Dynamic IP Warning**: If your IP changes, the API will stop working.
> Use a VPS/cloud server with a static IP for production.

### Step 4: Add Keys to Config

```env
BINANCE_API_KEY=your_api_key_here
BINANCE_SECRET_KEY=your_secret_key_here
BINANCE_TESTNET=true  # Always start with testnet!
```

### Testnet vs Production

| Setting | Description |
|---------|-------------|
| `BINANCE_TESTNET=true` | Paper trading with fake money |
| `BINANCE_TESTNET=false` | **REAL MONEY** - be careful! |

Get testnet API keys from: https://testnet.binancefuture.com

---

## AI/LLM Configuration

The bot uses LLMs for market analysis. Configure at least one provider:

### Claude (Anthropic) - Recommended

```env
AI_LLM_PROVIDER=claude
AI_CLAUDE_API_KEY=sk-ant-xxx...
AI_LLM_MODEL=claude-sonnet-4-20250514
```

Get your key: https://console.anthropic.com/

### OpenAI

```env
AI_LLM_PROVIDER=openai
AI_OPENAI_API_KEY=sk-xxx...
AI_LLM_MODEL=gpt-4o
```

Get your key: https://platform.openai.com/api-keys

### DeepSeek (Budget Option)

```env
AI_LLM_PROVIDER=deepseek
AI_DEEPSEEK_API_KEY=sk-xxx...
AI_LLM_MODEL=deepseek-chat
```

Get your key: https://platform.deepseek.com/

---

## Running the Bot

### Start the Bot

**With Docker:**
```bash
docker-compose up -d
```

**Manual:**
```bash
./trading-bot
```

### Access Dashboard

Open: http://localhost:8090

### Start Trading

1. **Paper Trading First**: Set `BINANCE_TESTNET=true`
2. **Configure Autopilot**: Set risk levels, position sizes
3. **Enable Autopilot**: Toggle on in the dashboard
4. **Monitor**: Watch for trades and adjust settings

### Go Live

Only after successful paper trading:

1. Edit `.env`:
   ```env
   BINANCE_TESTNET=false
   TRADING_DRY_RUN=false
   ```
2. Update API keys to production keys
3. Whitelist your server IP on Binance
4. Restart the bot

---

## Troubleshooting

### API Error: -2015 Invalid API-key

**Cause**: Your IP is not whitelisted on Binance

**Solution**:
1. Find your server's IP: `curl ifconfig.me`
2. Add it to Binance API restrictions
3. Wait 5 minutes for changes to propagate

### API Error: -1021 Timestamp out of sync

**Cause**: Server time is not synchronized

**Solution**:
```bash
# Install NTP
sudo apt-get install -y ntp
sudo systemctl enable ntp
sudo ntpdate pool.ntp.org
```

### Database Connection Failed

**Cause**: PostgreSQL not running or wrong credentials

**Solution**:
```bash
# Check if PostgreSQL is running
sudo systemctl status postgresql

# Start if needed
sudo systemctl start postgresql

# Verify connection
psql -h localhost -U trading_bot -d trading_bot
```

### Frontend Not Loading

**Cause**: Static files not built or missing

**Solution**:
```bash
cd web
npm install
npm run build
cd ..
# Restart the bot
```

### High CPU/Memory Usage

**Cause**: Too many symbols or intervals

**Solution**:
1. Reduce `FUTURES_AUTOPILOT_ALLOWED_SYMBOLS`
2. Increase `FUTURES_AUTOPILOT_DECISION_INTERVAL`
3. Disable unused features (ML, sentiment)

---

## Security Best Practices

1. **Never share** your API keys or license key
2. **Use testnet** until you're confident
3. **Set withdrawal disabled** on Binance API
4. **Use IP whitelisting** on Binance
5. **Use a VPS** with static IP for production
6. **Regular backups** of your database
7. **Monitor logs** for suspicious activity

---

## Support

- **Documentation**: https://docs.your-site.com
- **Issues**: https://github.com/your-repo/issues
- **Email**: support@your-site.com

---

## License

This software requires a valid license key for commercial use.
Trial mode is available for 7 days with limited features.
