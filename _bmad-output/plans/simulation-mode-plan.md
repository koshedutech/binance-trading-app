# Simulation Mode Implementation Plan

**Status**: PLANNED (Not Yet Implemented)
**Priority**: Next Sprint
**Created**: 2026-01-29

---

## Overview

Simulation mode allows testing strategies on **REAL live market data** without executing real trades or using real money. This is different from Binance Testnet (paper trading API) - we use the real Binance API for data but only simulate trade execution internally.

---

## Key Requirements

1. **Per-Strategy Toggle**: Each strategy can be in "Live" or "Simulation" mode independently
2. **Real Market Data**: Uses actual live candle data from Binance production API
3. **Simulated Execution**: No real orders placed - trades are simulated internally
4. **Separate Data Storage**: Simulation trades stored in separate tables (not mixed with real trades)
5. **Full Tracking**: Track simulated P&L, win rate, equity curve, fees (calculated, not real)

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         SIMULATION MODE FLOW                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   BINANCE API (Real)                                                    │
│        │                                                                │
│        ▼                                                                │
│   ┌─────────────┐                                                       │
│   │ Live Candle │────────────────────────────────────────┐              │
│   │   Stream    │                                        │              │
│   └─────────────┘                                        │              │
│        │                                                 │              │
│        ▼                                                 ▼              │
│   ┌───────────────────────┐                  ┌───────────────────────┐  │
│   │   LIVE MODE           │                  │   SIMULATION MODE     │  │
│   │                       │                  │                       │  │
│   │ • Pattern Detection   │                  │ • Pattern Detection   │  │
│   │ • Real Order Exec     │                  │ • Simulated Order Exec│  │
│   │ • Real P&L Tracking   │                  │ • Simulated P&L       │  │
│   │ • Real Binance Fees   │                  │ • Calculated Fees     │  │
│   └───────────────────────┘                  └───────────────────────┘  │
│        │                                                 │              │
│        ▼                                                 ▼              │
│   ┌───────────────────────┐                  ┌───────────────────────┐  │
│   │ trades table          │                  │ simulation_trades     │  │
│   │ (Real trades)         │                  │ (Simulated trades)    │  │
│   └───────────────────────┘                  └───────────────────────┘  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Database Schema (New Tables)

### Table: `simulation_sessions`
Tracks simulation sessions per strategy.

```sql
CREATE TABLE simulation_sessions (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    strategy_id VARCHAR(100) NOT NULL,          -- e.g., "ravindra_volume_imbalance"
    strategy_mode VARCHAR(50) NOT NULL,          -- e.g., "scalp"

    -- Session Config
    initial_budget DECIMAL(18,8) NOT NULL,       -- Starting budget
    current_equity DECIMAL(18,8) NOT NULL,       -- Current equity
    leverage INTEGER NOT NULL DEFAULT 10,

    -- Status
    status VARCHAR(20) NOT NULL DEFAULT 'active', -- active, paused, completed
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMP,

    -- Symbols being simulated
    symbols TEXT[] NOT NULL,                      -- e.g., ['DOGEUSDT', 'NEARUSDT', 'ETHUSDT']
    timeframe VARCHAR(10) NOT NULL,              -- e.g., "3m"

    -- Stats (updated after each trade)
    total_trades INTEGER DEFAULT 0,
    winning_trades INTEGER DEFAULT 0,
    losing_trades INTEGER DEFAULT 0,
    total_pnl DECIMAL(18,8) DEFAULT 0,
    total_fees DECIMAL(18,8) DEFAULT 0,
    max_drawdown_pct DECIMAL(10,4) DEFAULT 0,

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sim_sessions_user ON simulation_sessions(user_id);
CREATE INDEX idx_sim_sessions_strategy ON simulation_sessions(strategy_id);
CREATE INDEX idx_sim_sessions_status ON simulation_sessions(status);
```

### Table: `simulation_trades`
Stores individual simulated trades.

