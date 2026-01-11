# Epic 8: Daily Settlement & Mode Analytics

## Epic Overview

**Goal:** Implement end-of-day settlement processing that captures comprehensive daily summaries by trading mode, enabling mode performance analysis, billing/subscription calculations, and historical reporting.

**Business Value:**
- Mode-wise performance tracking for strategy optimization
- Daily P&L data for admin billing and profit-share calculations
- Historical data persistence beyond Binance's 90-day limit
- Capital utilization metrics for risk management

**Priority:** MEDIUM-HIGH - Analytics and billing foundation

**Estimated Complexity:** MEDIUM

**Depends On:**
- Epic 6 (Redis - for runtime state)
- Epic 7 (Client Order ID - for mode identification)

---

## Problem Statement

### Current Issues

| Issue | Severity | Impact |
|-------|----------|--------|
| **No mode-wise P&L tracking** | HIGH | Cannot compare mode performance |
| **No daily settlement process** | HIGH | No historical aggregates |
| **Open positions not snapshot** | MEDIUM | Daily P&L inaccurate |
| **Binance 90-day data limit** | MEDIUM | Historical data lost |
| **No admin billing data** | HIGH | Cannot calculate profit share |
| **Capital utilization unknown** | MEDIUM | Risk management blind spot |

### Current State

