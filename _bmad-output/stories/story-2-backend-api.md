# Story 2: Backend API - Paper Balance Endpoints

**Story ID:** PAPER-002
**Epic:** Editable Paper Trading Balance
**Priority:** High
**Estimated Effort:** 6 hours
**Author:** Bob (Scrum Master)
**Status:** Ready for Development

---

## Description

Implement three REST API endpoints for managing paper trading balances: GET (retrieve balance), PUT (manual update), and POST (sync from Binance). Includes repository layer, service layer, and API handler implementation.

---

## User Story

> As a frontend developer,
> I need REST API endpoints to manage paper trading balances,
> So that the UI can retrieve, update, and sync balances from the backend.

---

## Acceptance Criteria

### AC2.1: GET Endpoint - Retrieve Paper Balance

**Endpoint:** `GET /api/settings/paper-balance/:trading_type`

- [ ] Returns current paper balance for specified trading type
- [ ] Validates `trading_type` parameter (must be "spot" or "futures")
- [ ] Requires JWT authentication (returns 401 if missing/invalid)
- [ ] Returns user's own balance only (cannot access other users' balances)
- [ ] Response format matches specification:
  ```json
  {
    "trading_type": "futures",
    "paper_balance_usdt": "10000.00000000",
    "dry_run_mode": true
  }
  ```
- [ ] Returns 400 if `trading_type` invalid
- [ ] Returns 404 if user has no trading config for specified type

---

### AC2.2: PUT Endpoint - Manual Balance Update

**Endpoint:** `PUT /api/settings/paper-balance/:trading_type`

- [ ] Accepts JSON body: `{ "balance": 5000.50 }`
- [ ] Validates balance range: $10 ≤ balance ≤ $1,000,000
- [ ] Updates `paper_balance_usdt` in database for authenticated user
- [ ] Returns updated balance in response:
  ```json
  {
    "trading_type": "spot",
    "paper_balance_usdt": "5000.50000000",
    "message": "Paper balance updated successfully"
  }
  ```
- [ ] Returns 400 if balance out of range with clear error message:
  ```json
  {
    "error": "Balance must be between $10 and $1,000,000"
  }
  ```
- [ ] Returns 400 if balance is not a valid number
- [ ] Returns 401 if not authenticated
- [ ] Uses `decimal.Decimal` type (no precision loss)

---

### AC2.3: POST Endpoint - Sync from Binance

**Endpoint:** `POST /api/settings/sync-paper-balance/:trading_type`

- [ ] No request body required
- [ ] Fetches USDT balance from Binance API:
  - **Spot:** GET `/api/v3/account` → extract USDT from balances array
  - **Futures:** GET `/fapi/v2/balance` → extract USDT balance
- [ ] Updates `paper_balance_usdt` in database with fetched value
- [ ] Returns updated balance:
  ```json
  {
    "trading_type": "futures",
    "paper_balance_usdt": "3547.82000000",
    "synced_from": "binance_futures_account",
    "message": "Paper balance synced successfully"
  }
  ```
- [ ] Returns 400 if user has no Binance API credentials:
  ```json
  {
    "error": "Binance API credentials not configured",
    "action_required": "Please add your API keys in Settings"
  }
  ```
- [ ] Returns 502 if Binance API call fails (timeout, network error):
  ```json
  {
    "error": "Failed to fetch balance from Binance",
    "details": "API request timeout",
    "retry_suggested": true
  }
  ```
- [ ] Returns 503 if Binance rate limit exceeded:
  ```json
  {
    "error": "Binance API rate limit exceeded",
    "retry_after_seconds": 60
  }
  ```
- [ ] Sync operation is idempotent (safe to retry)

---

### AC2.4: Authentication & Authorization

- [ ] All three endpoints require valid JWT token
- [ ] Extract `user_id` from JWT claims
- [ ] User can ONLY access their own paper balance (no cross-user access)
- [ ] Return 401 Unauthorized if JWT missing or expired
- [ ] Return 403 Forbidden if user attempts to access another user's balance (defense-in-depth)

---

### AC2.5: Error Handling & Logging

- [ ] All database errors logged with context (user_id, trading_type, error message)
- [ ] Binance API errors logged with full details (status code, response body)
- [ ] Do NOT expose internal error details to client (generic 500 message)
- [ ] Log successful sync operations with fetched balance value (audit trail)
- [ ] Handle edge case: User has no `user_trading_configs` row → create with defaults

---

## Technical Implementation Notes

### File Structure

