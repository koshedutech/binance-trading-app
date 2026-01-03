# Architecture Document: Editable Paper Trading Balance

**Author:** Winston (Architect)
**Date:** 2026-01-02
**Version:** 1.0
**Status:** Approved

---

## Architecture Overview

This feature extends the existing `user_trading_configs` table to support per-user, per-trading-type paper balances with API-driven synchronization from Binance.

---

## System Context

```
┌─────────────────────────────────────────────────────────────┐
│                       User (Browser)                        │
└──────────────────────┬──────────────────────────────────────┘
                       │ HTTPS
                       ▼
┌─────────────────────────────────────────────────────────────┐
│              React Frontend (TypeScript)                    │
│  - Settings Page Component                                  │
│  - Paper Balance Controls (conditional render)              │
│  - API Service Layer                                        │
└──────────────────────┬──────────────────────────────────────┘
                       │ REST API (JSON)
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                 Go Backend (Gin Framework)                  │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  API Layer (internal/api/)                          │   │
│  │  - GET /api/settings/paper-balance/:type            │   │
│  │  - PUT /api/settings/paper-balance/:type            │   │
│  │  - POST /api/settings/sync-paper-balance/:type      │   │
│  └────┬────────────────────────────────────────────────┘   │
│       │                                                     │
│  ┌────▼───────────────────────────────────────────────┐    │
│  │  Service Layer (internal/services/)                │    │
│  │  - PaperBalanceService                             │    │
│  │  - BinanceClientService (existing)                 │    │
│  └────┬────────────────────────┬────────────────────┘     │
│       │                        │                           │
│  ┌────▼────────────────┐  ┌───▼─────────────────┐         │
│  │ Repository Layer    │  │ Binance API Client  │         │
│  │ TradingConfigRepo   │  │ (Spot & Futures)    │         │
│  └────┬────────────────┘  └─────────────────────┘         │
└───────┼───────────────────────────────────────────────────┘
        │ SQL Queries
        ▼
┌─────────────────────────────────────────────────────────────┐
│              PostgreSQL Database                            │
│  Table: user_trading_configs                                │
│  - user_id (FK)                                             │
│  - trading_type (spot/futures)                              │
│  - dry_run_mode (boolean)                                   │
│  - paper_balance_usdt (DECIMAL(20,8)) ← NEW                │
└─────────────────────────────────────────────────────────────┘
```

---

## Database Schema Changes

### Migration: Add `paper_balance_usdt` Column

```sql
-- Migration Up
ALTER TABLE user_trading_configs
ADD COLUMN paper_balance_usdt DECIMAL(20,8) NOT NULL DEFAULT 10000.0;

-- Backfill existing rows (explicit for clarity)
UPDATE user_trading_configs
SET paper_balance_usdt = 10000.0
WHERE paper_balance_usdt IS NULL;

-- Add check constraint for validation
ALTER TABLE user_trading_configs
ADD CONSTRAINT check_paper_balance_range
CHECK (paper_balance_usdt >= 10.0 AND paper_balance_usdt <= 1000000.0);

-- Migration Down (Rollback)
ALTER TABLE user_trading_configs
DROP CONSTRAINT IF EXISTS check_paper_balance_range;

ALTER TABLE user_trading_configs
DROP COLUMN paper_balance_usdt;
```

### Updated Table Schema

```
user_trading_configs
├── id (SERIAL PRIMARY KEY)
├── user_id (INTEGER NOT NULL, FK → users.id)
├── trading_type (VARCHAR(10) NOT NULL) -- 'spot' or 'futures'
├── dry_run_mode (BOOLEAN NOT NULL DEFAULT false)
├── paper_balance_usdt (DECIMAL(20,8) NOT NULL DEFAULT 10000.0) ← NEW
├── created_at (TIMESTAMP)
├── updated_at (TIMESTAMP)
└── UNIQUE(user_id, trading_type)
```

### Indexing Strategy

Existing unique index on `(user_id, trading_type)` is sufficient. No additional indexes required.