```
CURRENT STATE:
┌─────────────────────────────────────────────────────────────────┐
│ DAILY P&L CALCULATION:                                          │
│                                                                 │
│  - Relies on real-time Binance queries                          │
│  - No historical aggregation                                    │
│  - Mode not tracked (before Epic 7)                             │
│  - Open positions mark-to-market not captured                   │
│  - Data lost after 90 days                                      │
│                                                                 │
│ ADMIN BILLING:                                                  │
│                                                                 │
│  - No automated profit calculation                              │
│  - Manual reconciliation required                               │
│  - No daily breakdown available                                 │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Target State

### Daily Settlement Architecture

```
TARGET STATE:
┌─────────────────────────────────────────────────────────────────┐
│ END-OF-DAY SETTLEMENT PROCESS (User's Timezone Midnight)        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Step 1: SNAPSHOT OPEN POSITIONS                                │
│  ├─ Mark-to-market each open position                           │
│  ├─ Record unrealized P&L                                       │
│  ├─ Extract mode from clientOrderId (Epic 7)                    │
│                                                                 │
│  Step 2: AGGREGATE CLOSED TRADES                                │
│  ├─ Fetch all trades closed today from Binance                  │
│  ├─ Parse mode from clientOrderId                               │
│  ├─ Sum realized P&L by mode                                    │
│  ├─ Count wins/losses by mode                                   │
│                                                                 │
│  Step 3: CALCULATE FEES                                         │
│  ├─ Commission fees from trades                                 │
│  ├─ Funding fees from Binance income API                        │
│                                                                 │
│  Step 4: CALCULATE CAPITAL METRICS                              │
│  ├─ Max capital used during day                                 │
│  ├─ Average capital utilization                                 │
│                                                                 │
│  Step 5: STORE DAILY SUMMARY                                    │
│  ├─ One row per mode per user per day                           │
│  ├─ Plus one "ALL" row for totals                               │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Daily Summary Schema

```sql
CREATE TABLE daily_mode_summaries (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    summary_date DATE NOT NULL,
    timezone VARCHAR(50) NOT NULL,

    -- Mode identification (ULT, SCA, SWI, POS, ALL)
    mode VARCHAR(10) NOT NULL,

    -- Trade metrics
    trade_count INT DEFAULT 0,
    win_count INT DEFAULT 0,
    loss_count INT DEFAULT 0,
    win_rate DECIMAL(5,2),

    -- P&L metrics (in USDT)
    realized_pnl DECIMAL(18,8) DEFAULT 0,
    unrealized_pnl DECIMAL(18,8) DEFAULT 0,
    total_pnl DECIMAL(18,8) DEFAULT 0,

    -- Volume metrics
    total_volume_usdt DECIMAL(18,8) DEFAULT 0,
    avg_trade_size DECIMAL(18,8) DEFAULT 0,
    largest_win DECIMAL(18,8) DEFAULT 0,
    largest_loss DECIMAL(18,8) DEFAULT 0,

    -- Capital metrics
    starting_balance DECIMAL(18,8) DEFAULT 0,
    ending_balance DECIMAL(18,8) DEFAULT 0,
    max_capital_used DECIMAL(18,8) DEFAULT 0,
    avg_capital_used DECIMAL(18,8) DEFAULT 0,
    max_drawdown DECIMAL(18,8) DEFAULT 0,

    -- Fee metrics
    total_fees DECIMAL(18,8) DEFAULT 0,
    commission_fees DECIMAL(18,8) DEFAULT 0,
    funding_fees DECIMAL(18,8) DEFAULT 0,

    -- Position snapshot at EOD
    open_positions_count INT DEFAULT 0,
    open_positions_value DECIMAL(18,8) DEFAULT 0,

    -- Metadata
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    settlement_time TIMESTAMP,
    settlement_status VARCHAR(20) DEFAULT 'completed',  -- completed, failed, retrying
    settlement_error TEXT,  -- Error message if failed

    UNIQUE(user_id, summary_date, mode)
);

-- Indexes for fast queries
CREATE INDEX idx_daily_summaries_user_date
    ON daily_mode_summaries(user_id, summary_date DESC);
CREATE INDEX idx_daily_summaries_mode
    ON daily_mode_summaries(mode, summary_date DESC);
CREATE INDEX idx_daily_summaries_date_range
    ON daily_mode_summaries(summary_date, user_id);
```

---

## Requirements Traceability

### Functional Requirements

| ID | Requirement | Stories |
|----|-------------|---------|
| FR-0 | User timezone and settlement tracking in database | 8.0 |
| FR-1 | EOD snapshot of open positions with mark-to-market | 8.1 |
| FR-2 | Daily P&L aggregation by trading mode | 8.2 |
| FR-3 | Store daily summaries in database | 8.3 |
| FR-4 | Handle open positions in daily P&L (like Binance) | 8.4 |
| FR-5 | Admin dashboard showing all users' daily summaries | 8.5 |
| FR-6 | Historical reports with date range queries | 8.6 |
| FR-7 | Capital utilization tracking (max used per day) | 8.7 |
| FR-8 | Automatic retry for failed settlements | 8.8 |
| FR-9 | Admin monitoring and alerting for settlement failures | 8.9 |
| FR-10 | Data quality validation with anomaly detection | 8.10 |

### Non-Functional Requirements

| ID | Requirement | Stories |
|----|-------------|---------|
| NFR-1 | Settlement completes within 5 minutes per user | 8.1-8.4 |
| NFR-2 | Historical queries return within 2 seconds | 8.6 |
| NFR-3 | Data retained indefinitely (no 90-day limit) | 8.3 |
| NFR-4 | Settlement resilient to partial failures | 8.8 |
| NFR-5 | Settlement failure recovery with exponential backoff | 8.8 |
| NFR-6 | Admin alerts sent within 1 hour of persistent failure | 8.9 |
| NFR-7 | Data quality validation catches >95% of anomalies | 8.10 |

---

## Stories

### Story 8.0: User Timezone Database Migration

**Goal:** Add user timezone and settlement tracking columns to users table.

**Acceptance Criteria:**
- [ ] Migration adds `timezone VARCHAR(50) DEFAULT 'Asia/Kolkata'` to users table
- [ ] Migration adds `last_settlement_date DATE` to users table
- [ ] Migration is idempotent (checks if columns exist before adding)
- [ ] Default timezone is 'Asia/Kolkata' for existing users
- [ ] Migration includes rollback script
- [ ] Test migration on development database first

**Technical Notes:**
```sql
-- Migration: 20260106_add_user_timezone_settlement.sql

-- Check and add timezone column
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'users' AND column_name = 'timezone'
    ) THEN
        ALTER TABLE users ADD COLUMN timezone VARCHAR(50) DEFAULT 'Asia/Kolkata';
    END IF;
END $$;

-- Check and add last_settlement_date column
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'users' AND column_name = 'last_settlement_date'
    ) THEN
        ALTER TABLE users ADD COLUMN last_settlement_date DATE;
    END IF;
END $$;

-- Index for efficient settlement queries
CREATE INDEX IF NOT EXISTS idx_users_last_settlement
    ON users(last_settlement_date);

COMMENT ON COLUMN users.timezone IS 'IANA timezone (e.g., Asia/Kolkata, America/New_York) for daily settlement scheduling';
COMMENT ON COLUMN users.last_settlement_date IS 'Last date for which settlement was completed';

-- Rollback script:
-- ALTER TABLE users DROP COLUMN IF EXISTS timezone;
-- ALTER TABLE users DROP COLUMN IF EXISTS last_settlement_date;
-- DROP INDEX IF EXISTS idx_users_last_settlement;
```

**Migration Verification:**
```go
// internal/database/migrations/verify_user_timezone.go
func (db *Service) VerifyUserTimezoneMigration() error {
    var count int
    err := db.QueryRow(`
        SELECT COUNT(*)
        FROM information_schema.columns
        WHERE table_name = 'users'
        AND column_name IN ('timezone', 'last_settlement_date')
    `).Scan(&count)

    if err != nil || count != 2 {
        return fmt.Errorf("user timezone migration not applied correctly")
    }
    return nil
}
```

---

### Story 8.1: EOD Snapshot of Open Positions

**Goal:** Capture mark-to-market value of all open positions at end of day.

**Acceptance Criteria:**
- [ ] Scheduled job runs at user's timezone midnight
- [ ] For each open position:
  - Fetch current mark price from Binance
  - Calculate unrealized P&L
  - Extract mode from position's clientOrderId
- [ ] Store snapshot in `daily_position_snapshots` table
- [ ] Handle positions without clientOrderId (legacy) as "UNKNOWN" mode
- [ ] Graceful handling if Binance API unavailable

**Technical Notes:**
```go
// internal/settlement/position_snapshot.go
type PositionSnapshot struct {
    UserID         string
    SnapshotDate   time.Time
    Symbol         string
    Side           string      // LONG/SHORT
    Mode           string      // ULT/SCA/SWI/POS/UNKNOWN
    Quantity       float64
    EntryPrice     float64
    MarkPrice      float64
    UnrealizedPnl  float64
    Leverage       int
    ChainId        string      // From clientOrderId
}

func (s *SettlementService) SnapshotOpenPositions(userID string, asOf time.Time) ([]PositionSnapshot, error) {
    positions, err := s.binance.GetPositionRisk(userID)
    if err != nil {
        return nil, err
    }

    snapshots := make([]PositionSnapshot, 0)
    for _, pos := range positions {
        if pos.PositionAmt == 0 {
            continue // Skip closed positions
        }

        // Find related order to get mode
        mode := s.extractModeFromPosition(userID, pos)

        snapshots = append(snapshots, PositionSnapshot{
            UserID:        userID,
            SnapshotDate:  asOf,
            Symbol:        pos.Symbol,
            Side:          pos.PositionSide,
            Mode:          mode,
            Quantity:      pos.PositionAmt,
            EntryPrice:    pos.EntryPrice,
            MarkPrice:     pos.MarkPrice,
            UnrealizedPnl: pos.UnRealizedProfit,
            Leverage:      pos.Leverage,
        })
    }

    return snapshots, nil
}
```

---

### Story 8.2: Daily P&L Aggregation by Mode

**Goal:** Sum realized P&L from closed trades grouped by trading mode.

**Acceptance Criteria:**
- [ ] Fetch all trades closed during the day from Binance
- [ ] Parse clientOrderId to extract mode
- [ ] Aggregate by mode:
  - Total realized P&L
  - Trade count
  - Win count / Loss count
  - Win rate calculation
  - Largest win / Largest loss
- [ ] Handle trades without mode as "UNKNOWN"
- [ ] Calculate "ALL" totals across all modes

**Technical Notes:**
```go
// internal/settlement/pnl_aggregator.go
type ModePnL struct {
    Mode         string
    RealizedPnl  float64
    TradeCount   int
    WinCount     int
    LossCount    int
    WinRate      float64
    TotalVolume  float64
    LargestWin   float64
    LargestLoss  float64
}

func (s *SettlementService) AggregateDailyPnL(userID string, date time.Time) (map[string]*ModePnL, error) {
    // Fetch trades from Binance for the date range
    startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
    endOfDay := startOfDay.Add(24 * time.Hour)

    trades, err := s.binance.GetUserTrades(userID, startOfDay, endOfDay)
    if err != nil {
        return nil, err
    }

    // Aggregate by mode
    results := make(map[string]*ModePnL)

    for _, trade := range trades {
        parsed := orders.ParseClientOrderId(trade.ClientOrderId)
        mode := "UNKNOWN"
        if parsed != nil {
            mode = string(parsed.Mode)
        }

        if results[mode] == nil {
            results[mode] = &ModePnL{Mode: mode}
        }

        mp := results[mode]
        mp.TradeCount++
        mp.RealizedPnl += trade.RealizedPnl
        mp.TotalVolume += trade.QuoteQty

        if trade.RealizedPnl > 0 {
            mp.WinCount++
            if trade.RealizedPnl > mp.LargestWin {
                mp.LargestWin = trade.RealizedPnl
            }
        } else if trade.RealizedPnl < 0 {
            mp.LossCount++
            if trade.RealizedPnl < mp.LargestLoss {
                mp.LargestLoss = trade.RealizedPnl
            }
        }
    }

    // Calculate win rates
    for _, mp := range results {
        if mp.TradeCount > 0 {
            mp.WinRate = float64(mp.WinCount) / float64(mp.TradeCount) * 100
        }
    }

    // Calculate "ALL" totals
    results["ALL"] = s.sumAllModes(results)

    return results, nil
}
```

---

### Story 8.3: Daily Summary Storage

**Goal:** Persist daily settlement data to database.

**Acceptance Criteria:**
- [ ] Create `daily_mode_summaries` table (schema above)
- [ ] Settlement service writes one row per mode per day
- [ ] Upsert logic: Update if already exists for same date/mode
- [ ] Store settlement timestamp
- [ ] Store user's timezone for reference
- [ ] Index for fast queries by user/date/mode

**API Endpoints:**
```
GET  /api/user/daily-summaries
     ?start_date=2026-01-01
     &end_date=2026-01-31
     &mode=ULT (optional)

Response:
{
  "summaries": [
    {
      "date": "2026-01-06",
      "mode": "ULT",
      "trade_count": 15,
      "win_rate": 66.67,
      "realized_pnl": 245.50,
      "unrealized_pnl": 50.00,
      "total_pnl": 295.50,
      ...
    }
  ],
  "totals": {
    "trade_count": 45,
    "realized_pnl": 720.00,
    ...
  }
}
```

---

### Story 8.4: Handle Open Positions in Daily P&L

**Goal:** Calculate daily P&L correctly with open position mark-to-market.

**Acceptance Criteria:**
- [ ] Daily P&L = Realized P&L + Change in Unrealized P&L
- [ ] Compare today's unrealized vs yesterday's unrealized
- [ ] Match Binance's daily P&L calculation method
- [ ] Handle new positions opened today (no yesterday unrealized)
- [ ] Handle positions closed today (subtract yesterday's unrealized)

**Calculation Logic:**
```
Daily P&L Calculation (matches Binance):

realized_pnl = Sum of all closed trade P&L today

unrealized_change =
    today's unrealized P&L (snapshot at EOD)
  - yesterday's unrealized P&L (from yesterday's snapshot)

total_daily_pnl = realized_pnl + unrealized_change

Example:
- Yesterday EOD: Open BTCUSDT LONG, unrealized = +$100
- Today:
  - Closed 2 trades, realized = +$50
  - Still holding BTCUSDT, unrealized = +$150
- Daily P&L = $50 (realized) + ($150 - $100) (unrealized change) = $100
```

---

### Story 8.5: Admin Dashboard for Daily Summaries

**Goal:** Admin view to see all users' daily performance for billing.

**Acceptance Criteria:**
- [ ] Admin-only endpoint: `GET /api/admin/daily-summaries/all`
- [ ] List all users' daily summaries
- [ ] Filters: Date range, user, mode
- [ ] Sortable by P&L, trade count
- [ ] Export to CSV for billing
- [ ] Aggregate totals per user for profit-share calculation

**Admin Dashboard Features:**
```
┌──────────────────────────────────────────────────────────────────┐
│ Admin Dashboard: Daily Settlements                               │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│ Date Range: [2026-01-01] to [2026-01-31]   [Export CSV]          │
│                                                                  │
│ ┌──────────────────────────────────────────────────────────────┐ │
│ │ User             │ Trades │ P&L      │ Win Rate │ Fees      │ │
│ │ ────────────────────────────────────────────────────────────│ │
│ │ user1@email.com  │ 245    │ +$1,250  │ 62%      │ $125      │ │
│ │ user2@email.com  │ 89     │ -$320    │ 45%      │ $45       │ │
│ │ user3@email.com  │ 567    │ +$3,400  │ 71%      │ $340      │ │
│ │ ────────────────────────────────────────────────────────────│ │
│ │ TOTALS           │ 901    │ +$4,330  │ 59%      │ $510      │ │
│ └──────────────────────────────────────────────────────────────┘ │
│                                                                  │
│ Click user row to see mode breakdown                             │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

---

### Story 8.6: Historical Reports with Date Range Queries

**Goal:** Enable querying historical daily summaries for any date range.

**Acceptance Criteria:**
- [ ] Query by date range (up to 1 year)
- [ ] Aggregate multiple days into period summary
- [ ] Weekly, monthly, yearly rollups
- [ ] Mode comparison over time
- [ ] Performance graphs data (P&L trend, win rate trend)
- [ ] Efficient queries with proper indexes

**API Endpoints:**
```
GET /api/user/performance/summary
    ?period=weekly|monthly|yearly
    &start_date=2026-01-01
    &end_date=2026-12-31

GET /api/user/performance/by-mode
    ?start_date=2026-01-01
    &end_date=2026-01-31

GET /api/user/performance/trend
    ?metric=pnl|win_rate|trade_count
    &granularity=daily|weekly|monthly
    &start_date=2026-01-01
    &end_date=2026-01-31
```

---

### Story 8.7: Capital Utilization Tracking

**Goal:** Track maximum and average capital used each day.

**Acceptance Criteria:**
- [ ] Sample capital usage periodically (every 5 minutes)
- [ ] Store max capital used during day
- [ ] Calculate average capital utilization
- [ ] Track max drawdown from daily high
- [ ] Include in daily summary
- [ ] Used for risk monitoring and billing tier determination

**Technical Notes:**
```go
// Capital sampling during the day
type CapitalSample struct {
    Timestamp     time.Time
    TotalBalance  float64  // Wallet balance
    UsedMargin    float64  // In positions
    AvailableMargin float64
    UnrealizedPnl float64
}

// EOD aggregation
type CapitalMetrics struct {
    StartingBalance float64  // First sample of day
    EndingBalance   float64  // Last sample of day
    MaxCapitalUsed  float64  // Highest used margin
    AvgCapitalUsed  float64  // Average of samples
    MaxDrawdown     float64  // Largest unrealized loss
    PeakBalance     float64  // Highest balance during day
}
```

---

### Story 8.8: Settlement Failure Recovery

**Goal:** Implement robust error handling and retry mechanisms for settlement failures.

**Acceptance Criteria:**
- [ ] Binance API failures: Retry 3 times with exponential backoff (5s, 15s, 45s)
- [ ] Database failures: Rollback transaction, retry once after 10 seconds
- [ ] Partial data scenarios: Mark settlement as 'failed', store error details
- [ ] Admin endpoint: `POST /api/admin/settlements/retry/:user_id/:date`
- [ ] Settlement status tracked: 'completed', 'failed', 'retrying'
- [ ] Alert admin if settlement fails for >1 hour
- [ ] Failed settlements visible in admin dashboard

**Technical Notes:**
```go
// internal/settlement/error_handling.go
type SettlementError struct {
    UserID    string
    Date      time.Time
    Phase     string      // "snapshot", "aggregate", "store"
    Attempt   int
    Error     error
    Timestamp time.Time
}

func (s *SettlementService) RunDailySettlementWithRetry(userID string, date time.Time) error {
    maxRetries := 3
    backoff := []time.Duration{5 * time.Second, 15 * time.Second, 45 * time.Second}

    for attempt := 0; attempt < maxRetries; attempt++ {
        err := s.runSettlement(userID, date)
        if err == nil {
            return nil
        }

        // Log error
        s.logSettlementError(SettlementError{
            UserID:    userID,
            Date:      date,
            Phase:     s.identifyErrorPhase(err),
            Attempt:   attempt + 1,
            Error:     err,
            Timestamp: time.Now(),
        })

        // Check if retryable
        if !s.isRetryableError(err) {
            s.markSettlementFailed(userID, date, err)
            return err
        }

        // Wait before retry
        if attempt < maxRetries-1 {
            time.Sleep(backoff[attempt])
        }
    }

    // All retries exhausted
    s.markSettlementFailed(userID, date, errors.New("max retries exceeded"))
    s.alertAdmin(userID, date)
    return errors.New("settlement failed after retries")
}

func (s *SettlementService) isRetryableError(err error) bool {
    // Binance rate limit, timeout, connection errors
    if strings.Contains(err.Error(), "rate limit") ||
       strings.Contains(err.Error(), "timeout") ||
       strings.Contains(err.Error(), "connection refused") {
        return true
    }

    // Database deadlock, connection errors
    if strings.Contains(err.Error(), "deadlock") ||
       strings.Contains(err.Error(), "connection") {
        return true
    }

    return false
}

func (s *SettlementService) markSettlementFailed(userID string, date time.Time, err error) {
    // Update daily_mode_summaries with failed status
    s.db.Exec(`
        UPDATE daily_mode_summaries
        SET settlement_status = 'failed',
            settlement_error = $1
        WHERE user_id = $2 AND summary_date = $3
    `, err.Error(), userID, date)
}
```

**Admin Retry Endpoint:**
```go
// internal/api/handlers_admin.go
func (h *AdminHandlers) RetrySettlement(w http.ResponseWriter, r *http.Request) {
    userID := chi.URLParam(r, "user_id")
    dateStr := chi.URLParam(r, "date")

    date, err := time.Parse("2006-01-02", dateStr)
    if err != nil {
        http.Error(w, "invalid date", http.StatusBadRequest)
        return
    }

    // Mark as retrying
    h.db.Exec(`
        UPDATE daily_mode_summaries
        SET settlement_status = 'retrying'
        WHERE user_id = $1 AND summary_date = $2
    `, userID, date)

    // Run settlement
    go h.settlement.RunDailySettlementWithRetry(userID, date)

    w.WriteHeader(http.StatusAccepted)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "retrying",
        "message": "Settlement retry initiated",
    })
}
```

---

### Story 8.9: Settlement Monitoring & Alerts

**Goal:** Admin dashboard and alerting for failed settlements.

**Acceptance Criteria:**
- [ ] Admin endpoint: `GET /api/admin/settlements/status`
- [ ] Returns list of all settlements with status breakdown
- [ ] Filter by status: all, failed, completed, retrying
- [ ] Show error details for failed settlements
- [ ] Manual retry button per failed settlement
- [ ] Email alert when settlement fails for >1 hour
- [ ] Metrics: success rate, average duration, failure count

**API Response:**
```json
GET /api/admin/settlements/status?status=failed

