# Binance Trading Bot - Web Interface Setup Guide

## Overview

Your trading bot now has a complete web interface with:
- ✅ Real-time dashboard with live P&L tracking
- ✅ Position and order management
- ✅ Strategy controls (enable/disable)
- ✅ Market screener results
- ✅ Trading signals history
- ✅ Performance metrics and charts
- ✅ PostgreSQL database for persistence
- ✅ WebSocket for real-time updates
- ✅ Port 8088 for web access

## Quick Start

### 1. Update main.go

**IMPORTANT**: Replace your `main.go` with the version in `FINAL_MAIN_GO.md`. This integrates all the new components.

```bash
# Backup your current main.go
cp main.go main.go.backup

# Copy the new main.go from FINAL_MAIN_GO.md
```

### 2. Install Frontend Dependencies (Optional for Development)

If you want to run the frontend in development mode:

```bash
cd web
npm install
```

### 3. Start with Docker Compose

The easiest way to run everything:

```bash
# Build and start all services (PostgreSQL + Trading Bot + Web Interface)
docker-compose up --build
```

This will:
- Start PostgreSQL database on port 5432
- Build the React frontend
- Build the Go backend with frontend embedded
- Start the trading bot with web interface on port 8088
- Run database migrations automatically

### 4. Access the Dashboard

Open your browser and navigate to:
```
http://localhost:8088
```

You should see:
- Trading bot dashboard
- Real-time metrics
- Open positions table
- Active strategies
- Market screener results

## Architecture

```
┌─────────────────────────────────────────────────┐
│          Browser (http://localhost:8088)        │
└────────────────┬────────────────────────────────┘
                 │ HTTP/WebSocket
┌────────────────▼────────────────────────────────┐
│         Gin Web Server (Port 8088)              │
│  ┌──────────────┐    ┌──────────────┐          │
│  │ REST API     │    │ WebSocket    │          │
│  │ /api/*       │    │ /ws          │          │
│  └──────┬───────┘    └──────┬───────┘          │
│         │                   │                   │
│  ┌──────▼───────────────────▼───────┐          │
│  │        Event Bus                  │          │
│  └──────┬───────────────────┬────────┘          │
└─────────┼───────────────────┼───────────────────┘
          │                   │
┌─────────▼─────────┐  ┌──────▼───────────┐
│  Trading Bot      │  │   PostgreSQL     │
│  - Strategies     │  │   Database       │
│  - Orders         │  │   - Trades       │
│  - Positions      │  │   - Orders       │
│  - Screener       │  │   - Signals      │
└───────────────────┘  └──────────────────┘
```

## Configuration

### Environment Variables

The application uses these environment variables (set in docker-compose.yml):

**Binance API:**
```
BINANCE_API_KEY=your_key_here
BINANCE_SECRET_KEY=your_secret_here
BINANCE_BASE_URL=https://testnet.binance.vision
BINANCE_TESTNET=true
```

**Database:**
```
DB_HOST=postgres
DB_PORT=5432
DB_USER=trading_bot
DB_PASSWORD=trading_bot_password
DB_NAME=trading_bot
DB_SSLMODE=disable
```

**Web Server:**
```
WEB_PORT=8088
WEB_HOST=0.0.0.0
```

### Changing the Port

To use a different port than 8088:

1. Update `docker-compose.yml`:
```yaml
environment:
  - WEB_PORT=9000  # Your desired port
ports:
  - "9000:9000"     # Match the internal port
```

2. Rebuild and restart:
```bash
docker-compose up --build
```

## Development Mode

### Backend Development

Run the Go backend locally (requires Go 1.21+):

```bash
# Set environment variables
export BINANCE_API_KEY=your_key
export BINANCE_SECRET_KEY=your_secret
export BINANCE_TESTNET=true
export DB_HOST=localhost
export WEB_PORT=8088

# Download dependencies
go mod download

# Run the application
go run main.go
```

### Frontend Development

Run the React frontend with hot reload:

```bash
cd web
npm install
npm run dev
```

This starts Vite dev server on http://localhost:5173 with:
- Hot module replacement
- Proxy to backend API on localhost:8088
- WebSocket connection to backend

## Docker Commands

```bash
# Start services
docker-compose up

# Start in background
docker-compose up -d

# View logs
docker-compose logs -f

# View specific service logs
docker-compose logs -f trading-bot
docker-compose logs -f postgres

# Stop services
docker-compose down

# Stop and remove volumes (clears database)
docker-compose down -v

# Rebuild after code changes
docker-compose up --build

# Access PostgreSQL directly
docker-compose exec postgres psql -U trading_bot -d trading_bot
```

## Database Management

### View Database

Access PostgreSQL:
```bash
docker-compose exec postgres psql -U trading_bot -d trading_bot
```

Useful queries:
```sql
-- View all trades
SELECT * FROM trades ORDER BY entry_time DESC LIMIT 10;

-- View open positions
SELECT * FROM trades WHERE status = 'OPEN';

-- View recent signals
SELECT * FROM signals ORDER BY timestamp DESC LIMIT 20;

-- View trading metrics
SELECT
  COUNT(*) as total_trades,
  SUM(CASE WHEN pnl > 0 THEN 1 ELSE 0 END) as winning_trades,
  SUM(pnl) as total_pnl
FROM trades WHERE status = 'CLOSED';
```

### Optional: pgAdmin

Uncomment the pgAdmin section in `docker-compose.yml` for a web-based database UI:

```yaml
pgadmin:
  image: dpage/pgadmin4:latest
  # ... (already in docker-compose.yml, just uncomment)
```

