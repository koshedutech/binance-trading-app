# Docker Setup Guide for Binance Trading Bot

This guide will help you set up and run the Binance Trading Bot using Docker and Docker Compose.

## Prerequisites

Before you begin, ensure you have the following installed:

- **Docker** (version 20.10 or higher)
- **Docker Compose** (version 2.0 or higher)
- **Binance API credentials** (Testnet recommended for testing)

### Check Docker Installation

```bash
docker --version
docker-compose --version
```

If Docker is not installed, visit:
- **Windows/Mac**: [Docker Desktop](https://www.docker.com/products/docker-desktop)
- **Linux**: [Docker Engine](https://docs.docker.com/engine/install/)

## Quick Start

### 1. Get Binance API Keys

#### For Testnet (Recommended for Testing)
1. Visit [Binance Testnet](https://testnet.binance.vision/)
2. Login with your email
3. Click "Generate HMAC_SHA256 Key"
4. Save your **API Key** and **Secret Key** securely

#### For Production (Real Trading)
1. Login to [Binance](https://www.binance.com)
2. Go to **Profile → API Management**
3. Create a new API key
4. Enable **Spot & Margin Trading** permissions
5. Set **IP restrictions** for security (recommended)
6. Save your credentials securely

### 2. Configure API Credentials

You have two options for configuration:

#### Option A: Using .env file (Recommended)

Edit the `.env` file:
```bash
nano .env
```

Update with your credentials:
```bash
BINANCE_API_KEY=your_actual_api_key_here
BINANCE_SECRET_KEY=your_actual_secret_key_here
BINANCE_BASE_URL=https://testnet.binance.vision
BINANCE_TESTNET=true
```

**Important**:
- Use `https://testnet.binance.vision` for testnet
- Use `https://api.binance.com` for production
- Set `BINANCE_TESTNET=false` when using production

#### Option B: Using config.json

Edit the `config.json` file:
```bash
nano config.json
```

Update the binance section:
```json
{
  "binance": {
    "api_key": "your_actual_api_key_here",
    "secret_key": "your_actual_secret_key_here",
    "base_url": "https://testnet.binance.vision",
    "testnet": true
  },
  "screener": {
    "enabled": true,
    "interval": "15m",
    "min_volume": 100000,
    "min_price_change": 2.0,
    "exclude_symbols": ["BUSDUSDT", "USDCUSDT"],
    "quote_currency": "USDT",
    "max_symbols": 50,
    "screening_interval": 60
  },
  "trading": {
    "max_open_positions": 5,
    "max_risk_per_trade": 2.0,
    "dry_run": true
  }
}
```

### 3. Build and Run with Docker Compose

#### Build the Docker Image
```bash
docker-compose build
```

This will:
- Build a multi-stage Docker image
- Install all Go dependencies
- Compile the trading bot
- Create an optimized Alpine Linux image (~20MB)

#### Start the Trading Bot
```bash
docker-compose up -d
```

The `-d` flag runs the container in detached mode (background).

#### View Logs
```bash
docker-compose logs -f
```

Use `Ctrl+C` to stop viewing logs (bot continues running).

#### Stop the Bot
```bash
docker-compose down
```

### 4. Verify Everything is Working

Check container status:
```bash
docker ps
```

You should see:
```
CONTAINER ID   IMAGE                    STATUS         PORTS
abc123...      binance-trading-bot      Up 10 seconds
```

Check logs for successful startup:
```bash
docker-compose logs | grep "Starting"
```

Expected output:
```
trading-bot | Starting Binance Trading Bot...
trading-bot | Dry run mode: true
trading-bot | Strategy registered: breakout_high
trading-bot | Screener started
```

## Configuration Details

### Trading Configuration

The bot is configured for **dry run mode** by default (no real trades):

```json
"trading": {
  "max_open_positions": 5,
  "max_risk_per_trade": 2.0,
  "dry_run": true
}
```

**Important**: Keep `dry_run: true` until you're confident the bot works correctly!

### Screener Configuration

The market screener automatically finds trading opportunities:

```json
"screener": {
  "enabled": true,
  "interval": "15m",            // Candle interval (1m, 5m, 15m, 1h, 4h, 1d)
  "min_volume": 100000,         // Minimum 24h volume in USDT
  "min_price_change": 2.0,      // Minimum 24h price change %
  "quote_currency": "USDT",     // Filter for USDT pairs
  "max_symbols": 50,            // Max symbols to scan
  "screening_interval": 60      // Seconds between scans
}
```

## Docker Compose Configuration

The `docker-compose.yml` includes:

### Services
- **trading-bot**: Main application container

### Volumes
- `./config.json:/app/config.json:ro` - Configuration (read-only)
- `./logs:/app/logs` - Log files (persistent)

### Environment Variables
- `BINANCE_API_KEY` - Your API key
- `BINANCE_SECRET_KEY` - Your secret key
- `BINANCE_BASE_URL` - API endpoint
- `BINANCE_TESTNET` - Testnet flag

### Logging
- Driver: json-file
- Max size: 10MB per file
- Max files: 3 (rotates automatically)

## Docker Commands Reference

### Basic Operations
```bash
# Build the image
docker-compose build

# Start the bot (detached)
docker-compose up -d

# Stop the bot
docker-compose down

# Restart the bot
docker-compose restart

# View logs (follow mode)
docker-compose logs -f

# View last 100 lines
docker-compose logs --tail=100
```

### Container Management
```bash
# List running containers
docker ps

# List all containers (including stopped)
docker ps -a

# Execute command in running container
docker-compose exec trading-bot sh

# Check container resource usage
docker stats binance-trading-bot
```

### Cleanup
```bash
# Remove stopped containers
docker-compose down

# Remove containers and volumes
docker-compose down -v

# Remove unused images
docker image prune

# Remove everything (images, containers, volumes)
docker system prune -a
```

## Monitoring and Maintenance

### View Live Logs
```bash
docker-compose logs -f trading-bot
```

### Check Application Status
```bash
# Is the container running?
docker ps | grep binance-trading-bot

# Check container health
docker inspect binance-trading-bot | grep Status
```

### Access Log Files
Logs are persisted in the `./logs` directory:
```bash
ls -lh logs/
tail -f logs/trading-bot.log
```

### Update Configuration
After changing `config.json` or `.env`:
```bash
docker-compose restart
```

## Troubleshooting

### Issue: Container Exits Immediately
```bash
# Check logs for errors
docker-compose logs

# Common causes:
# - Invalid API credentials
# - Missing config.json
# - Syntax error in config.json
```

### Issue: "Invalid API Key"
```bash
# Verify credentials in .env
cat .env

# Ensure you're using testnet keys with testnet URL
# Testnet keys: https://testnet.binance.vision
# Production keys: https://api.binance.com
```

### Issue: "Cannot find config.json"
```bash
# Verify file exists
ls -la config.json

# Check Docker volume mount
docker-compose config | grep volumes
```

### Issue: Permission Denied (Logs)
```bash
# Fix logs directory permissions
chmod 777 logs/
```

### Issue: Port Already in Use
If you uncomment the web interface port:
```bash
# Find what's using port 8080
lsof -i :8080  # Mac/Linux
netstat -ano | findstr :8080  # Windows

# Change port in docker-compose.yml
ports:
  - "8081:8080"  # Use different host port
```

## Production Deployment

### Before Going Live

1. **Test Extensively**
   - Run in dry run mode for at least 2-4 weeks
   - Monitor all signals and simulated trades
   - Verify strategy performance

2. **Update Configuration**
   ```json
   {
     "binance": {
       "base_url": "https://api.binance.com",
       "testnet": false
     },
     "trading": {
       "dry_run": false,  // Enable real trading
       "max_open_positions": 3,  // Start small
       "max_risk_per_trade": 1.0  // Low risk
     }
   }
   ```

3. **Enable Security Features**
   - Use API key IP restrictions
   - Enable only necessary permissions
   - Never commit real credentials to git
   - Use Docker secrets for sensitive data

4. **Set Up Monitoring**
   ```bash
   # Optional: Add Prometheus/Grafana
   # Uncomment in docker-compose.yml
   ```

5. **Backup Strategy**
   - Regular backups of config and logs
   - Document your strategies
   - Keep trading journal

### Production Docker Compose

For production, consider:
```yaml
services:
  trading-bot:
    restart: always  # Auto-restart on failure
    logging:
      driver: "json-file"
      options:
        max-size: "50m"
        max-file: "10"
    deploy:
      resources:
        limits:
          memory: 512M
```

## Security Best Practices

1. **Never share your API keys**
2. **Use .env file** (not committed to git)
3. **Enable IP restrictions** on Binance API keys
4. **Start with testnet** before production
5. **Use read-only volumes** for config
6. **Regular security updates**: `docker-compose pull && docker-compose up -d`
7. **Monitor API usage** to avoid rate limits

## Advanced Configuration

### Enable Web Interface (Future)
Uncomment in `docker-compose.yml`:
```yaml
ports:
  - "8080:8080"
```

And in `Dockerfile`:
```dockerfile
EXPOSE 8080
```

### Add Monitoring Stack
Uncomment Prometheus and Grafana sections in `docker-compose.yml`:
```bash
docker-compose up -d prometheus grafana
```

Access:
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)

## Support and Resources

- **Documentation**: See README.md, QUICKSTART.md
- **Binance API Docs**: https://binance-docs.github.io/apidocs/spot/en/
- **Binance Testnet**: https://testnet.binance.vision/
- **Docker Docs**: https://docs.docker.com/

## Disclaimer

**This trading bot is for educational purposes only. Cryptocurrency trading carries significant risk of financial loss. Use at your own risk. The authors are not responsible for any losses incurred.**

Always:
- Start with testnet
- Use dry run mode extensively
- Trade only what you can afford to lose
- Understand the strategies before deploying
- Monitor your bot regularly
- Comply with local regulations

---

**Happy Trading!** Remember to start small, test thoroughly, and never risk more than you can afford to lose.