---

## API Contract Specification

### 1. Get Paper Balance

**Endpoint:** `GET /api/settings/paper-balance/:trading_type`

**Path Parameters:**
- `trading_type` (string, required): `spot` or `futures`

**Authentication:** Required (JWT)

**Response 200 OK:**
```json
{
  "trading_type": "futures",
  "paper_balance_usdt": "10000.00000000",
  "dry_run_mode": true
}
```

**Response 400 Bad Request:**
```json
{
  "error": "Invalid trading type. Must be 'spot' or 'futures'"
}
```

**Response 401 Unauthorized:**
```json
{
  "error": "Authentication required"
}
```

---

### 2. Update Paper Balance (Manual Entry)

**Endpoint:** `PUT /api/settings/paper-balance/:trading_type`

**Path Parameters:**
- `trading_type` (string, required): `spot` or `futures`

**Authentication:** Required (JWT)

**Request Body:**
```json
{
  "balance": 5000.50
}
```

**Validation Rules:**
- `balance` must be numeric
- `balance` >= 10.0
- `balance` <= 1000000.0

**Response 200 OK:**
```json
{
  "trading_type": "futures",
  "paper_balance_usdt": "5000.50000000",
  "message": "Paper balance updated successfully"
}
```

**Response 400 Bad Request:**
```json
{
  "error": "Balance must be between $10 and $1,000,000"
}
```

---

### 3. Sync Paper Balance from Real Binance Account

**Endpoint:** `POST /api/settings/sync-paper-balance/:trading_type`

**Path Parameters:**
- `trading_type` (string, required): `spot` or `futures`

**Authentication:** Required (JWT)

**Request Body:** None

**Business Logic:**
1. Verify user has Binance API credentials configured
2. Call appropriate Binance API:
   - **Spot**: `GET /api/v3/account` → extract USDT balance from `balances` array
   - **Futures**: `GET /fapi/v2/balance` → extract USDT balance
3. Update `paper_balance_usdt` in database
4. Return updated balance

**Response 200 OK:**
```json
{
  "trading_type": "spot",
  "paper_balance_usdt": "3547.82000000",
  "synced_from": "binance_spot_account",
  "message": "Paper balance synced successfully"
}
```

**Response 400 Bad Request (No API Key):**
```json
{
  "error": "Binance API credentials not configured",
  "action_required": "Please add your API keys in Settings"
}
```

**Response 502 Bad Gateway (Binance API Failed):**
```json
{
  "error": "Failed to fetch balance from Binance",
  "details": "API request timeout",
  "retry_suggested": true
}
```

**Response 503 Service Unavailable (Rate Limited):**
```json
{
  "error": "Binance API rate limit exceeded",
  "retry_after_seconds": 60
}
```

---

## Component Design

### Backend Components

#### 1. Repository Layer (`internal/database/trading_config_repository.go`)

```go
type TradingConfigRepository interface {
    GetPaperBalance(userID int64, tradingType string) (decimal.Decimal, error)
    UpdatePaperBalance(userID int64, tradingType string, balance decimal.Decimal) error
    GetTradingConfig(userID int64, tradingType string) (*TradingConfig, error)
}
```

**Implementation Notes:**
- Use `github.com/shopspring/decimal` for precise decimal handling
- Return `sql.ErrNoRows` if config doesn't exist (handled by service layer)
- Use prepared statements to prevent SQL injection

---

#### 2. Service Layer (`internal/services/paper_balance_service.go`)

```go
type PaperBalanceService struct {
    repo           TradingConfigRepository
    binanceClient  BinanceClientInterface
}

func (s *PaperBalanceService) GetPaperBalance(userID int64, tradingType string) (decimal.Decimal, error)
func (s *PaperBalanceService) UpdatePaperBalance(userID int64, tradingType string, balance decimal.Decimal) error
func (s *PaperBalanceService) SyncFromBinance(userID int64, tradingType string) (decimal.Decimal, error)
```