```sql
CREATE TABLE simulation_trades (
    id SERIAL PRIMARY KEY,
    session_id INTEGER NOT NULL REFERENCES simulation_sessions(id),

    -- Trade Identity
    symbol VARCHAR(20) NOT NULL,
    trade_number INTEGER NOT NULL,               -- Sequential trade # in session

    -- Entry
    entry_time TIMESTAMP NOT NULL,
    entry_price DECIMAL(18,8) NOT NULL,
    quantity DECIMAL(18,8) NOT NULL,
    position_value DECIMAL(18,8) NOT NULL,       -- quantity * entry_price

    -- Risk Management
    stop_loss DECIMAL(18,8) NOT NULL,
    take_profit DECIMAL(18,8) NOT NULL,
    risk_percent DECIMAL(10,4) NOT NULL,

    -- Exit (NULL while position open)
    exit_time TIMESTAMP,
    exit_price DECIMAL(18,8),
    exit_reason VARCHAR(50),                     -- 'take_profit', 'stop_loss', 'trailing_be', 'trailing_1r'

    -- P&L Calculation
    gross_pnl DECIMAL(18,8),
    entry_fee DECIMAL(18,8),
    exit_fee DECIMAL(18,8),
    total_fees DECIMAL(18,8),
    net_pnl DECIMAL(18,8),
    net_pnl_percent DECIMAL(10,4),

    -- Equity Tracking
    equity_before DECIMAL(18,8) NOT NULL,
    equity_after DECIMAL(18,8),

    -- Pattern Data (for analysis)
    pattern_data JSONB,                          -- Reference candle, consolidation data, etc.

    -- Status
    status VARCHAR(20) NOT NULL DEFAULT 'open',  -- open, closed

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sim_trades_session ON simulation_trades(session_id);
CREATE INDEX idx_sim_trades_symbol ON simulation_trades(symbol);
CREATE INDEX idx_sim_trades_status ON simulation_trades(status);
CREATE INDEX idx_sim_trades_entry ON simulation_trades(entry_time);
```

---

## Settings Structure

Add to `default-settings.json` under each strategy:

```json
"simulation": {
    "enabled": false,
    "mode": "simulation",          // "live" or "simulation"
    "initial_budget_usd": 100,
    "calculate_fees": true,
    "taker_fee_percent": 0.05,
    "symbols": ["DOGEUSDT", "NEARUSDT", "ETHUSDT"],
    "auto_start": false            // Start simulation when strategy enabled
}
```

---

## UI Components Needed

### 1. Strategy Card Toggle
```
┌─────────────────────────────────────────────────────────────────┐
│ Ravindra Volume Imbalance                          [Simulation] │
│                                                    ────────────│
│                                                    ○ Live      │
│ Timeframe: 3m                                      ● Simulation│
│ Symbols: DOGE, NEAR, ETH                                       │
└─────────────────────────────────────────────────────────────────┘
```

