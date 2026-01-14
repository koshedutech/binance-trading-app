# Story 9.6: Actual Fee Tracking from Binance API

## Story Overview

**Epic**: Epic 9 - Entry Signal Quality Improvements & Infrastructure
**Story Type**: Feature Enhancement
**Priority**: High
**Complexity**: Medium
**Status**: In Progress (Core Implementation Complete)
**Created:** 2026-01-14
**Last Updated:** 2026-01-14

### Implementation Summary (2026-01-14)
**Completed:**
- Fixed all hardcoded fee rates (0.0004 → 0.0005)
- Created database migration 025 for user_fee_rates table
- Added fee tracking fields to GiniePosition struct
- Implemented WebSocket fee capture in FuturesController
- Added UpdatePositionFees() and GetPosition() methods to GinieAutopilot

**Deferred to separate story:**
- Loading user fee rates from database into memory cache on startup/login
- Using dynamic user-specific rates in calculations

---

## Problem Statement

### Current Issues

1. **Fee calculation uses hardcoded estimates** instead of actual Binance data
2. **Wrong taker fee rate** - Code has 0.04% but standard accounts pay **0.05%**
3. **No per-trade fee tracking** - Actual fees paid per order not captured
4. **Multiple inconsistent hardcoded values** across 7 locations in codebase

### Verified Actual Rates (Your Account)

| Fee Type | Rate | Decimal |
|----------|------|---------|
| **Maker** | 0.02% | 0.0002 |
| **Taker** | 0.05% | 0.0005 |

**Note:** The API verified these are your actual Binance standard account rates.

---

## Solution Summary

Implement a two-tier fee tracking system:

| Tier | Purpose | When |
|------|---------|------|
| **Tier 1: Rate Cache** | Store user's fee tier rates | Login + daily refresh |
| **Tier 2: Actual Fees** | Capture real fees per trade | Every order fill |

---

## Part 1: Current Fee Locations (Audit)

### Hardcoded Fee Locations

| Location | File | Current Value | Correct Value | Status |
|----------|------|---------------|---------------|--------|
| 1 | `ginie_autopilot.go:24-32` | Taker: 0.0004 | 0.0005 | **WRONG** |
| 2 | `handlers_mode.go:59-65` | Taker: 0.05% | 0.05% | Correct |
| 3 | `ginie_autopilot.go` (inline) | 0.0004 | 0.0005 | **WRONG** |
| 4 | `ginie_analyzer.go:4592` | 0.0004 | 0.0005 | **WRONG** |
| 5 | `futures_mock_client.go:272` | 0.0004 | 0.0005 | **WRONG** |
| 6 | `futures_mock_client.go:120-121` | 0.0004 | 0.0005 | **WRONG** |

### Code Details

#### Location 1: Constants in `ginie_autopilot.go`
```go
const (
    TakerFeeRate = 0.0004  // WRONG - should be 0.0005
    MakerFeeRate = 0.0002  // Correct
)
```

#### Location 2: Constants in `handlers_mode.go`
```go
const (
    takerFeePercent      = 0.05  // Correct (0.05%)
    makerFeePercent      = 0.02  // Correct (0.02%)
    roundTripFeePercent  = 0.10  // Should be 0.14% (0.05% + 0.05% + 0.02% + 0.02%)
)
```

#### Location 3-5: Inline hardcoded values
```go
// Multiple locations using wrong rate:
exitFeeUSD := currentPrice * closeQty * 0.0004 // WRONG
```

---

## Part 2: Commission Rate API (IMPLEMENTED)

### API Endpoint Added

**File:** `internal/binance/futures_client.go`

```go
// GetCommissionRate fetches user's actual commission rates from Binance
// Endpoint: GET /fapi/v1/commissionRate
func (c *FuturesClientImpl) GetCommissionRate(symbol string) (*CommissionRate, error)
```

**Response:**
```json
{
  "symbol": "BTCUSDT",
  "makerCommissionRate": "0.0002",
  "takerCommissionRate": "0.0005"
}
```

### API Route Added

**File:** `internal/api/server.go`

```go
futures.GET("/commission-rate", s.handleGetCommissionRate)
```

**Usage:**
```bash
GET /api/futures/commission-rate?symbol=BTCUSDT
```

---

## Part 3: Fee Data from Order Fills

### WebSocket ORDER_TRADE_UPDATE Fields

Binance provides actual fee data with every order fill:

```go
// internal/binance/user_data_stream.go
type OrderTradeUpdate struct {
    CommissionAsset string  `json:"N"`   // "USDT"
    Commission      float64 `json:"n"`   // Actual fee (e.g., 0.05)
    IsMakerSide     bool    `json:"m"`   // true = maker, false = taker
}
```

### REST API Trade Response

```go
// internal/binance/futures_types.go
type FuturesTrade struct {
    Commission      float64 `json:"commission,string"`
    CommissionAsset string  `json:"commissionAsset"`
}
```

**Key Insight:** We get actual fees per trade - no need to estimate!

---

## Part 4: Implementation Tasks

### Task 1: Update Hardcoded Fee Constants
- [x] Fix `ginie_autopilot.go` TakerFeeRate: 0.0004 → 0.0005
- [x] Fix inline 0.0004 values in PnL calculations (now uses TakerFeeRate constant)
- [x] Fix `ginie_analyzer.go` binanceTakerFee constant
- [x] Fix `futures_mock_client.go` mock fee values
- [x] Fix `early_profit_test.go` test expected values

