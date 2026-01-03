# Story 3: Trading Logic Update - Use DB Paper Balance

**Story ID:** PAPER-003
**Epic:** Editable Paper Trading Balance
**Priority:** High
**Estimated Effort:** 3 hours
**Author:** Bob (Scrum Master)
**Status:** Ready for Development

---

## Description

Update Spot and Futures trading handlers to retrieve paper balances from the database instead of using hardcoded $10,000 value. Ensure backward compatibility with defensive coding (fallback to default if database value is NULL).

---

## User Story

> As a trader in paper trading mode,
> I want my custom paper balance to be used for all simulated trades,
> So that my testing environment reflects my configured balance accurately.

---

## Acceptance Criteria

### AC3.1: Futures Handler Uses DB Balance

**File:** `internal/api/handlers_futures.go`

- [ ] Remove hardcoded balance: `const paperBalance = 10000.0`
- [ ] Query database for `paper_balance_usdt` using authenticated `user_id`
- [ ] Use fetched balance in futures balance response
- [ ] Code change location: Lines 160-167 (current hardcoded section)
- [ ] Maintain existing response format (no breaking changes to API contract)

**Before (Current Code):**
```go
// Line 160-167 (approximate)
if dryRunMode {
    balances = append(balances, FuturesBalance{
        Asset:              "USDT",
        Balance:            "10000.0",  // ← HARDCODED
        AvailableBalance:   "10000.0",  // ← HARDCODED
        CrossWalletBalance: "10000.0",  // ← HARDCODED
    })
}
```

**After (Updated Code):**
```go
if dryRunMode {
    paperBalance, err := h.paperBalanceService.GetPaperBalance(userID, "futures")
    if err != nil {
        h.logger.Warn("Failed to fetch paper balance, using default", "error", err)
        paperBalance = decimal.NewFromFloat(10000.0) // Fallback
    }

    balanceStr := paperBalance.String()
    balances = append(balances, FuturesBalance{
        Asset:              "USDT",
        Balance:            balanceStr,
        AvailableBalance:   balanceStr,
        CrossWalletBalance: balanceStr,
    })
}
```

---

### AC3.2: Spot Handler Uses DB Balance

**File:** `internal/api/handlers_spot.go` (or equivalent)

- [ ] Identify spot trading balance response logic
- [ ] Replace any hardcoded paper balance with database query
- [ ] Use `paper_balance_usdt` from `user_trading_configs` where `trading_type = 'spot'`
- [ ] Maintain existing response format

**Note:** If spot balance is handled in a separate file, apply same pattern as AC3.1.

---

### AC3.3: Backward Compatibility & Defensive Coding

- [ ] If database query fails, log warning and fallback to $10,000
- [ ] If `paper_balance_usdt` is NULL (should never happen post-migration), fallback to $10,000
- [ ] No panic or application crash if database unavailable
- [ ] Log all fallback scenarios for monitoring

**Fallback Pattern:**
```go
paperBalance, err := h.paperBalanceService.GetPaperBalance(userID, tradingType)
if err != nil || paperBalance.IsZero() {
    h.logger.Warn("Using fallback paper balance", "user_id", userID, "error", err)
    paperBalance = decimal.NewFromFloat(10000.0)
}
```

---

### AC3.4: Position Entry/Exit Balance Updates

- [ ] Verify existing logic decrements balance on position entry
- [ ] Verify existing logic increments balance on position exit (with P/L)
- [ ] Ensure balance updates use database-backed values, not in-memory cache
- [ ] Confirm `UpdatePaperBalance()` repository method is called after trade

**Critical:** If current implementation uses in-memory balance tracking, refactor to use database as source of truth.

---

### AC3.5: No Breaking Changes to Existing API

- [ ] Response format unchanged (JSON structure identical)
- [ ] Field names unchanged (`balance`, `availableBalance`, etc.)
- [ ] HTTP status codes unchanged
- [ ] Frontend requires no changes to API consumption

---

## Technical Implementation Notes

### Files to Modify

1. **`internal/api/handlers_futures.go`** (Primary)
   - Update `GetFuturesBalance()` or equivalent handler
   - Lines ~160-167 (hardcoded balance section)