{
  "settlements": [
    {
      "user_id": "uuid-123",
      "user_email": "user@example.com",
      "date": "2026-01-05",
      "status": "failed",
      "error": "Binance API timeout after 3 retries",
      "last_attempt": "2026-01-06T00:15:00Z",
      "failed_since_hours": 2.5
    }
  ],
  "summary": {
    "total_settlements": 150,
    "completed": 145,
    "failed": 3,
    "retrying": 2,
    "success_rate": 96.67
  }
}
```

**Alert Logic:**
```go
// internal/settlement/monitoring.go
func (s *SettlementService) CheckForStalledSettlements() {
    // Run every 15 minutes
    ticker := time.NewTicker(15 * time.Minute)
    for range ticker.C {
        var failures []SettlementFailure
        s.db.Query(`
            SELECT user_id, summary_date, settlement_error, settlement_time
            FROM daily_mode_summaries
            WHERE settlement_status = 'failed'
            AND settlement_time < NOW() - INTERVAL '1 hour'
            AND alerted = false
        `).Scan(&failures)

        for _, failure := range failures {
            s.sendAdminAlert(failure)
            s.db.Exec(`
                UPDATE daily_mode_summaries
                SET alerted = true
                WHERE user_id = $1 AND summary_date = $2
            `, failure.UserID, failure.Date)
        }
    }
}