**SyncFromBinance Logic:**
```go
func (s *PaperBalanceService) SyncFromBinance(userID int64, tradingType string) (decimal.Decimal, error) {
    // 1. Get user's Binance API credentials
    apiKey, secretKey, err := s.getAPICredentials(userID)
    if err != nil {
        return decimal.Zero, ErrNoAPICredentials
    }

    // 2. Fetch balance from Binance
    var balance decimal.Decimal
    if tradingType == "spot" {
        balance, err = s.binanceClient.GetSpotUSDTBalance(apiKey, secretKey)
    } else {
        balance, err = s.binanceClient.GetFuturesUSDTBalance(apiKey, secretKey)
    }
    if err != nil {
        return decimal.Zero, fmt.Errorf("binance API error: %w", err)
    }

    // 3. Validate balance is within allowed range
    if balance.LessThan(decimal.NewFromFloat(10.0)) {
        return decimal.Zero, ErrBalanceBelowMinimum
    }

    // 4. Update database
    err = s.repo.UpdatePaperBalance(userID, tradingType, balance)
    if err != nil {
        return decimal.Zero, fmt.Errorf("database update failed: %w", err)
    }

    return balance, nil
}
```

---

#### 3. API Handler Layer (`internal/api/handlers_paper_balance.go`)

```go
func GetPaperBalance(c *gin.Context)
func UpdatePaperBalance(c *gin.Context)
func SyncPaperBalance(c *gin.Context)
```

**Error Mapping:**
- `ErrNoAPICredentials` → 400 Bad Request
- Binance timeout/error → 502 Bad Gateway
- Binance rate limit → 503 Service Unavailable
- Validation errors → 400 Bad Request
- Database errors → 500 Internal Server Error

---

### Frontend Components

#### 1. Settings Page Component (`web/src/pages/Settings.tsx`)

**Conditional Rendering:**
```tsx
{tradingConfig.dry_run_mode && (
  <PaperBalanceSection
    tradingType={currentTradingType}
    balance={paperBalance}
    onUpdate={handleBalanceUpdate}
    onSync={handleBalanceSync}
  />
)}
```

---

#### 2. Paper Balance Section Component (`web/src/components/PaperBalanceSection.tsx`)

**State Management:**
- `balance` (string): Current paper balance
- `isEditing` (boolean): Edit mode toggle
- `isSyncing` (boolean): Sync operation in progress
- `error` (string | null): Error message display

**UI Elements:**
- Balance display with currency formatting
- Edit button / Save button
- Sync from Real Balance button
- Error/success toast notifications

---

#### 3. API Service (`web/src/services/paperBalanceService.ts`)

```typescript
export const paperBalanceService = {
  getPaperBalance: async (tradingType: 'spot' | 'futures'): Promise<PaperBalanceResponse> => {
    const response = await api.get(`/api/settings/paper-balance/${tradingType}`);
    return response.data;
  },

  updatePaperBalance: async (tradingType: 'spot' | 'futures', balance: number): Promise<void> => {
    await api.put(`/api/settings/paper-balance/${tradingType}`, { balance });
  },

  syncPaperBalance: async (tradingType: 'spot' | 'futures'): Promise<PaperBalanceResponse> => {
    const response = await api.post(`/api/settings/sync-paper-balance/${tradingType}`);
    return response.data;
  }
};
```

---

## Error Handling Strategy

### Client-Side Validation
- Balance range: $10 - $1,000,000
- Numeric input only (allow commas, parse to float)
- Real-time feedback (red border + error text)

### Server-Side Validation
- Database constraint: `CHECK (paper_balance_usdt >= 10.0 AND paper_balance_usdt <= 1000000.0)`
- API endpoint validation before database write
- Sanitize input to prevent injection

### Binance API Error Handling