```
internal/
├── database/
│   └── trading_config_repository.go  ← NEW: Repository methods
├── services/
│   └── paper_balance_service.go      ← NEW: Business logic
├── api/
│   ├── handlers_paper_balance.go     ← NEW: API handlers
│   └── routes.go                     ← MODIFY: Add new routes
└── binance/
    ├── spot_client.go                ← MODIFY: Add GetUSDTBalance method
    └── futures_client.go             ← MODIFY: Add GetUSDTBalance method
```

---

### Repository Layer (`internal/database/trading_config_repository.go`)

```go
package database

import (
    "database/sql"
    "github.com/shopspring/decimal"
)

type TradingConfigRepository struct {
    db *sql.DB
}

// GetPaperBalance retrieves the paper balance for a specific user and trading type
func (r *TradingConfigRepository) GetPaperBalance(userID int64, tradingType string) (decimal.Decimal, error) {
    var balance decimal.Decimal
    query := `
        SELECT paper_balance_usdt
        FROM user_trading_configs
        WHERE user_id = $1 AND trading_type = $2
    `
    err := r.db.QueryRow(query, userID, tradingType).Scan(&balance)
    if err == sql.ErrNoRows {
        return decimal.Zero, ErrConfigNotFound
    }
    return balance, err
}

// UpdatePaperBalance updates the paper balance for a specific user and trading type
func (r *TradingConfigRepository) UpdatePaperBalance(userID int64, tradingType string, balance decimal.Decimal) error {
    query := `
        UPDATE user_trading_configs
        SET paper_balance_usdt = $1, updated_at = NOW()
        WHERE user_id = $2 AND trading_type = $3
    `
    result, err := r.db.Exec(query, balance, userID, tradingType)
    if err != nil {
        return err
    }

    rowsAffected, _ := result.RowsAffected()
    if rowsAffected == 0 {
        // No config exists - create one with defaults
        return r.createDefaultConfig(userID, tradingType, balance)
    }
    return nil
}

// GetTradingConfig retrieves full config (including dry_run_mode)
func (r *TradingConfigRepository) GetTradingConfig(userID int64, tradingType string) (*TradingConfig, error) {
    var config TradingConfig
    query := `
        SELECT user_id, trading_type, dry_run_mode, paper_balance_usdt
        FROM user_trading_configs
        WHERE user_id = $1 AND trading_type = $2
    `
    err := r.db.QueryRow(query, userID, tradingType).Scan(
        &config.UserID,
        &config.TradingType,
        &config.DryRunMode,
        &config.PaperBalanceUSDT,
    )
    return &config, err
}

func (r *TradingConfigRepository) createDefaultConfig(userID int64, tradingType string, balance decimal.Decimal) error {
    query := `
        INSERT INTO user_trading_configs (user_id, trading_type, dry_run_mode, paper_balance_usdt)
        VALUES ($1, $2, false, $3)
    `
    _, err := r.db.Exec(query, userID, tradingType, balance)
    return err
}
```

---

### Service Layer (`internal/services/paper_balance_service.go`)

```go
package services

import (
    "errors"
    "fmt"
    "github.com/shopspring/decimal"
)

var (
    ErrNoAPICredentials    = errors.New("binance API credentials not configured")
    ErrBalanceOutOfRange   = errors.New("balance must be between $10 and $1,000,000")
    ErrBinanceAPIFailed    = errors.New("failed to fetch balance from Binance")
)

type PaperBalanceService struct {
    repo           *database.TradingConfigRepository
    spotClient     *binance.SpotClient
    futuresClient  *binance.FuturesClient
    authService    *auth.Service
}

func NewPaperBalanceService(repo *database.TradingConfigRepository, spotClient *binance.SpotClient, futuresClient *binance.FuturesClient, authService *auth.Service) *PaperBalanceService {
    return &PaperBalanceService{
        repo:          repo,
        spotClient:    spotClient,
        futuresClient: futuresClient,
        authService:   authService,
    }
}

// GetPaperBalance retrieves the current paper balance
func (s *PaperBalanceService) GetPaperBalance(userID int64, tradingType string) (decimal.Decimal, bool, error) {
    config, err := s.repo.GetTradingConfig(userID, tradingType)
    if err != nil {
        return decimal.Zero, false, err
    }
    return config.PaperBalanceUSDT, config.DryRunMode, nil
}

// UpdatePaperBalance validates and updates the paper balance
func (s *PaperBalanceService) UpdatePaperBalance(userID int64, tradingType string, balance decimal.Decimal) error {
    // Validate range
    minBalance := decimal.NewFromFloat(10.0)
    maxBalance := decimal.NewFromFloat(1000000.0)

    if balance.LessThan(minBalance) || balance.GreaterThan(maxBalance) {
        return ErrBalanceOutOfRange
    }

    return s.repo.UpdatePaperBalance(userID, tradingType, balance)
}

// SyncFromBinance fetches real balance and updates paper balance
func (s *PaperBalanceService) SyncFromBinance(userID int64, tradingType string) (decimal.Decimal, error) {
    // Get user's API credentials
    apiKey, secretKey, err := s.authService.GetBinanceCredentials(userID)
    if err != nil || apiKey == "" {
        return decimal.Zero, ErrNoAPICredentials
    }

    // Fetch balance from Binance
    var balance decimal.Decimal
    if tradingType == "spot" {
        balance, err = s.spotClient.GetUSDTBalance(apiKey, secretKey)
    } else if tradingType == "futures" {
        balance, err = s.futuresClient.GetUSDTBalance(apiKey, secretKey)
    } else {
        return decimal.Zero, errors.New("invalid trading type")
    }

    if err != nil {
        return decimal.Zero, fmt.Errorf("%w: %v", ErrBinanceAPIFailed, err)
    }

    // Update database
    err = s.repo.UpdatePaperBalance(userID, tradingType, balance)
    if err != nil {
        return decimal.Zero, err
    }

    return balance, nil
}
```