Access at: http://localhost:5050
- Email: admin@admin.com
- Password: admin

## Features Guide

### 1. Dashboard

Main page showing:
- **Metrics Cards**: Total P&L, Win Rate, Open Positions, Total Trades
- **Performance Stats**: Average win/loss, largest win/loss, profit factor
- **Real-time Updates**: WebSocket connection for live data

### 2. Positions Management

- **View**: All open positions with live P&L
- **Close**: Manually close any position
- **Monitor**: Duration, entry price, current price, P&L percentage

### 3. Order Management

- **Active Orders**: View and cancel pending orders
- **Order History**: View past orders with status
- **Manual Trading**: Place new orders (implement in BotAPIWrapper)

### 4. Strategy Controls

- **Enable/Disable**: Toggle strategies on/off
- **Monitor**: View last signal time and status
- **Configure**: See current strategy parameters

### 5. Market Screener

- **Top Opportunities**: Real-time market scanning results
- **Signals**: Detected patterns (BREAKOUT, SUPPORT, etc.)
- **Volume**: 24h trading volume
- **Price Change**: 24h percentage change

### 6. Signals History

- **Recent Signals**: All strategy-generated signals
- **Execution Status**: Whether signal was executed
- **Reason**: Why the signal was generated
- **Timestamp**: When the signal occurred

## Troubleshooting

### Port 8088 Already in Use

```bash
# Find what's using the port
lsof -i :8088  # Mac/Linux
netstat -ano | findstr :8088  # Windows

# Kill the process or change the port in docker-compose.yml
```

### Database Connection Error

```bash
# Check if postgres is running
docker-compose ps

# View postgres logs
docker-compose logs postgres

# Restart postgres
docker-compose restart postgres
```

### Frontend Not Loading

```bash
# Ensure frontend was built
cd web && npm run build

# Check if dist/ directory exists
ls web/dist/

# Rebuild Docker image
docker-compose up --build
```

### WebSocket Not Connecting

1. Check browser console for errors
2. Ensure backend is running: `docker-compose ps`
3. Check WebSocket endpoint: `ws://localhost:8088/ws`
4. View backend logs: `docker-compose logs trading-bot`

### API Credentials Invalid

1. Verify your `.env` file has correct keys
2. For testnet: Get keys from https://testnet.binance.vision/
3. Restart services: `docker-compose restart`

## Production Considerations

### Security

1. **Change Database Password**:
   ```yaml
   # In docker-compose.yml
   POSTGRES_PASSWORD=your_strong_password
   DB_PASSWORD=your_strong_password
   ```

2. **Use Environment Variables**: Never commit API keys
   ```bash
   # Create .env file (gitignored)
   BINANCE_API_KEY=real_key
   BINANCE_SECRET_KEY=real_secret
   ```

3. **Enable HTTPS**: Use reverse proxy (nginx) with SSL
4. **Firewall**: Only expose necessary ports
5. **Authentication**: Add login system (not implemented)

### Performance

1. **Database Backups**:
   ```bash
   # Backup
   docker-compose exec postgres pg_dump -U trading_bot trading_bot > backup.sql

   # Restore
   docker-compose exec -T postgres psql -U trading_bot trading_bot < backup.sql
   ```

2. **Log Rotation**: Already configured in docker-compose.yml
3. **Resource Limits**: Add to docker-compose.yml:
   ```yaml
   deploy:
     resources:
       limits:
         memory: 512M
   ```

### Monitoring

1. **Health Check**:
   ```bash
   curl http://localhost:8088/health
   ```

2. **Metrics Endpoint**:
   ```bash
   curl http://localhost:8088/api/metrics
   ```

3. **Prometheus** (optional): Uncomment in docker-compose.yml

## API Endpoints

### REST API

```
GET  /health                      - Health check
GET  /api/bot/status              - Bot status
GET  /api/positions               - Open positions
GET  /api/positions/history       - Closed positions
POST /api/positions/:symbol/close - Close position
GET  /api/orders                  - Active orders
GET  /api/orders/history          - Order history
POST /api/orders                  - Place order
DELETE /api/orders/:id            - Cancel order
GET  /api/strategies              - List strategies
PUT  /api/strategies/:name/toggle - Toggle strategy
GET  /api/signals                 - Recent signals
GET  /api/screener/results        - Screener results
GET  /api/metrics                 - Trading metrics
GET  /api/events                  - System events
```

### WebSocket

```
ws://localhost:8088/ws

Events:
- CONNECTED
- TRADE_OPENED
- TRADE_CLOSED
- ORDER_PLACED
- ORDER_FILLED
- SIGNAL_GENERATED
- POSITION_UPDATE
- PRICE_UPDATE
- SCREENER_UPDATE
```

## Next Steps

1. **Implement Manual Trading**: Complete the BotAPIWrapper methods in main.go
2. **Add Charts**: Integrate TradingView Lightweight Charts for price history
3. **Add Authentication**: Implement login system for security
4. **Email Notifications**: Add email alerts for important events
5. **Mobile Responsive**: Test and optimize for mobile devices
6. **Backtesting**: Add historical data analysis
7. **Performance Graphs**: Add P&L charts over time

## Support

- Check logs: `docker-compose logs -f`
- View database: `docker-compose exec postgres psql -U trading_bot`
- Restart services: `docker-compose restart`
- Full reset: `docker-compose down -v && docker-compose up --build`

---

**Remember**: Start in testnet mode and thoroughly test before going live!

Your trading bot dashboard is ready at **http://localhost:8088** 🚀