### Task 2: Add User Fee Rates Caching
- [x] Create `user_fee_rates` database table (migration 025)
- [ ] Fetch commission rate on user login (DEFERRED - separate story)
- [ ] Store in database with `fetched_at` timestamp (DEFERRED - separate story)
- [ ] Add daily refresh mechanism (DEFERRED - separate story)

### Task 3: Capture Actual Fees Per Position
- [x] Add fee tracking fields to `GiniePosition` struct:
  ```go
  EntryFeeUSD     float64 `json:"entry_fee_usd"`
  EntryWasMaker   bool    `json:"entry_was_maker"`
  ExitFeeUSD      float64 `json:"exit_fee_usd"`
  ExitWasMaker    bool    `json:"exit_was_maker"`
  TotalFeesUSD    float64 `json:"total_fees_usd"`
  ```
- [x] Capture `Commission` and `IsMakerSide` from WebSocket on order fill
- [x] Store with position data
- [x] Added `UpdatePositionFees()` method to GinieAutopilot
- [x] Added `GetPosition()` method to GinieAutopilot

### Task 4: Use Actual Fees in Calculations
- [x] Fixed hardcoded estimates to use correct TakerFeeRate constant (0.0005)
- [ ] Replace with `ga.userFeeRates.TakerRate` (DEFERRED - needs cache loading)
- [ ] Use actual captured fee for closed position PnL (DEFERRED - needs cache loading)
- [ ] Update breakeven calculations with real fees (DEFERRED - needs cache loading)

---

## Part 5: Database Changes

### New Table: user_fee_rates

```sql
CREATE TABLE user_fee_rates (
    id SERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id) UNIQUE,

    -- Rates from Binance API
    maker_rate DECIMAL(10,8),  -- e.g., 0.0002 (0.02%)
    taker_rate DECIMAL(10,8),  -- e.g., 0.0005 (0.05%)

    -- Fetch metadata
    fetched_at TIMESTAMP,
    symbol VARCHAR(20),        -- Rate fetched for this symbol

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_fee_rates_user ON user_fee_rates(user_id);
```

---

## Part 6: Fee Caching Strategy

### When to Fetch Commission Rate

| Event | Action |
|-------|--------|
| User Login | Fetch from Binance API → Store in DB |
| Daily (cron) | Refresh all active users' rates |
| Manual Refresh | User clicks refresh in settings |
| First trade of day | Check if rates > 24h old → refresh |

### Rate Limit Safe

- **1 API call per day per user** (not per trade)
- Actual fees come from order fill data (free)
- No additional API calls during trading

---

## Acceptance Criteria

### AC9.6.1: Fix Hardcoded Values
- [x] All hardcoded taker fees updated from 0.0004 to 0.0005
- [x] Mock client uses correct rates
- [x] Test file expected values updated
- Note: handlers_mode.go roundTripFeePercent is correct at 0.10% (taker+taker for entry+exit)

### AC9.6.2: Commission Rate API
- [x] `GetCommissionRate()` method implemented
- [x] API endpoint `/api/futures/commission-rate` working
- [x] Database table `user_fee_rates` created (migration 025)
- [ ] Fetches on user login (DEFERRED - separate story for cache loading)
- [ ] Stores in `user_fee_rates` table (DEFERRED - separate story)

### AC9.6.3: Per-Trade Fee Capture
- [x] Fee fields added to GiniePosition (EntryFeeUSD, ExitFeeUSD, TotalFeesUSD, IsMaker flags)
- [x] Commission captured from WebSocket ORDER_TRADE_UPDATE
- [x] IsMakerSide captured to know maker vs taker
- [x] Stored with position data via UpdatePositionFees()

### AC9.6.4: Use Real Fees
- [x] PnL calculations use correct TakerFeeRate constant (0.0005)
- [ ] Dynamic user fee rates from database (DEFERRED - needs cache loading story)
- [ ] Breakeven calculation uses real entry fee (DEFERRED)
- [ ] Exit calculations use real exit fee (DEFERRED)

---

## Testing Strategy

### Test 1: Commission Rate API
```bash
# Should return actual rates from Binance
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8095/api/futures/commission-rate?symbol=BTCUSDT"

# Expected:
{
  "maker_commission_rate": 0.0002,
  "taker_commission_rate": 0.0005,
  "maker_percent": 0.02,
  "taker_percent": 0.05
}
```

### Test 2: Fee Capture on Trade
```
1. Open a position
2. Check GiniePosition has EntryFeeUSD and EntryWasMaker populated
3. Close position
4. Check ExitFeeUSD and ExitWasMaker populated
5. Verify TotalFeesUSD = EntryFeeUSD + ExitFeeUSD
```

---

## Dependencies

- **Provides to Epic 10:** User fee rates and per-position fee data
- **Used by:** Story 10.1 (Position Efficiency Tracking) - needs actual fees for threshold calculations

---

## Files to Modify

| File | Changes |
|------|---------|
| `ginie_autopilot.go` | Fix TakerFeeRate constant, inline values |
| `ginie_analyzer.go` | Fix binanceTakerFee constant |
| `futures_mock_client.go` | Fix mock fee rates |
| `handlers_mode.go` | Fix roundTripFeePercent |
| `ginie_types.go` | Add fee tracking fields to GiniePosition |
| `user_data_stream.go` | Capture Commission and IsMakerSide |
| Database | Add user_fee_rates table |

---

## Summary

This story focuses **only on fee tracking**:
1. Fix wrong hardcoded values (0.04% → 0.05%)
2. Cache user's actual rates from Binance API
3. Capture actual fees per trade from order fills
4. Provide fee data for Epic 10 position management

The time-unit efficiency exit system and profit protection features have been moved to **Epic 10: Position Management & Optimization**.