### 2. Simulation Dashboard
```
┌─────────────────────────────────────────────────────────────────┐
│ SIMULATION: Ravindra Volume Imbalance                   [Stop]  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   Started: Jan 29, 2026 10:30 AM        Running: 2h 15m        │
│                                                                 │
│   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│   │ Initial     │  │ Current     │  │ Net P&L     │            │
│   │ $100.00     │  │ $147.50     │  │ +$47.50     │            │
│   │             │  │             │  │ +47.5%      │            │
│   └─────────────┘  └─────────────┘  └─────────────┘            │
│                                                                 │
│   Trades: 5  |  Wins: 3  |  Losses: 2  |  Win Rate: 60%       │
│   Total Fees: $2.35  |  Profit Factor: 2.8                     │
│                                                                 │
│   CURRENT POSITION: DOGEUSDT (LONG)                            │
│   Entry: $0.1252  |  SL: $0.1238  |  TP: $0.1304               │
│   Unrealized P&L: +$12.40 (+4.2%)                              │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 3. Simulation Trade History
```
┌─────────────────────────────────────────────────────────────────┐
│ SIMULATION TRADE HISTORY                                        │
├───────┬────────┬─────────┬────────┬────────┬────────┬──────────┤
│ #     │ Symbol │ Entry   │ Exit   │ P&L    │ Fees   │ Status   │
├───────┼────────┼─────────┼────────┼────────┼────────┼──────────┤
│ 5     │ DOGE   │ $0.1252 │ -      │ +$12.4 │ $0.52  │ Open     │
│ 4     │ NEAR   │ $1.5200 │ $1.5840│ +$42.1 │ $0.45  │ TP Hit   │
│ 3     │ DOGE   │ $0.1189 │ $0.1172│ -$14.3 │ $0.48  │ SL Hit   │
│ 2     │ ETH    │ $3012   │ $3052  │ +$13.3 │ $0.41  │ Trail+1  │
│ 1     │ NEAR   │ $1.4850 │ $1.4620│ -$15.5 │ $0.49  │ SL Hit   │
└───────┴────────┴─────────┴────────┴────────┴────────┴──────────┘
```

---

## Backend Implementation Steps

### Phase 1: Database & Models
1. Create migration for `simulation_sessions` and `simulation_trades` tables
2. Create Go models and repository methods
3. Add simulation toggle to strategy settings reader

### Phase 2: Simulation Engine
1. Create `SimulationEngine` that wraps strategy detection
2. Implement simulated order execution (no real API calls)
3. Track position state, trailing stops, exit conditions
4. Calculate fees (0.05% × 2 per trade)

### Phase 3: Real-Time Processing
1. Subscribe to live candle updates (same as live mode)
2. Run pattern detection on each candle
3. Execute simulated entries/exits based on price action
4. Update `simulation_trades` table in real-time

### Phase 4: API & WebSocket
1. Add API endpoints for simulation CRUD
2. Add WebSocket events for simulation updates
3. Broadcast simulation trade events to frontend

### Phase 5: Frontend
1. Add simulation toggle to strategy cards
2. Create simulation dashboard component
3. Create simulation trade history table
4. Add real-time updates via WebSocket

---

## API Endpoints

```
POST   /api/simulation/start          Start simulation for a strategy
POST   /api/simulation/stop           Stop simulation
GET    /api/simulation/sessions       List simulation sessions
GET    /api/simulation/session/:id    Get session details
GET    /api/simulation/trades/:id     Get trades for session
DELETE /api/simulation/session/:id    Delete session and trades
```

---

## WebSocket Events

```javascript
// New simulation trade opened
{
    "type": "SIMULATION_TRADE_OPENED",
    "data": {
        "session_id": 1,
        "trade_id": 5,
        "symbol": "DOGEUSDT",
        "entry_price": 0.1252,
        "stop_loss": 0.1238,
        "take_profit": 0.1304,
        "equity_before": 135.10
    }
}

// Simulation trade closed
{
    "type": "SIMULATION_TRADE_CLOSED",
    "data": {
        "session_id": 1,
        "trade_id": 5,
        "exit_price": 0.1304,
        "exit_reason": "take_profit",
        "net_pnl": 52.16,
        "equity_after": 187.26
    }
}

// Simulation equity update
{
    "type": "SIMULATION_EQUITY_UPDATE",
    "data": {
        "session_id": 1,
        "current_equity": 147.50,
        "unrealized_pnl": 12.40
    }
}
```

---

## Estimated Effort

| Component | Effort |
|-----------|--------|
| Database migration | 1 hour |
| Go models & repository | 2 hours |
| Simulation engine | 4 hours |
| API endpoints | 2 hours |
| WebSocket events | 1 hour |
| Frontend components | 4 hours |
| Testing | 2 hours |
| **Total** | **~16 hours** |

---

## Notes

- This is NOT Binance Testnet/Paper Trading API
- Uses REAL live market data from Binance production
- Only trade execution is simulated (no real orders)
- Fees are calculated (0.05% taker × 2 = 0.1% per trade)
- Data stored in separate tables - never mixed with real trades
- Can run simulation and live mode on different strategies simultaneously