---

### API Handler Layer (`internal/api/handlers_paper_balance.go`)

```go
package api

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/shopspring/decimal"
)

// GetPaperBalance handles GET /api/settings/paper-balance/:trading_type
func (h *Handler) GetPaperBalance(c *gin.Context) {
    userID := c.GetInt64("user_id") // From JWT middleware
    tradingType := c.Param("trading_type")

    if !isValidTradingType(tradingType) {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Invalid trading type. Must be 'spot' or 'futures'",
        })
        return
    }

    balance, dryRunMode, err := h.paperBalanceService.GetPaperBalance(userID, tradingType)
    if err != nil {
        h.logger.Error("Failed to get paper balance", "user_id", userID, "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve balance"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "trading_type":       tradingType,
        "paper_balance_usdt": balance.String(),
        "dry_run_mode":       dryRunMode,
    })
}

// UpdatePaperBalance handles PUT /api/settings/paper-balance/:trading_type
func (h *Handler) UpdatePaperBalance(c *gin.Context) {
    userID := c.GetInt64("user_id")
    tradingType := c.Param("trading_type")

    if !isValidTradingType(tradingType) {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Invalid trading type. Must be 'spot' or 'futures'",
        })
        return
    }

    var req struct {
        Balance float64 `json:"balance" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
        return
    }

    balance := decimal.NewFromFloat(req.Balance)

    err := h.paperBalanceService.UpdatePaperBalance(userID, tradingType, balance)
    if err == services.ErrBalanceOutOfRange {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Balance must be between $10 and $1,000,000",
        })
        return
    }
    if err != nil {
        h.logger.Error("Failed to update paper balance", "user_id", userID, "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update balance"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "trading_type":       tradingType,
        "paper_balance_usdt": balance.String(),
        "message":            "Paper balance updated successfully",
    })
}

// SyncPaperBalance handles POST /api/settings/sync-paper-balance/:trading_type
func (h *Handler) SyncPaperBalance(c *gin.Context) {
    userID := c.GetInt64("user_id")
    tradingType := c.Param("trading_type")

    if !isValidTradingType(tradingType) {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Invalid trading type. Must be 'spot' or 'futures'",
        })
        return
    }

    balance, err := h.paperBalanceService.SyncFromBinance(userID, tradingType)

    if err == services.ErrNoAPICredentials {
        c.JSON(http.StatusBadRequest, gin.H{
            "error":           "Binance API credentials not configured",
            "action_required": "Please add your API keys in Settings",
        })
        return
    }

    if errors.Is(err, services.ErrBinanceAPIFailed) {
        c.JSON(http.StatusBadGateway, gin.H{
            "error":           "Failed to fetch balance from Binance",
            "details":         err.Error(),
            "retry_suggested": true,
        })
        return
    }

    if err != nil {
        h.logger.Error("Failed to sync paper balance", "user_id", userID, "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync balance"})
        return
    }

    h.logger.Info("Paper balance synced successfully", "user_id", userID, "balance", balance)

    c.JSON(http.StatusOK, gin.H{
        "trading_type":       tradingType,
        "paper_balance_usdt": balance.String(),
        "synced_from":        fmt.Sprintf("binance_%s_account", tradingType),
        "message":            "Paper balance synced successfully",
    })
}