2. **`internal/api/handlers_spot.go`** (If exists)
   - Update spot balance handler similarly

3. **Dependency Injection:**
   - Ensure `Handler` struct has access to `PaperBalanceService`
   - Update constructor to inject service dependency

---

### Handler Struct Update

```go
type Handler struct {
    // Existing fields...
    binanceClient       *binance.Client
    db                  *sql.DB

    // NEW: Add paper balance service
    paperBalanceService *services.PaperBalanceService
}

func NewHandler(db *sql.DB, binanceClient *binance.Client, paperBalanceService *services.PaperBalanceService) *Handler {
    return &Handler{
        db:                  db,
        binanceClient:       binanceClient,
        paperBalanceService: paperBalanceService,
    }
}
```

---

### Example: Futures Balance Handler Update

**File:** `internal/api/handlers_futures.go`

```go
func (h *Handler) GetFuturesBalance(c *gin.Context) {
    userID := c.GetInt64("user_id") // From JWT middleware

    // Get trading config to check dry_run_mode
    config, err := h.tradingConfigRepo.GetTradingConfig(userID, "futures")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get config"})
        return
    }

    var balances []FuturesBalance

    if config.DryRunMode {
        // UPDATED: Fetch paper balance from database
        paperBalance, err := h.paperBalanceService.GetPaperBalance(userID, "futures")
        if err != nil {
            h.logger.Warn("Failed to fetch paper balance, using fallback",
                "user_id", userID,
                "error", err,
            )
            paperBalance = decimal.NewFromFloat(10000.0) // Fallback
        }

        balanceStr := paperBalance.String()

        balances = append(balances, FuturesBalance{
            Asset:              "USDT",
            Balance:            balanceStr,
            AvailableBalance:   balanceStr,
            CrossWalletBalance: balanceStr,
        })
    } else {
        // Real trading mode: Call Binance API (existing logic unchanged)
        balances, err = h.binanceClient.GetFuturesBalance(...)
        if err != nil {
            c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch real balance"})
            return
        }
    }

    c.JSON(http.StatusOK, gin.H{"balances": balances})
}
```

---

### Position Entry/Exit Updates

**File:** `internal/api/handlers_futures.go` (or `handlers_trading.go`)

**Example: Position Entry**
```go
func (h *Handler) OpenPosition(c *gin.Context) {
    // ... existing logic ...

    if config.DryRunMode {
        // Deduct position cost from paper balance
        currentBalance, _ := h.paperBalanceService.GetPaperBalance(userID, "futures")
        newBalance := currentBalance.Sub(positionCost)

        err := h.paperBalanceService.UpdatePaperBalance(userID, "futures", newBalance)
        if err != nil {
            h.logger.Error("Failed to update paper balance after position entry", "error", err)
            // Continue anyway - balance will be corrected on next sync
        }
    }

    // ... rest of position entry logic ...
}
```

**Example: Position Exit**
```go
func (h *Handler) ClosePosition(c *gin.Context) {
    // ... existing logic ...

    if config.DryRunMode {
        // Add profit/loss to paper balance
        currentBalance, _ := h.paperBalanceService.GetPaperBalance(userID, "futures")
        newBalance := currentBalance.Add(profitOrLoss)

        err := h.paperBalanceService.UpdatePaperBalance(userID, "futures", newBalance)
        if err != nil {
            h.logger.Error("Failed to update paper balance after position exit", "error", err)
        }
    }

    // ... rest of position exit logic ...
}
```

---

## Testing Requirements

### Unit Tests

**File:** `internal/api/handlers_futures_test.go`

```go
func TestGetFuturesBalance_DryRunMode_UsesDBBalance(t *testing.T) {
    // Mock paperBalanceService.GetPaperBalance() returns 5000.0
    // Mock config.DryRunMode = true
    // Call GetFuturesBalance handler
    // Assert response contains "5000.00000000"
}

func TestGetFuturesBalance_DBError_UsesFallback(t *testing.T) {
    // Mock paperBalanceService.GetPaperBalance() returns error
    // Mock config.DryRunMode = true
    // Call GetFuturesBalance handler
    // Assert response contains "10000.00000000" (fallback)
    // Assert warning logged
}

func TestGetFuturesBalance_RealMode_CallsBinanceAPI(t *testing.T) {
    // Mock config.DryRunMode = false
    // Mock binanceClient.GetFuturesBalance() returns real balance
    // Call GetFuturesBalance handler
    // Assert paperBalanceService NOT called
    // Assert response contains Binance API balance
}
```