func (s *SettlementService) sendAdminAlert(failure SettlementFailure) {
    subject := fmt.Sprintf("Settlement Failed: %s - %s", failure.UserEmail, failure.Date)
    body := fmt.Sprintf(`
        Settlement failed and needs manual intervention:

        User: %s (%s)
        Date: %s
        Error: %s
        Failed Since: %s

        Retry: POST /api/admin/settlements/retry/%s/%s
    `, failure.UserEmail, failure.UserID, failure.Date, failure.Error,
       failure.FailedSince, failure.UserID, failure.Date)

    s.emailService.SendToAdmin(subject, body)
}
```

**Admin Dashboard Component:**
```tsx
// web/src/pages/AdminSettlementStatus.tsx
interface FailedSettlement {
  user_id: string;
  user_email: string;
  date: string;
  status: string;
  error: string;
  failed_since_hours: number;
}

function AdminSettlementStatus() {
  const [failures, setFailures] = useState<FailedSettlement[]>([]);

  const retrySettlement = async (userId: string, date: string) => {
    await fetch(`/api/admin/settlements/retry/${userId}/${date}`, {
      method: 'POST',
    });
    // Refresh list
    loadFailures();
  };

  return (
    <div>
      <h2>Failed Settlements</h2>
      <table>
        <thead>
          <tr>
            <th>User</th>
            <th>Date</th>
            <th>Error</th>
            <th>Failed Since</th>
            <th>Action</th>
          </tr>
        </thead>
        <tbody>
          {failures.map(f => (
            <tr key={`${f.user_id}-${f.date}`}>
              <td>{f.user_email}</td>
              <td>{f.date}</td>
              <td title={f.error}>{f.error.substring(0, 50)}...</td>
              <td>{f.failed_since_hours.toFixed(1)} hours</td>
              <td>
                <button onClick={() => retrySettlement(f.user_id, f.date)}>
                  Retry
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
```

---

### Story 8.10: Data Quality Validation

**Goal:** Validate settlement data integrity and flag anomalies.

**Acceptance Criteria:**
- [ ] Win rate validation: Must be between 0-100%
- [ ] Total P&L validation: Flag if outside -$10,000 to +$10,000 range (configurable)
- [ ] Trade count validation: Flag if >500 trades/day (suspicious)
- [ ] Unrealized P&L validation: Compare with Binance API snapshot
- [ ] Mark anomalies with `data_quality_flag` in database
- [ ] Admin review queue for flagged settlements
- [ ] Manual approval/rejection workflow

**Technical Notes:**
```go
// internal/settlement/validation.go
type ValidationResult struct {
    IsValid bool
    Errors  []string
    Warnings []string
}

func (s *SettlementService) ValidateSettlementData(summary *DailyModeSummary) ValidationResult {
    result := ValidationResult{IsValid: true}

    // Win rate validation
    if summary.WinRate < 0 || summary.WinRate > 100 {
        result.Errors = append(result.Errors,
            fmt.Sprintf("Invalid win rate: %.2f%%", summary.WinRate))
        result.IsValid = false
    }

    // P&L bounds check
    if summary.TotalPnl < -10000 || summary.TotalPnl > 10000 {
        result.Warnings = append(result.Warnings,
            fmt.Sprintf("P&L outside normal range: $%.2f", summary.TotalPnl))
    }

    // Trade count check
    if summary.TradeCount > 500 {
        result.Warnings = append(result.Warnings,
            fmt.Sprintf("High trade count: %d", summary.TradeCount))
    }

    // Unrealized P&L consistency
    if summary.UnrealizedPnl != 0 {
        // Fetch current unrealized from Binance for comparison
        binanceUnrealized, err := s.binance.GetUnrealizedPnl(summary.UserID)
        if err == nil {
            diff := math.Abs(summary.UnrealizedPnl - binanceUnrealized)
            if diff > 100 { // $100 tolerance
                result.Warnings = append(result.Warnings,
                    fmt.Sprintf("Unrealized P&L mismatch: Stored=%.2f, Binance=%.2f",
                        summary.UnrealizedPnl, binanceUnrealized))
            }
        }
    }

    // Win/loss count matches trade count
    if summary.WinCount + summary.LossCount != summary.TradeCount {
        result.Errors = append(result.Errors,
            "Win + Loss count doesn't match total trade count")
        result.IsValid = false
    }

    return result
}

func (s *SettlementService) runSettlement(userID string, date time.Time) error {
    // ... settlement logic ...

    // Validate before storing
    validation := s.ValidateSettlementData(summary)
    if !validation.IsValid {
        return fmt.Errorf("validation failed: %v", validation.Errors)
    }

    // Flag if warnings present
    if len(validation.Warnings) > 0 {
        summary.DataQualityFlag = true
        summary.DataQualityNotes = strings.Join(validation.Warnings, "; ")
    }

    // Store in database
    return s.db.SaveDailySummary(summary)
}
```

**Database Schema Update:**
```sql
-- Add to daily_mode_summaries table
ALTER TABLE daily_mode_summaries ADD COLUMN IF NOT EXISTS
    data_quality_flag BOOLEAN DEFAULT false;

ALTER TABLE daily_mode_summaries ADD COLUMN IF NOT EXISTS
    data_quality_notes TEXT;

ALTER TABLE daily_mode_summaries ADD COLUMN IF NOT EXISTS
    reviewed_by UUID REFERENCES users(id);

ALTER TABLE daily_mode_summaries ADD COLUMN IF NOT EXISTS
    reviewed_at TIMESTAMP;

-- Index for admin review queue
CREATE INDEX IF NOT EXISTS idx_daily_summaries_quality_review
    ON daily_mode_summaries(data_quality_flag, reviewed_at)
    WHERE data_quality_flag = true AND reviewed_at IS NULL;
```

**Admin Review Endpoint:**
```go
// GET /api/admin/settlements/review-queue
func (h *AdminHandlers) GetReviewQueue(w http.ResponseWriter, r *http.Request) {
    var flagged []DailyModeSummary
    h.db.Query(`
        SELECT * FROM daily_mode_summaries
        WHERE data_quality_flag = true
        AND reviewed_at IS NULL
        ORDER BY summary_date DESC
    `).Scan(&flagged)

    json.NewEncoder(w).Encode(flagged)
}

// POST /api/admin/settlements/approve/:id
func (h *AdminHandlers) ApproveSettlement(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    adminID := r.Context().Value("user_id").(string)

    h.db.Exec(`
        UPDATE daily_mode_summaries
        SET reviewed_by = $1, reviewed_at = NOW()
        WHERE id = $2
    `, adminID, id)

    w.WriteHeader(http.StatusOK)
}
```

---

## Settlement Scheduler

### Scheduling Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│ SETTLEMENT SCHEDULER                                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│ Per-User Midnight Detection:                                    │
│                                                                 │
│ 1. Each user has timezone setting (Epic 7.6)                    │
│ 2. Scheduler tracks "next settlement time" per user             │
│ 3. When current time >= user's midnight:                        │
│    - Run settlement for previous day                            │
│    - Update next settlement time                                │
│                                                                 │
│ Implementation Options:                                         │
│                                                                 │
│ Option A: Cron-based (check every minute)                       │
│   - Simple to implement                                         │
│   - May miss exact midnight by up to 1 minute                   │
│                                                                 │
│ Option B: Time-wheel scheduler                                  │
│   - Schedule exact midnight per user                            │
│   - More complex but precise                                    │
│                                                                 │
│ Recommended: Option A with 1-minute check interval              │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Scheduler Implementation:**
```go
// internal/settlement/scheduler.go
type SettlementScheduler struct {
    db          *database.Service
    settlement  *SettlementService
    ticker      *time.Ticker
}

func (s *SettlementScheduler) Start() {
    s.ticker = time.NewTicker(1 * time.Minute)
    go func() {
        for range s.ticker.C {
            s.checkAndRunSettlements()
        }
    }()
}

func (s *SettlementScheduler) checkAndRunSettlements() {
    // Get all users with their timezones
    users, _ := s.db.GetUsersWithTimezones()

    for _, user := range users {
        loc, _ := time.LoadLocation(user.Timezone)
        userNow := time.Now().In(loc)

        // Check if we just passed midnight
        if userNow.Hour() == 0 && userNow.Minute() == 0 {
            yesterday := userNow.AddDate(0, 0, -1)

            // Format date as string (YYYY-MM-DD) to avoid DST issues
            dateStr := yesterday.Format("2006-01-02")

            go s.settlement.RunDailySettlement(user.ID, dateStr)
        }
    }
}
```

### DST (Daylight Saving Time) Handling

**IMPORTANT: Automatic DST Handling**

Go's `time.Location` automatically handles DST transitions:

```go
// Example: DST transition handling
loc, _ := time.LoadLocation("America/New_York")
now := time.Now().In(loc)

// On DST transition days, midnight still occurs once
// Go handles the clock change automatically

// Store date as string, not timestamp
dateStr := now.Format("2006-01-02")  // ✅ CORRECT: "2026-03-09"

// NOT this (DST-sensitive)
timestamp := now.Unix()  // ❌ WRONG: Can shift during DST
```

**DST Edge Cases Tested:**

| Scenario | Date | Time Change | Handling |
|----------|------|-------------|----------|
| **Spring Forward** | 2026-03-08 | 2:00 AM → 3:00 AM | Midnight occurs normally at 12:00 AM, settlement runs before DST change |
| **Fall Back** | 2026-11-01 | 2:00 AM → 1:00 AM | Midnight occurs once, settlement runs on first occurrence |
| **Timezone Change** | User changes timezone | N/A | Next settlement uses new timezone, date stored as string remains consistent |

**Best Practices:**

1. Always use `time.LoadLocation()` with IANA timezone names
2. Store `summary_date` as DATE type in SQL (not TIMESTAMP)
3. Store `settlement_time` as TIMESTAMP for audit trail
4. Use `date.Format("2006-01-02")` for date strings in code
5. Never calculate date arithmetic with timestamps during DST transitions

**Testing DST Transitions:**
```go
// Test case: Ensure settlement runs on DST transition
func TestSettlementDuringDST(t *testing.T) {
    loc, _ := time.LoadLocation("America/New_York")

    // March 8, 2026 - Spring forward at 2 AM
    testTime := time.Date(2026, 3, 8, 0, 0, 0, 0, loc)

    scheduler := NewSettlementScheduler(db, settlement)
    scheduler.checkAndRunSettlements()

    // Verify settlement ran for March 7
    expectedDate := "2026-03-07"
    assert.Equal(t, expectedDate, lastSettlementDate)
}
```

---

## Dependencies

### BLOCKERS (Must Complete Before Starting)

**CRITICAL**: These dependencies are **MANDATORY** before Epic 8 implementation can begin:

| Dependency | Component | Required For | Validation |
|------------|-----------|--------------|------------|
| **Epic 6: Redis** | Redis container deployed | Capital sampling cache | `docker ps \| grep redis` |
| **Epic 6: CacheService** | `internal/cache/service.go` implemented | Runtime state caching | `grep -r "type CacheService" internal/cache/` |
| **Epic 7: ParseClientOrderId** | `internal/orders/client_order_id.go` function | Mode extraction from orders | `grep -r "func ParseClientOrderId" internal/orders/` |
| **Story 8.0: User Timezone Migration** | `timezone` and `last_settlement_date` columns in users table | Settlement scheduling | `psql -c "\d users" \| grep timezone` |

### Prerequisite Dependencies Details

**Epic 6 - Redis (CacheService)**
- **What's Needed**: Redis container running + Go Redis client library
- **Used For**: Storing intraday capital samples without DB overhead
- **Story Impact**: Story 8.7 (Capital Utilization Tracking)
- **Check Command**:
  ```bash
  docker ps | grep redis && \
  grep -q "github.com/redis/go-redis" go.mod
  ```

**Epic 7 - Client Order ID Parsing**
- **What's Needed**: `ParseClientOrderId(clientOrderId string) *ParsedOrderId` function
- **Used For**: Extracting mode (ULT/SCA/SWI/POS) from trade's clientOrderId
- **Story Impact**: Stories 8.1, 8.2, 8.3 (mode breakdown calculations)
- **Function Signature**:
  ```go
  type ParsedOrderId struct {
      Mode     ModeType  // ULT, SCA, SWI, POS
      ChainId  string
      Stage    string
      ...
  }
  func ParseClientOrderId(clientOrderId string) *ParsedOrderId
  ```

**Story 8.0 - User Timezone Migration**
- **What's Needed**: Database columns added to users table
- **Used For**: Determining when to run settlement per user
- **Story Impact**: All settlement scheduling (Stories 8.1-8.10)
- **SQL to Verify**:
  ```sql
  SELECT column_name, data_type, column_default
  FROM information_schema.columns
  WHERE table_name = 'users'
  AND column_name IN ('timezone', 'last_settlement_date');
  ```

### External Dependencies

| Dependency | Type | Required For | Fallback Strategy |
|------------|------|--------------|-------------------|
| **Binance API** | External | Position/trade data | Retry 3x with backoff (Story 8.8) |
| **PostgreSQL** | Infrastructure | Data persistence | Transaction rollback + retry (Story 8.8) |
| **Email Service** | External (optional) | Admin alerts | Log to file if email unavailable |

### Dependency Order

```
START Epic 8
    ↓
┌─────────────────────────────────────────┐
│ Prerequisites (MUST be complete first)  │
├─────────────────────────────────────────┤
│ 1. Epic 6: Redis + CacheService         │
│ 2. Epic 7: ParseClientOrderId           │
│ 3. Story 8.0: User Timezone Migration   │
└─────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────┐
│ Core Settlement (Can proceed in order)  │
├─────────────────────────────────────────┤
│ Story 8.1: Position Snapshots           │
│ Story 8.2: P&L Aggregation              │
│ Story 8.3: Database Storage             │
│ Story 8.4: Open Position Handling       │
└─────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────┐
│ Advanced Features (Parallel possible)   │
├─────────────────────────────────────────┤
│ Story 8.5: Admin Dashboard              │
│ Story 8.6: Historical Reports           │
│ Story 8.7: Capital Tracking             │
│ Story 8.8: Failure Recovery             │
│ Story 8.9: Monitoring & Alerts          │
│ Story 8.10: Data Quality Validation     │
└─────────────────────────────────────────┘
```

---

## Success Criteria

### Core Functionality
1. **User Timezone Migration Complete** (Story 8.0):
   - `timezone` and `last_settlement_date` columns added to users table
   - Migration is idempotent and can be run multiple times
   - Default timezone set for existing users

2. **Settlement Runs** (Stories 8.1-8.4):
   - Daily settlement executes for all users at their midnight (user timezone)
   - Position snapshots captured accurately
   - Mode breakdown calculated correctly from clientOrderId

3. **Mode Breakdown Accurate** (Story 8.2):
   - P&L matches Binance when aggregated across all modes
   - Win rate calculations correct (wins / total trades)
   - "ALL" mode shows correct totals

4. **Historical Data Persists** (Story 8.3):
   - Data available beyond Binance's 90-day limit
   - Database queries efficient (<2s for 1-year range)

5. **Admin Can Bill** (Story 8.5):
   - Admin dashboard shows all users' P&L for billing
   - CSV export available for accounting
   - Filterable by date range, user, mode

6. **Capital Tracked** (Story 8.7):
   - Max capital used recorded each day
   - Average capital utilization calculated
   - Capital sampling runs every 5 minutes

7. **Reports Work** (Story 8.6):
   - Date range queries return correct aggregations
   - Weekly, monthly, yearly rollups accurate
   - Performance trend graphs display correctly

### Reliability & Quality
8. **Failure Recovery Works** (Story 8.8):
   - Binance API failures retry 3 times with exponential backoff
   - Database failures rollback and retry once
   - Failed settlements marked in database with error details
   - Admin can manually retry failed settlements

9. **Monitoring & Alerts Active** (Story 8.9):
   - Admin receives email alert for settlements failed >1 hour
   - Admin dashboard shows all failed settlements
   - Success rate and metrics visible
   - Manual retry button functional

10. **Data Quality Validated** (Story 8.10):
    - Win rate validated (0-100% range)
    - P&L validated (configurable bounds)
    - Anomalies flagged for admin review
    - Admin review queue functional
    - Manual approval workflow operational

### Performance & Stability
11. **Performance Targets Met**:
    - Settlement completes within 5 minutes per user (NFR-1)
    - Historical queries return within 2 seconds (NFR-2)
    - Data retained indefinitely (NFR-3)
    - Settlement resilient to partial failures (NFR-4)

12. **DST Handling Correct**:
    - Settlements run correctly during Spring forward
    - Settlements run correctly during Fall back
    - Date storage consistent across timezone changes

### Acceptance Gates
**EPIC 8 IS COMPLETE WHEN:**
- [ ] All 11 stories (8.0-8.10) acceptance criteria met
- [ ] All migrations applied successfully
- [ ] All unit, integration, and E2E tests passing
- [ ] Admin can view historical data beyond 90 days
- [ ] Admin can retry failed settlements
- [ ] Admin receives alerts for persistent failures
- [ ] Data quality validation catches anomalies
- [ ] Settlement runs for multi-timezone users without issues
- [ ] Performance targets met in load testing
- [ ] Documentation updated (API endpoints, admin guide)

---

## Technical Considerations

### New Files

```
internal/settlement/
├── service.go              # Main settlement service (Stories 8.1-8.4)
├── scheduler.go            # Per-user midnight scheduler (Scheduler section)
├── position_snapshot.go    # Open position snapshots (Story 8.1)
├── pnl_aggregator.go       # P&L by mode aggregation (Story 8.2)
├── capital_tracker.go      # Capital utilization sampling (Story 8.7)
├── error_handling.go       # Retry logic and failure recovery (Story 8.8)
├── monitoring.go           # Settlement monitoring and alerting (Story 8.9)
├── validation.go           # Data quality validation (Story 8.10)
└── types.go                # Settlement types

internal/database/
├── repository_daily_summaries.go   # DB operations (Story 8.3)
├── migrations/
│   ├── 20260106_add_user_timezone_settlement.sql  # Story 8.0
│   └── 20260106_create_daily_summaries.sql        # Stories 8.3, 8.8, 8.10

internal/api/
└── handlers_settlements.go   # Settlement admin endpoints (Stories 8.8, 8.9, 8.10)

web/src/components/Analytics/
├── DailySummaryTable.tsx
├── ModePerformanceChart.tsx
├── PnlTrendChart.tsx
└── CapitalUtilizationChart.tsx

web/src/pages/
├── AdminDailySettlements.tsx      # Admin dashboard (Story 8.5)
├── AdminSettlementStatus.tsx      # Failed settlements view (Story 8.9)
└── AdminSettlementReviewQueue.tsx # Data quality review (Story 8.10)
```

### Modified Files

| File | Changes | Story |
|------|---------|-------|
| main.go | Start settlement scheduler, monitoring goroutine | Scheduler, 8.9 |
| internal/api/server.go | Add settlement endpoints (retry, status, review queue) | 8.5, 8.8, 8.9, 8.10 |
| internal/api/handlers_admin.go | Admin settlement views, retry endpoint | 8.5, 8.8, 8.9 |
| web/src/services/analyticsApi.ts | Analytics API client | 8.5, 8.6 |
| web/src/services/adminApi.ts | Admin settlement API calls | 8.9, 8.10 |
| internal/config/config.go | Add settlement configuration (retry params, thresholds) | 8.8, 8.10 |

### Database Migration

```sql
-- Migration: 20260106_create_daily_summaries.sql

-- Table 1: Daily mode summaries (main settlement data)
CREATE TABLE daily_mode_summaries (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    summary_date DATE NOT NULL,
    timezone VARCHAR(50) NOT NULL,

    -- Mode identification (ULT, SCA, SWI, POS, ALL)
    mode VARCHAR(10) NOT NULL,

    -- Trade metrics
    trade_count INT DEFAULT 0,
    win_count INT DEFAULT 0,
    loss_count INT DEFAULT 0,
    win_rate DECIMAL(5,2),

    -- P&L metrics (in USDT)
    realized_pnl DECIMAL(18,8) DEFAULT 0,
    unrealized_pnl DECIMAL(18,8) DEFAULT 0,
    total_pnl DECIMAL(18,8) DEFAULT 0,

    -- Volume metrics
    total_volume_usdt DECIMAL(18,8) DEFAULT 0,
    avg_trade_size DECIMAL(18,8) DEFAULT 0,
    largest_win DECIMAL(18,8) DEFAULT 0,
    largest_loss DECIMAL(18,8) DEFAULT 0,

    -- Capital metrics
    starting_balance DECIMAL(18,8) DEFAULT 0,
    ending_balance DECIMAL(18,8) DEFAULT 0,
    max_capital_used DECIMAL(18,8) DEFAULT 0,
    avg_capital_used DECIMAL(18,8) DEFAULT 0,
    max_drawdown DECIMAL(18,8) DEFAULT 0,

    -- Fee metrics
    total_fees DECIMAL(18,8) DEFAULT 0,
    commission_fees DECIMAL(18,8) DEFAULT 0,
    funding_fees DECIMAL(18,8) DEFAULT 0,

    -- Position snapshot at EOD
    open_positions_count INT DEFAULT 0,
    open_positions_value DECIMAL(18,8) DEFAULT 0,

    -- Metadata
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    settlement_time TIMESTAMP,
    settlement_status VARCHAR(20) DEFAULT 'completed',  -- completed, failed, retrying
    settlement_error TEXT,  -- Error message if failed

    -- Data quality (Story 8.10)
    data_quality_flag BOOLEAN DEFAULT false,
    data_quality_notes TEXT,
    reviewed_by UUID REFERENCES users(id),
    reviewed_at TIMESTAMP,
    alerted BOOLEAN DEFAULT false,  -- For Story 8.9 alerting

    UNIQUE(user_id, summary_date, mode)
);

-- Indexes for performance
CREATE INDEX idx_daily_summaries_user_date
    ON daily_mode_summaries(user_id, summary_date DESC);
CREATE INDEX idx_daily_summaries_mode
    ON daily_mode_summaries(mode, summary_date DESC);
CREATE INDEX idx_daily_summaries_date_range
    ON daily_mode_summaries(summary_date, user_id);
CREATE INDEX idx_daily_summaries_status
    ON daily_mode_summaries(settlement_status)
    WHERE settlement_status != 'completed';
CREATE INDEX idx_daily_summaries_quality_review
    ON daily_mode_summaries(data_quality_flag, reviewed_at)
    WHERE data_quality_flag = true AND reviewed_at IS NULL;

-- Table 2: Position snapshots at EOD
CREATE TABLE daily_position_snapshots (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    snapshot_date DATE NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    side VARCHAR(10) NOT NULL,
    mode VARCHAR(10),
    chain_id VARCHAR(30),
    quantity DECIMAL(18,8),
    entry_price DECIMAL(18,8),
    mark_price DECIMAL(18,8),
    unrealized_pnl DECIMAL(18,8),
    leverage INT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_snapshots_user_date (user_id, snapshot_date)
);

-- Table 3: Capital samples during the day
CREATE TABLE capital_samples (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    sampled_at TIMESTAMP NOT NULL,
    total_balance DECIMAL(18,8),
    used_margin DECIMAL(18,8),
    available_margin DECIMAL(18,8),
    unrealized_pnl DECIMAL(18,8),

    INDEX idx_capital_samples_user_time (user_id, sampled_at)
);

-- Comments for documentation
COMMENT ON TABLE daily_mode_summaries IS 'Daily settlement summaries by trading mode';
COMMENT ON COLUMN daily_mode_summaries.settlement_status IS 'Status: completed, failed, retrying';
COMMENT ON COLUMN daily_mode_summaries.data_quality_flag IS 'True if data validation warnings present';
COMMENT ON COLUMN daily_mode_summaries.alerted IS 'True if admin has been alerted about failure';
```

---

## Testing Strategy

### Unit Tests
- P&L aggregation calculations (Story 8.2)
- Mode extraction from clientOrderId (Story 8.2)
- Capital metrics calculations (Story 8.7)
- Win rate calculations (Story 8.2)
- Data quality validation logic (Story 8.10)
  - Win rate bounds (0-100%)
  - P&L range validation
  - Trade count validation
  - Win/Loss count consistency
- Error classification (retryable vs non-retryable) (Story 8.8)
- Exponential backoff timing (Story 8.8)

### Integration Tests
- Full settlement flow for test user (Stories 8.1-8.4)
- Database persistence with all columns (Story 8.3)
- Admin query endpoints (Story 8.5)
- Settlement retry after failure (Story 8.8)
- Failed settlement alerting (Story 8.9)
- Data quality flagging workflow (Story 8.10)
- Admin review queue operations (Story 8.10)

### End-to-End Tests
- Settlement at midnight (time simulation) (Scheduler)
- Multi-timezone scenarios (Story 8.0, Scheduler)
- DST transitions (Spring forward, Fall back) (Scheduler)
- Date range report generation (Story 8.6)
- Complete failure recovery cycle:
  1. Settlement fails (Binance timeout)
  2. Retry 3 times with backoff
  3. Mark as failed
  4. Admin receives alert
  5. Admin retries manually
  6. Settlement succeeds (Stories 8.8, 8.9)
- Data quality validation flow:
  1. Settlement produces anomalous data
  2. Flagged for review
  3. Appears in admin queue
  4. Admin approves
  5. Flag cleared (Story 8.10)

### Load Tests
- 100 concurrent user settlements (stress test)
- Binance API rate limit handling
- Database connection pool under load
- Settlement duration monitoring (must complete <5 min/user)

---

## Risks & Mitigations

| Risk | Severity | Mitigation | Story |
|------|----------|------------|-------|
| **Binance API rate limits during settlement** | MEDIUM | Stagger user settlements, cache where possible, exponential backoff retry (5s, 15s, 45s) | 8.8 |
| **Binance API timeout/connection failure** | HIGH | 3-retry strategy with exponential backoff, admin alerting after 1 hour | 8.8, 8.9 |
| **Database transaction failure** | MEDIUM | Rollback + single retry after 10s, mark as failed if unsuccessful | 8.8 |
| **Partial settlement data** | MEDIUM | Mark as 'failed' status, store error details, admin retry endpoint | 8.8, 8.9 |
| **Missing clientOrderId (legacy orders)** | LOW | Default to "UNKNOWN" mode, still include in "ALL" totals | 8.2 |
| **Timezone edge cases / DST transitions** | LOW | Use Go time.Location (auto-handles DST), store dates as strings, test Spring/Fall transitions | 8.0, Scheduler |
| **Settlement job failure (crash/restart)** | MEDIUM | Persistent status tracking in DB, auto-retry on restart, manual trigger endpoint | 8.8, 8.9 |
| **Invalid/anomalous data** | MEDIUM | Data quality validation gates, flag for admin review, manual approval workflow | 8.10 |
| **Win rate calculation errors** | LOW | Validate 0-100% range, reject if invalid, alert admin | 8.10 |
| **P&L data integrity** | MEDIUM | Compare with Binance snapshot, flag if >$100 difference, admin review queue | 8.10 |
| **High trade volume (500+ trades/day)** | LOW | Flag as suspicious, admin review, configurable threshold | 8.10 |
| **Admin not notified of failures** | MEDIUM | Email alerts after 1 hour, monitoring dashboard, periodic status checks | 8.9 |
| **Failed settlements accumulate** | MEDIUM | Admin dashboard shows all failures, one-click retry, bulk retry option | 8.9 |
| **Settlement timing drift** | LOW | 1-minute check interval, track last_settlement_date to prevent duplicates | Scheduler |
| **User changes timezone mid-day** | LOW | Next settlement uses new timezone, current day already scheduled | 8.0 |

---

## Author

**Created By:** BMAD Party Mode (Analyst: Mary, Architect: Winston, PM: John, Dev: Amelia, QA: Murat)
**Date:** 2026-01-06
**Version:** 1.0
**Depends On:** Epic 6 (Redis), Epic 7 (Client Order ID)