func isValidTradingType(t string) bool {
    return t == "spot" || t == "futures"
}
```

---

### Route Registration (`internal/api/routes.go`)

```go
// Add to existing route setup
func SetupRoutes(r *gin.Engine, handler *Handler) {
    api := r.Group("/api")
    {
        // Existing routes...

        // Paper balance endpoints (requires auth middleware)
        settings := api.Group("/settings")
        settings.Use(authMiddleware)
        {
            settings.GET("/paper-balance/:trading_type", handler.GetPaperBalance)
            settings.PUT("/paper-balance/:trading_type", handler.UpdatePaperBalance)
            settings.POST("/sync-paper-balance/:trading_type", handler.SyncPaperBalance)
        }
    }
}
```

---

### Binance Client Updates

**File:** `internal/binance/spot_client.go`

```go
// Add method to fetch USDT balance
func (c *SpotClient) GetUSDTBalance(apiKey, secretKey string) (decimal.Decimal, error) {
    // Call Binance API: GET /api/v3/account
    // Parse response, find USDT in balances array
    // Return balance as decimal.Decimal
}
```

**File:** `internal/binance/futures_client.go`

```go
// Add method to fetch USDT balance
func (c *FuturesClient) GetUSDTBalance(apiKey, secretKey string) (decimal.Decimal, error) {
    // Call Binance API: GET /fapi/v2/balance
    // Parse response, find USDT balance
    // Return balance as decimal.Decimal
}
```

---

## Testing Requirements

### Unit Tests

**File:** `internal/services/paper_balance_service_test.go`

```go
func TestUpdatePaperBalance_ValidatesRange(t *testing.T) {
    // Test balance < $10 returns ErrBalanceOutOfRange
    // Test balance > $1M returns ErrBalanceOutOfRange
    // Test balance within range succeeds
}

func TestSyncFromBinance_NoAPICredentials(t *testing.T) {
    // Mock authService.GetBinanceCredentials() returns error
    // Assert returns ErrNoAPICredentials
}

func TestSyncFromBinance_BinanceAPIFailure(t *testing.T) {
    // Mock binance client returns error
    // Assert returns ErrBinanceAPIFailed
}
```

**File:** `internal/api/handlers_paper_balance_test.go`

```go
func TestGetPaperBalance_Success(t *testing.T) {
    // Mock service returns balance
    // Assert 200 response with correct JSON
}

func TestUpdatePaperBalance_InvalidRange(t *testing.T) {
    // Send balance = 5 (below minimum)
    // Assert 400 response with error message
}

func TestSyncPaperBalance_Success(t *testing.T) {
    // Mock service returns synced balance
    // Assert 200 response with synced_from field
}
```

### Integration Tests

```go
func TestPaperBalanceAPI_EndToEnd(t *testing.T) {
    // 1. GET balance (expect default 10000)
    // 2. PUT balance = 5000
    // 3. GET balance (expect 5000)
    // 4. Verify database row updated
}
```

### Manual Testing (Postman/curl)

```bash
# GET paper balance
curl -H "Authorization: Bearer <JWT>" \
  http://localhost:8094/api/settings/paper-balance/futures

# PUT update balance
curl -X PUT -H "Authorization: Bearer <JWT>" \
  -H "Content-Type: application/json" \
  -d '{"balance": 5000.50}' \
  http://localhost:8094/api/settings/paper-balance/spot

# POST sync from Binance
curl -X POST -H "Authorization: Bearer <JWT>" \
  http://localhost:8094/api/settings/sync-paper-balance/futures
```

---

## Dependencies

### Prerequisite
- **Story 1:** Database migration completed (table has `paper_balance_usdt` column)

### Blocks
- **Story 3:** Trading logic update (needs API to function)
- **Story 4:** Frontend UI (needs API endpoints to call)

---

## Definition of Done

- [ ] All acceptance criteria met (AC2.1 - AC2.5)
- [ ] Repository methods implemented and tested
- [ ] Service layer implemented and tested
- [ ] API handlers implemented and tested
- [ ] Routes registered correctly
- [ ] Binance client methods implemented
- [ ] All unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Manual API testing completed (all 3 endpoints)
- [ ] Error handling verified (all error codes tested)
- [ ] Code review approved
- [ ] No regressions in existing API endpoints

---

## Notes for Developer

- **Decimal Library:** Install `github.com/shopspring/decimal` if not already in dependencies
- **JWT Middleware:** Ensure existing auth middleware extracts `user_id` into Gin context
- **Logging:** Use structured logging (e.g., `zerolog`, `logrus`) - include user_id and trading_type in all logs
- **Binance API:** Refer to existing Binance client implementations for auth header format

---

## Related Stories

- **Story 1:** Database migration (prerequisite)
- **Story 3:** Trading logic update (blocked by this story)
- **Story 4:** Frontend UI (blocked by this story)