**File:** `internal/api/handlers_trading_test.go`

```go
func TestOpenPosition_DryRunMode_DecrementsBalance(t *testing.T) {
    // Set initial paper balance = 10000
    // Open position with cost = 2000
    // Assert paper balance updated to 8000
}

func TestClosePosition_DryRunMode_AddsProfit(t *testing.T) {
    // Set initial paper balance = 8000
    // Close position with profit = 500
    // Assert paper balance updated to 8500
}
```

---

### Integration Tests

```go
func TestPaperTradingWorkflow_EndToEnd(t *testing.T) {
    // 1. Set paper balance to $5000 (via API)
    // 2. Enable dry_run_mode
    // 3. GET futures balance (expect $5000)
    // 4. Open position (cost $2000)
    // 5. GET futures balance (expect $3000)
    // 6. Close position (profit $300)
    // 7. GET futures balance (expect $3300)
}
```

---

### Manual Testing

```bash
# Setup: Update paper balance
curl -X PUT -H "Authorization: Bearer <JWT>" \
  -H "Content-Type: application/json" \
  -d '{"balance": 7500.0}' \
  http://localhost:8094/api/settings/paper-balance/futures

# Test: Get futures balance (should show 7500)
curl -H "Authorization: Bearer <JWT>" \
  http://localhost:8094/api/futures/balance

# Expected Response:
# {
#   "balances": [
#     {
#       "asset": "USDT",
#       "balance": "7500.00000000",
#       "availableBalance": "7500.00000000",
#       "crossWalletBalance": "7500.00000000"
#     }
#   ]
# }
```

---

## Dependencies

### Prerequisites
- **Story 1:** Database migration completed
- **Story 2:** Backend API endpoints implemented (`PaperBalanceService` available)

### Blocks
- **Story 4:** Frontend UI (indirectly - UI needs working trading logic to test)

---

## Definition of Done

- [ ] All acceptance criteria met (AC3.1 - AC3.5)
- [ ] Hardcoded balance removed from futures handler
- [ ] Hardcoded balance removed from spot handler (if applicable)
- [ ] Database query integrated for paper balance retrieval
- [ ] Fallback to $10,000 implemented for error cases
- [ ] Position entry/exit logic updated to use DB balance
- [ ] All unit tests passing
- [ ] Integration tests passing
- [ ] Manual testing completed (balance reflects database value)
- [ ] No breaking changes to API response format
- [ ] Code review approved
- [ ] Logs confirm database balance being used

---

## Rollback Plan

If issues arise post-deployment:

1. **Immediate Rollback:** Revert handlers to use hardcoded $10,000
2. **Database Intact:** No data loss (migration remains, handlers just ignore it)
3. **Re-deploy:** Fix issues and re-deploy when ready

**Rollback Code (Emergency):**
```go
// Temporary hardcoded fallback (emergency only)
const EMERGENCY_FALLBACK = 10000.0

if dryRunMode {
    balanceStr := fmt.Sprintf("%.8f", EMERGENCY_FALLBACK)
    balances = append(balances, FuturesBalance{
        Asset:   "USDT",
        Balance: balanceStr,
        // ... rest of fields ...
    })
}
```

---

## Notes for Developer

- **Service Injection:** Ensure `PaperBalanceService` is properly injected into Handler constructor
- **Logging:** Add structured logs at INFO level when database balance is fetched successfully
- **Testing Dry Run Mode:** Use test user account with `dry_run_mode = true` for manual testing
- **Performance:** Database query adds ~50ms latency - acceptable for balance checks
- **Caching Consideration:** Do NOT cache balance in memory (database is source of truth for consistency)

---

## Related Stories

- **Story 1:** Database migration (prerequisite)
- **Story 2:** Backend API endpoints (prerequisite)
- **Story 4:** Frontend UI (parallel development possible)