| Error Type | HTTP Code | User Message | Retry Strategy |
|------------|-----------|--------------|----------------|
| Timeout | 502 | "Sync failed - Binance is slow. Try again." | Manual retry |
| Rate Limit | 503 | "Too many requests. Wait 60 seconds." | Exponential backoff |
| Auth Error | 400 | "API keys invalid. Check Settings." | No retry |
| Network Error | 502 | "Connection failed. Check internet." | Manual retry |

### Database Error Handling
- Transaction rollback on sync failure
- Log errors for monitoring
- Return generic 500 to user (don't leak internals)

---

## Security Considerations

### Authentication & Authorization
- All endpoints require valid JWT token
- User can ONLY access their own paper balance
- Middleware validates `userID` from token matches request

### Data Protection
- Binance API keys stored encrypted (existing auth service)
- No API keys transmitted in responses
- Balance data not cached client-side (always fetch from server)

### Input Validation
- Parameterized SQL queries (prevent injection)
- Strict type validation on `trading_type` parameter
- Decimal precision validation (no arbitrary precision attacks)

---

## Performance Considerations

### Database Queries
- Index on `(user_id, trading_type)` ensures <100ms lookups
- No N+1 query issues (single query per operation)

### Binance API Calls
- Timeout: 5 seconds max
- Caching: Consider 5-minute cache for sync results (future optimization)
- Rate limiting: Respect Binance limits (1200 requests/minute for Spot)

### Frontend Performance
- Debounce manual input (500ms delay before validation)
- Optimistic UI updates for better UX
- Loading states prevent double-submission

---

## Testing Strategy

### Unit Tests
- Repository methods (mocked database)
- Service layer logic (mocked Binance client)
- API handlers (mocked service layer)
- Frontend components (React Testing Library)

### Integration Tests
- Full API flow: Request → Database → Response
- Binance API integration (use testnet)
- Database migration up/down scripts

### E2E Tests
1. User enables paper mode
2. User edits paper balance to $5,000
3. User places paper trade
4. Verify balance decremented correctly
5. User clicks "Sync from Real Balance"
6. Verify balance updated from Binance

---

## Deployment Plan

### Migration Deployment
1. **Pre-Deployment**: Backup `user_trading_configs` table
2. **Deployment**: Run migration script during maintenance window
3. **Verification**: Query sample rows to confirm default 10000.0 applied
4. **Rollback Plan**: Execute migration down script if issues detected

### Feature Rollout
- **Phase 1**: Deploy backend APIs (no UI) - test endpoints manually
- **Phase 2**: Deploy frontend UI - enable for 10% of users (feature flag)
- **Phase 3**: Monitor error rates, expand to 50% users
- **Phase 4**: Full rollout to 100% users

### Monitoring
- Track sync API error rates (alert if >5%)
- Monitor database query performance
- Log Binance API failures for pattern analysis

---

## Assumptions and Constraints

### Assumptions
- Binance API reliability >95% uptime
- Users primarily trade USDT-based pairs
- PostgreSQL decimal precision sufficient for all balance values

### Constraints
- Hard cap of $1,000,000 for MVP (configurable in future)
- USDT only (multi-currency support deferred)
- No historical balance tracking in v1.0

---

## Future Enhancements

### Version 2.0
- Balance change audit log (timestamp, old/new value, source)
- Scheduled auto-sync (cron job)
- Configurable max limit per user tier

### Version 3.0
- Multi-currency support (BTC, ETH, BNB)
- Paper balance templates (conservative, moderate, aggressive)
- Social sharing of paper trading results

---

## Approval Sign-Off

- **Architect (Winston)**: ✅ Approved 2026-01-02
- **Product Manager (John)**: _Pending Review_
- **Developer (Amelia)**: _Pending Review_
- **Test Architect (Murat)**: _Pending Review_

---

## References

- [Binance API Documentation - Account Balance](https://binance-docs.github.io/apidocs/spot/en/#account-information-user_data)
- [PostgreSQL DECIMAL Type](https://www.postgresql.org/docs/current/datatype-numeric.html)
- [shopspring/decimal Library](https://github.com/shopspring/decimal)
