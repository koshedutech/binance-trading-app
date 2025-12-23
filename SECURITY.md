# Binance Trading Bot - Security Guide

Comprehensive security best practices for running the Binance Trading Bot with real money.

## Table of Contents

1. [API Key Security](#api-key-security)
2. [Network Security](#network-security)
3. [Database Security](#database-security)
4. [Application Security](#application-security)
5. [Trading Limits & Circuit Breakers](#trading-limits--circuit-breakers)
6. [Monitoring & Alerts](#monitoring--alerts)
7. [Incident Response](#incident-response)
8. [Compliance & Audit](#compliance--audit)

---

## API Key Security

### 1.1 Binance API Key Creation

#### ✅ DO:

1. **Create separate API keys for different environments:**
   - One key for testnet (paper trading)
   - One key for production (real trading)
   - Never reuse the same key

2. **Use descriptive names:**
   ```
   "Trading Bot - Testnet - MyName - Jan2025"
   "Trading Bot - Production - MyName - Jan2025"
   ```
   This makes it easy to identify and rotate later.

3. **Create API keys via security-verified session:**
   - Use 2FA enabled account
   - Create keys from secure network (not public WiFi)
   - Verify email confirmation link immediately

#### ❌ DON'T:

1. **Never mix testnet and production keys**
   - Testnet key format: starts with random chars
   - Production key format: typically longer
   - Always verify which environment you're connecting to

2. **Never share your API key or secret**
   - Never email them
   - Never paste in Discord/Telegram/Reddit
   - Never commit to GitHub
   - Never screenshot for support requests

3. **Never use the same key for multiple bots**
   - Each bot should have its own API key
   - Makes it easier to disable if one bot is compromised
   - Better audit trail

---

### 1.2 API Key Permissions (Critical)

#### Correct Permissions Configuration

1. **Go to:** https://www.binance.com/en/account/settings/api-management
2. **Edit your API key restrictions:**

| Permission | Enable | Reason |
|-----------|--------|--------|
| Enable Reading | ✅ YES | Bot needs to check balance and orders |
| Enable Spot & Margin Trading | ✅ YES | For spot mode (if enabled) |
| Enable Futures | ✅ YES | For futures autopilot |
| **Enable Withdrawals** | ❌ **NO** | **CRITICAL** - prevents theft of funds |
| Restrict to trusted IPs only | ✅ YES | Only your server can use this key |

#### Why "Enable Withdrawals" Must Be Disabled

**Scenario:** Your server is hacked
- **With withdrawals enabled:** Attacker can transfer all your Binance balance to their wallet
- **With withdrawals disabled:** Attacker can only trade (lose money slowly, more time to detect)

**Disable it immediately if enabled:**
1. Go to API Management
2. Edit the key
3. Under restrictions, find "Enable Withdrawals"
4. Make sure it's **OFF / DISABLED**
5. Click Save

---

### 1.3 IP Whitelist Configuration

#### Step 1: Identify Your Server IP

**For VPS/Cloud Server:**
```bash
# SSH into your server and run:
curl ifconfig.me
# Or:
hostname -I

# Example output: 203.0.113.45
```

**For Local/Home Server:**
```bash
# Find your public IP:
curl ifconfig.me

# Find your local network IP:
hostname -I

# Use public IP for Binance whitelisting
```

**For Docker Container:**
```bash
# Find the container's IP:
docker inspect trading-bot | grep "IPAddress"
# But use the SERVER'S public IP for Binance, not container IP
```

#### Step 2: Whitelist on Binance

1. Go to: https://www.binance.com/en/account/settings/api-management
2. Click on your API key
3. Find "Restrict access to trusted IPs only"
4. Click "Add IP"
5. Enter your server's public IP (e.g., 203.0.113.45)
6. Click "Add"
7. **Wait 2-5 minutes** for changes to propagate
8. **Test the connection:**
   ```bash
   curl -H "X-MBX-APIKEY: YOUR_KEY" "https://api.binance.com/api/v3/account"
   # Should return your account data without error
   ```

#### Step 3: IP Whitelist Maintenance

**⚠️ If your IP changes, your API will stop working!**

Options:
1. **Use static IP (Recommended)**
   - Most VPS providers offer static IPs
   - Cost: Usually included or $1-5/month extra
   - Example: DigitalOcean, Linode, AWS

2. **IP Whitelist Multiple IPs**
   - Add both old and new IP if planning to migrate
   - But security best practice: use only ONE IP
   - Coordinate migration carefully

3. **Dynamic DNS (Less Secure)**
   - Only use if you have no other option
   - Still needs to be whitelisted on Binance
   - Higher risk of temporary outages

**Check Current Whitelist:**
```bash
# Test if your current IP is in the whitelist
curl -H "X-MBX-APIKEY: YOUR_KEY" "https://api.binance.com/api/v3/account" -v
# If IP is whitelisted, you'll get account data
# If not whitelisted, you'll get: {"code":-2015,"msg":"Invalid API-key, IP, or permissions for action"}
```

---

### 1.4 API Key Storage & Rotation

#### Storage

**✅ CORRECT:**
```bash
# .env file (local, never committed to git)
BINANCE_API_KEY=zmxxxxxxxxxxxxxxxxxxx
BINANCE_SECRET_KEY=xxxxxxxxxxxxxxxxxx
```

**❌ WRONG:**
```bash
# In source code
const API_KEY = "zmxxxxxxxxxxxxxxxxxxx"

# Committed to GitHub
# In environment variables visible in Docker build logs
# In configuration files in /etc
```

#### Rotation Schedule

**Testnet Keys:**
- Rotate every 3 months (or never, since no real money at risk)
- Or when developer leaves the team

**Production Keys:**
- Rotate every 1 month (recommended)
- Or every 3 months minimum
- Always before employee departure

**Rotation Process:**
1. Create new API key
2. Update bot config with new key
3. Restart bot and verify it's working
4. Wait 24 hours to confirm stability
5. Delete old API key on Binance

---

### 1.5 API Key Security Audit

**Run this monthly:**

```bash
# 1. Check API key is still set correctly
grep BINANCE_API_KEY .env

# 2. Test API key connection
curl -H "X-MBX-APIKEY: YOUR_KEY" "https://api.binance.com/api/v3/account"

# 3. Verify IP whitelist on Binance
# Go to: https://www.binance.com/en/account/settings/api-management
# Check that only your server IP is whitelisted

# 4. Verify withdrawal is still disabled
# (Can't check via API, must check on Binance website)

# 5. Review recent API usage
# On Binance: API Management → View Account IP Access History
# Verify all IPs are from your server

# 6. Check logs for failed API attempts
docker-compose logs trading-bot | grep -i "invalid.*key\|api.*error"
```

---

## Network Security

### 2.1 Server Network Configuration

#### Firewall Rules

**Allow only these ports:**
```bash
# SSH access (for administration)
22/tcp   - Restricted to your home/office IP only

# HTTP (redirect to HTTPS)
80/tcp   - Open to all (will redirect to 443)

# HTTPS (encrypted dashboard access)
443/tcp  - Open to all (encrypted)

# Block everything else by default
```

**UFW Firewall Example (Linux):**
```bash
# Enable firewall
sudo ufw enable

# Allow SSH from specific IP only
sudo ufw allow from 192.168.1.100 to any port 22

# Allow HTTP/HTTPS
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# Allow database access ONLY from localhost (not internet)
sudo ufw allow 5432/tcp  # Only if needed for remote access

# Check rules
sudo ufw status
```

#### Network Segmentation

**Ideal Setup (if possible):**
- VPC/Private network for database
- Separate security group for web server
- Database has NO inbound internet access
- Database only accepts connections from bot server

**Docker Network Setup:**
```bash
# Ensure database is not exposed to internet
docker-compose logs postgres | grep -i "0.0.0.0"
# Should NOT see postgres listening on 0.0.0.0:5432

# Correct: 127.0.0.1:5432 (localhost only)
# Wrong: 0.0.0.0:5432 (accessible from internet)
```

---

### 2.2 HTTPS/TLS Configuration

**For Production Dashboard Access:**

#### Option 1: Let's Encrypt (Free)

```bash
# Install certbot
sudo apt-get install certbot python3-certbot-nginx

# Get certificate
sudo certbot certonly --standalone -d your-domain.com

# This creates:
# /etc/letsencrypt/live/your-domain.com/fullchain.pem
# /etc/letsencrypt/live/your-domain.com/privkey.pem
```

#### Option 2: Self-Signed Certificate

```bash
# Generate self-signed certificate (for testing only)
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365
```

#### Configure HTTPS in Nginx

**Create `docker-compose.override.yml`:**
```yaml
version: '3.8'
services:
  nginx:
    image: nginx:latest
    ports:
      - "443:443"
      - "80:80"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - /etc/letsencrypt/live/your-domain.com/fullchain.pem:/etc/nginx/certs/cert.pem:ro
      - /etc/letsencrypt/live/your-domain.com/privkey.pem:/etc/nginx/certs/key.pem:ro
    depends_on:
      - trading-bot
```

---

## Database Security

### 3.1 PostgreSQL Password Security

#### Strong Password Requirements

**Generate a strong password:**
```bash
# Using OpenSSL (Linux/Mac)
openssl rand -base64 32

# Using PowerShell (Windows)
[System.Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes((New-Guid).ToString())) | Select-Object -First 32
```

**Example strong password:**
```
aB3$xK9!mP2#wL7@qR5%vN8&jH4*dF6
```

#### Update Database Password

```bash
# 1. Access PostgreSQL
docker-compose exec postgres psql -U trading_bot

# 2. Change password
ALTER USER trading_bot WITH PASSWORD 'new_secure_password';
\q  # Exit

# 3. Update .env
nano .env
# Change: DB_PASSWORD=new_secure_password

# 4. Restart bot
docker-compose restart trading-bot

# 5. Verify connection
docker-compose logs trading-bot | grep -i "database.*connected\|database.*error"
```

### 3.2 Database Backup & Recovery

#### Automated Daily Backups

**Create `backup.sh`:**
```bash
#!/bin/bash

BACKUP_DIR="/backups"
DATE=$(date +%Y%m%d_%H%M%S)
DB_NAME="trading_bot"
DB_USER="trading_bot"

# Create backup
docker-compose exec -T postgres pg_dump -U $DB_USER $DB_NAME | gzip > $BACKUP_DIR/backup_$DATE.sql.gz

# Keep only last 30 days
find $BACKUP_DIR -name "backup_*.sql.gz" -mtime +30 -delete

echo "Backup completed: backup_$DATE.sql.gz"
```

**Add to Crontab (daily at 2 AM):**
```bash
crontab -e

# Add this line:
0 2 * * * /path/to/backup.sh
```

#### Manual Backup

```bash
# Full database backup
docker-compose exec postgres pg_dump -U trading_bot trading_bot > backup.sql

# Compressed backup (smaller file)
docker-compose exec postgres pg_dump -U trading_bot trading_bot | gzip > backup.sql.gz
```

#### Recovery from Backup

```bash
# 1. Stop the bot
docker-compose stop trading-bot

# 2. Restore database
gunzip < backup.sql.gz | docker-compose exec -T postgres psql -U trading_bot -d trading_bot

# 3. Restart bot
docker-compose start trading-bot

# 4. Verify
docker-compose logs trading-bot | grep -i "database.*connected"
```

---

## Application Security

### 4.1 Authentication

#### Enable Dashboard Authentication

```env
# .env
AUTH_ENABLED=true
JWT_SECRET=your_very_long_random_secret_here_min_32_chars
JWT_EXPIRY=24h
REQUIRE_EMAIL_VERIFICATION=false  # Optional
```

**Generate secure JWT secret:**
```bash
openssl rand -base64 64
```

#### User Management

```bash
# Change default admin password IMMEDIATELY after setup
# (If using default credentials)

# API endpoint to create user:
curl -X POST http://localhost:8090/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "very_secure_password",
    "email": "admin@example.com"
  }'

# Login:
curl -X POST http://localhost:8090/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "very_secure_password"
  }'
```

#### Session Security

- Login token expires after 24 hours (set `JWT_EXPIRY`)
- Tokens are stored in browser localStorage (encrypted by browser)
- Clear cookies on logout
- Use HTTPS only in production

---

### 4.2 Environment Variable Security

**✅ Correct:**
```bash
# .env file (never committed)
BINANCE_API_KEY=your_key
BINANCE_SECRET_KEY=your_secret
DATABASE_PASSWORD=secure_password
```

**❌ Wrong:**
```bash
# In docker-compose.yml (committed to git)
environment:
  BINANCE_API_KEY: your_key  # 🚨 EXPOSED!

# In Dockerfile
ENV BINANCE_SECRET_KEY=your_secret  # 🚨 EXPOSED IN BUILD!

# In source code
const PASSWORD = "password123"  # 🚨 HARDCODED!
```

**Secure .env Usage:**
```bash
# 1. Add .env to .gitignore (already done)
cat .gitignore | grep "\.env"

# 2. Use .env.example (template without secrets)
cp .env.example .env  # User fills in their own values

# 3. Never log secrets in debug mode
LOG_LEVEL=INFO  # Not DEBUG (which might log environment variables)

# 4. Restrict file permissions
chmod 600 .env  # Only owner can read/write
```

---

### 4.3 Logging & Secret Handling

#### Configure Secure Logging

```env
# .env
LOG_LEVEL=INFO
LOG_JSON=true
LOG_INCLUDE_FILE=true
LOG_OUTPUT=/var/log/trading-bot.log
```

**What to log:**
- ✅ API connection attempts (IP, success/failure)
- ✅ Trade execution (symbol, price, quantity)
- ✅ Error messages (but NOT API keys)
- ✅ System events (startup, shutdown)

**What NOT to log:**
- ❌ API keys
- ❌ Database passwords
- ❌ JWT tokens
- ❌ Raw request/response bodies with secrets

#### Log Monitoring

```bash
# Check for accidental secrets in logs
grep -r "BINANCE_API_KEY\|SECRET_KEY\|PASSWORD" /var/log/

# Monitor for suspicious API errors
docker-compose logs trading-bot | grep -i "invalid.*key\|unauthorized\|forbidden"
```

---

## Trading Limits & Circuit Breakers

### 5.1 Circuit Breaker Configuration

**Prevent catastrophic losses with circuit breakers:**

```env
# CRITICAL: Enable circuit breaker
CIRCUIT_BREAKER_ENABLED=true

# Stop trading if lose $100 in one hour
CIRCUIT_MAX_LOSS_PER_HOUR=100.0

# Stop if lose 10 trades in a row
CIRCUIT_MAX_CONSECUTIVE_LOSSES=10

# Can't resume for 30 minutes after trigger
CIRCUIT_COOLDOWN_MINUTES=30
```

#### How It Works

1. **Tracks recent losses:**
   - Last hour: Sum of all losing trades
   - Consecutive: Count of trades without a win

2. **Triggers pause if:**
   - Cumulative loss in last hour > `MAX_LOSS_PER_HOUR`
   - OR consecutive losses > `MAX_CONSECUTIVE_LOSSES`

3. **Pause effect:**
   - No new trades allowed
   - Existing positions stay open (not closed)
   - Auto-resumes after `COOLDOWN_MINUTES`

### 5.2 Per-Mode Capital Allocation

**Prevent single mode from consuming all capital:**

```json
{
  "mode_allocation": {
    "ultra_fast_scalp_percent": 30,
    "scalp_percent": 30,
    "swing_percent": 25,
    "position_percent": 15,

    "max_ultra_fast_positions": 3,
    "max_scalp_positions": 4,
    "max_swing_positions": 3,
    "max_position_positions": 2,

    "max_ultra_fast_usd_per_position": 200,
    "max_scalp_usd_per_position": 300,
    "max_swing_usd_per_position": 500,
    "max_position_usd_per_position": 750
  }
}
```

**Benefits:**
- Diversification across trading styles
- Limits exposure in any single mode
- Easy to balance risk vs reward

---

### 5.3 Position Size Limits

**CRITICAL: Start small and scale up slowly**

```env
# Week 1-2 (Learning)
FUTURES_AUTOPILOT_MAX_POSITION_SIZE=50.0     # $50 per position max
FUTURES_AUTOPILOT_MAX_DAILY_LOSS=20.0        # $20 daily loss limit

# Week 3-4 (Validation)
FUTURES_AUTOPILOT_MAX_POSITION_SIZE=200.0    # $200 per position max
FUTURES_AUTOPILOT_MAX_DAILY_LOSS=100.0       # $100 daily loss limit

# Week 5+ (Full Size - if consistently profitable)
FUTURES_AUTOPILOT_MAX_POSITION_SIZE=500.0    # $500 per position max
FUTURES_AUTOPILOT_MAX_DAILY_LOSS=500.0       # $500 daily loss limit
```

---

## Monitoring & Alerts

### 6.1 Daily Health Check

**Create `health-check.sh`:**
```bash
#!/bin/bash

echo "=== Trading Bot Health Check ==="
echo "Time: $(date)"

# Check bot is running
docker-compose ps | grep -i "trading-bot.*up" && echo "✅ Bot running" || echo "❌ Bot not running"

# Check database connection
docker-compose exec trading-bot curl -s http://localhost:8090/api/health | grep -q "ok" && echo "✅ Health: OK" || echo "❌ Health: FAILED"

# Check Binance connection
docker-compose logs trading-bot --tail 20 | grep -i "connected\|failed" | tail -1

# Check recent errors
echo ""
echo "Recent errors (last 10):"
docker-compose logs trading-bot --tail 100 | grep -i "error\|failed" | tail -10

# Check disk space
echo ""
echo "Disk usage:"
df -h | grep -E "^/dev/|Filesystem"

# Check database size
echo ""
echo "Database size:"
docker-compose exec postgres psql -U trading_bot -d trading_bot -c "SELECT pg_size_pretty(pg_database_size('trading_bot'));" 2>/dev/null || echo "Could not check"

echo ""
echo "=== End Health Check ==="
```

**Run daily:**
```bash
chmod +x health-check.sh
./health-check.sh

# Or schedule it:
crontab -e
# 0 9 * * * /path/to/health-check.sh | mail -s "Trading Bot Health" your-email@example.com
```

### 6.2 Alert Triggers

**Set up alerts for:**
1. Bot stops running
2. Database connection fails
3. API errors increase
4. Unusual trading activity
5. Circuit breaker triggers
6. Daily loss exceeds threshold

---

## Incident Response

### 7.1 If API Key is Compromised

**Immediate actions (within 5 minutes):**
1. Disable the API key on Binance
2. Restart bot with empty API key
   ```bash
   # Edit .env
   BINANCE_API_KEY=
   # Restart
   docker-compose restart trading-bot
   ```
3. Check recent trades on Binance for unauthorized activity

**Within 1 hour:**
1. Analyze logs for unusual activity
   ```bash
   docker-compose logs trading-bot | grep -i "trade\|order\|error" > suspicious_activity.log
   ```
2. Create new API key
3. Update bot config
4. Review account activity on Binance (transfers, trades)

**Within 24 hours:**
1. Rotate database password
2. Review all server access logs
3. File report with Binance support if funds were lost
4. Review and improve security (see Security Checklist)

### 7.2 If Bot Stops Working

**Troubleshooting steps:**
```bash
# Check if container is still running
docker-compose ps

# Check logs for errors
docker-compose logs trading-bot --tail 50

# Try to restart
docker-compose restart trading-bot

# Check if database is accessible
docker-compose exec trading-bot psql -h postgres -U trading_bot -d trading_bot -c "SELECT 1;"

# Check API connectivity
curl -H "X-MBX-APIKEY: YOUR_KEY" "https://api.binance.com/api/v3/account"
```

### 7.3 If You're Getting DDoS'ed

**Signs:**
- Dashboard very slow or unresponsive
- API requests timing out
- High CPU usage on server

**Response:**
```bash
# 1. Check if it's legitimate traffic
docker-compose logs trading-bot | tail -100

# 2. Enable rate limiting in Nginx
# (Configure in nginx.conf)

# 3. Whitelist only your IP for dashboard access
# Modify firewall to block 80/443 except from your IP

# 4. Consider using Cloudflare DDoS protection
# Route traffic through Cloudflare before your server
```

---

## Compliance & Audit

### 8.1 Audit Trail

**Keep records of:**
1. All API key rotations (date, time, reason)
2. All configuration changes (date, time, what changed)
3. All access logs (who, when, from where)
4. All trading activity (automatically logged)

**Create audit log:**
```bash
# Add to backup.sh
echo "[$(date)] Bot status: $(docker-compose ps | grep trading-bot)" >> /var/log/trading-bot-audit.log
echo "[$(date)] Database: $(du -sh /var/lib/postgresql/data)" >> /var/log/trading-bot-audit.log
```

### 8.2 Compliance Considerations

**Tax implications:**
- Trading bot generates trades with taxable gains/losses
- Keep records of all trades for tax reporting
- Consider consulting tax professional

**Record retention:**
- Keep all logs for minimum 1 year
- Keep all trades for minimum 7 years (audit requirement in many jurisdictions)
- Database backups should be kept indefinitely (for recovery)

---

## Security Checklist (Before Going Live)

- [ ] ✅ API key created with separate testnet and production keys
- [ ] ✅ Withdrawals DISABLED on API key
- [ ] ✅ IP whitelist enabled with server's static IP
- [ ] ✅ API key stored in .env file (not in code)
- [ ] ✅ .gitignore includes .env
- [ ] ✅ .env file permissions: `chmod 600 .env`
- [ ] ✅ Database password is strong (20+ characters)
- [ ] ✅ Database not exposed to internet
- [ ] ✅ Daily backups configured and tested
- [ ] ✅ Backup restore tested at least once
- [ ] ✅ Firewall configured (only 22, 80, 443 open)
- [ ] ✅ HTTPS configured for dashboard
- [ ] ✅ Authentication enabled on dashboard
- [ ] ✅ Circuit breaker enabled
- [ ] ✅ Position size limits set conservatively
- [ ] ✅ Logs monitored daily
- [ ] ✅ Health checks automated
- [ ] ✅ Incident response plan documented
- [ ] ✅ Rotation schedule created for API keys

---

## Additional Resources

- **Binance Security**: https://www.binance.com/en/support/faq/how-to-protect-your-binance-account-27c53f7c24cc4f37abaf14f628395bd5
- **OWASP Security**: https://owasp.org/www-project-top-ten/
- **Docker Security**: https://docs.docker.com/engine/security/
- **PostgreSQL Security**: https://www.postgresql.org/docs/current/sql-createrole.html

---

**Last Updated:** 2025-12-23
**Version:** 1.0
