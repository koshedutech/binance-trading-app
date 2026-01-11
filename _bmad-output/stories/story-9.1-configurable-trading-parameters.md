# Story 9.1: Make Hardcoded Trading Parameters User-Configurable

**Story ID:** CONFIG-9.1
**Epic:** Epic 9 - User-Configurable Trading Parameters
**Priority:** P1 (High - Enhances User Customization)
**Estimated Effort:** 16 hours
**Author:** Claude Code Agent
**Status:** Ready for Development
**Depends On:** Story 5.2, Epic 6 (Database-First Configuration & Cache)

---

## Problem Statement

### Current State

A comprehensive code audit identified **20+ hardcoded trading values** scattered across the codebase that affect trading decisions but cannot be customized by users:

| Category | Hardcoded Values | Impact |
|----------|-----------------|--------|
| **Early Profit Booking** | 3%, 15% ROI thresholds | When profits are automatically booked |
| **Order Placement** | 0.1% buffer for slippage | Limit order placement relative to market |
| **Retry Limits** | 3 attempts fixed | How many times failed orders retry |
| **Protection Timeout** | 30 seconds | How long before position flagged unprotected |
| **Circuit Breaker** | $100/hr, $300/day | Loss limits that pause trading |
| **Paper Balance** | $10,000 default | Starting balance for paper trading |
| **Binance Fees** | 0.05% taker, 0.02% maker | Fee calculations (VIP0 assumed) |
| **TP Gain Levels** | 1%, 2%, 3%, 4% | Take-profit trigger percentages |
| **Trailing Stop** | 3% for position mode | Default trailing percentage |
| **ADX Threshold** | 15.0 | Trend strength requirement |
| **Funding Rate** | 0.1% max threshold | When to avoid high funding |

### Expected Behavior

- All trading parameters loaded from user's database configuration
- Each user can customize parameters to their trading style
- Sensible defaults provided for new users
- Validation ensures parameters stay within safe bounds
- Admin can set global defaults that new users inherit
- Parameters take effect immediately (no restart required)

---

## User Story

> As a trader using Ginie Autopilot,
> I want to customize trading parameters like profit booking thresholds, retry limits, and fee rates,
> So that I can fine-tune the bot to match my trading style, risk tolerance, and Binance VIP tier.

---

## Technical Architecture

### Configuration Groups

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  NEW CONFIGURATION SECTIONS (Added to user_settings)                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  1. PROFIT BOOKING CONFIG                                                    │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ early_profit_min_roi:    3.0   // Min ROI % to enable early booking   │   │
│  │ early_profit_max_roi:   15.0   // Max ROI % cap                       │   │
│  │ min_profit_threshold:    0.1   // Min % to consider as profit         │   │
│  │ profit_booking_enabled: true   // Master toggle                       │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  2. ORDER EXECUTION CONFIG                                                   │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ slippage_buffer_pct:     0.1   // 0.1% buffer for limit orders        │   │
│  │ max_order_retries:       3     // Retry attempts for failed orders    │   │
│  │ order_timeout_ms:     5000     // Order placement timeout             │   │
│  │ protection_timeout_sec: 30     // Unprotected position alert time     │   │
│  │ max_heal_attempts:       3     // SL/TP heal retry limit              │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  3. FEE CONFIG                                                               │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ taker_fee_percent:      0.05   // Taker fee (VIP0 = 0.05%)            │   │
│  │ maker_fee_percent:      0.02   // Maker fee (VIP0 = 0.02%)            │   │
│  │ auto_detect_vip:       false   // Auto-detect from Binance API        │   │
│  │ vip_level:                 0   // Manual VIP level setting            │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  4. PAPER TRADING CONFIG                                                     │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ paper_balance_usd:   10000.0   // Starting paper trading balance      │   │
│  │ paper_margin_buffer:    0.05   // 5% margin buffer                    │   │
│  │ paper_fee_simulation:  true    // Simulate fees in paper mode         │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  5. TAKE PROFIT LEVELS CONFIG                                                │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ default_tp_levels: [1.0, 2.0, 3.0, 4.0]  // TP gain percentages       │   │
│  │ position_trailing_pct:   3.0   // Trailing % for position mode        │   │
│  │ swing_trailing_pct:      2.0   // Trailing % for swing mode           │   │
│  │ scalp_trailing_pct:      1.0   // Trailing % for scalp mode           │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  6. TECHNICAL THRESHOLDS CONFIG                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ adx_threshold:          15.0   // ADX trend strength requirement      │   │
│  │ max_funding_rate:      0.001   // 0.1% max funding rate               │   │
│  │ high_funding_multiplier: 2.0   // 2x funding = elevated threshold     │   │
│  │ volatility_low:          0.5   // Below 0.5% = very low volatility    │   │
│  │ volatility_high:         3.0   // Above 3% = very high volatility     │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  7. POSITION SIZE THRESHOLDS                                                 │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ min_recommended_usd:   100.0   // Warning threshold for small pos     │   │
│  │ optimal_min_usd:       200.0   // Recommended minimum position        │   │
│  │ breakeven_buffer_pct:    0.1   // Buffer above entry for breakeven    │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Database Schema Extension

```sql
-- Extend user_settings with trading parameters (JSONB column exists)
-- These fields will be stored in the config_json column

-- Example structure in config_json:
{
  "trading_parameters": {
    "profit_booking": {
      "early_profit_min_roi": 3.0,
      "early_profit_max_roi": 15.0,
      "min_profit_threshold": 0.1,
      "enabled": true
    },
    "order_execution": {
      "slippage_buffer_pct": 0.1,
      "max_order_retries": 3,
      "order_timeout_ms": 5000,
      "protection_timeout_sec": 30,
      "max_heal_attempts": 3
    },
    "fees": {
      "taker_fee_percent": 0.05,
      "maker_fee_percent": 0.02,
      "auto_detect_vip": false,
      "vip_level": 0
    },
    "paper_trading": {
      "paper_balance_usd": 10000.0,
      "paper_margin_buffer": 0.05,
      "paper_fee_simulation": true
    },
    "tp_levels": {
      "default_tp_levels": [1.0, 2.0, 3.0, 4.0],
      "position_trailing_pct": 3.0,
      "swing_trailing_pct": 2.0,
      "scalp_trailing_pct": 1.0
    },
    "technical_thresholds": {
      "adx_threshold": 15.0,
      "max_funding_rate": 0.001,
      "high_funding_multiplier": 2.0,
      "volatility_low": 0.5,
      "volatility_high": 3.0
    },
    "position_sizing": {
      "min_recommended_usd": 100.0,
      "optimal_min_usd": 200.0,
      "breakeven_buffer_pct": 0.1
    }
  }
}
```

---

## Acceptance Criteria

### AC9.1.1: Define TradingParameters Struct
- [ ] Create `TradingParameters` struct in `settings.go` with all 7 config groups
- [ ] Add JSON tags for database serialization
- [ ] Create `DefaultTradingParameters()` with safe default values
- [ ] Add to `AutopilotSettings` as new field

### AC9.1.2: Database Loading
- [ ] Load `TradingParameters` from user's database config
- [ ] Fallback to `DefaultTradingParameters()` if not found
- [ ] Merge partial updates (user can update just one field)
- [ ] Cache loaded params with 5-minute TTL

### AC9.1.3: Replace Hardcoded Values
- [ ] Replace all 20+ hardcoded values in `ginie_autopilot.go` with config reads
- [ ] Replace hardcoded values in `handlers_mode.go` (fees, position thresholds)
- [ ] Replace hardcoded values in `handlers_futures.go` (paper balance)
- [ ] Replace hardcoded values in `dynamic_sltp.go` (volatility thresholds)

### AC9.1.4: API Endpoints
- [ ] `GET /api/futures/settings/trading-params` - Get current trading parameters
- [ ] `PUT /api/futures/settings/trading-params` - Update trading parameters (partial)
- [ ] `POST /api/futures/settings/trading-params/reset` - Reset to defaults
- [ ] All endpoints require authentication

### AC9.1.5: Validation
- [ ] Validate percentage ranges (0-100%)
- [ ] Validate fee rates (0-1%)
- [ ] Validate retry limits (1-10)
- [ ] Validate timeout values (1000-60000ms)
- [ ] Validate paper balance ($100-$1,000,000)
- [ ] Return detailed error messages for invalid values

### AC9.1.6: Immediate Effect
- [ ] Parameter changes take effect immediately (no restart)
- [ ] Cache invalidated on update
- [ ] Running autopilot uses new values on next tick

### AC9.1.7: VIP Tier Support (Stretch Goal)
- [ ] Option to auto-detect VIP level from Binance API
- [ ] Map VIP levels to fee rates:
  - VIP0: Taker 0.05%, Maker 0.02%
  - VIP1: Taker 0.04%, Maker 0.018%
  - VIP2: Taker 0.035%, Maker 0.016%
  - etc.
- [ ] Manual override always takes precedence

---

## Technical Implementation

### Task 1: Define TradingParameters Struct

```go
// internal/autopilot/settings.go

// TradingParameters contains all configurable trading values
// that were previously hardcoded across the codebase
type TradingParameters struct {
    ProfitBooking      ProfitBookingConfig      `json:"profit_booking"`
    OrderExecution     OrderExecutionConfig     `json:"order_execution"`
    Fees               FeeConfig                `json:"fees"`
    PaperTrading       PaperTradingConfig       `json:"paper_trading"`
    TPLevels           TPLevelsConfig           `json:"tp_levels"`
    TechnicalThresholds TechnicalThresholdsConfig `json:"technical_thresholds"`
    PositionSizing     PositionSizingConfig     `json:"position_sizing"`
}

// ProfitBookingConfig controls early profit booking behavior
type ProfitBookingConfig struct {
    EarlyProfitMinROI   float64 `json:"early_profit_min_roi"`   // Min ROI % (default: 3.0)
    EarlyProfitMaxROI   float64 `json:"early_profit_max_roi"`   // Max ROI % (default: 15.0)
    MinProfitThreshold  float64 `json:"min_profit_threshold"`   // Min % to book (default: 0.1)
    Enabled             bool    `json:"enabled"`                 // Master toggle
}

// OrderExecutionConfig controls order placement behavior
type OrderExecutionConfig struct {
    SlippageBufferPct    float64 `json:"slippage_buffer_pct"`    // Limit order buffer (default: 0.1)
    MaxOrderRetries      int     `json:"max_order_retries"`      // Retry attempts (default: 3)
    OrderTimeoutMs       int     `json:"order_timeout_ms"`       // Timeout in ms (default: 5000)
    ProtectionTimeoutSec int     `json:"protection_timeout_sec"` // Unprotected alert (default: 30)
    MaxHealAttempts      int     `json:"max_heal_attempts"`      // SL/TP heal retries (default: 3)
}

// FeeConfig controls fee calculations
type FeeConfig struct {
    TakerFeePercent float64 `json:"taker_fee_percent"` // Taker fee % (default: 0.05)
    MakerFeePercent float64 `json:"maker_fee_percent"` // Maker fee % (default: 0.02)
    AutoDetectVIP   bool    `json:"auto_detect_vip"`   // Auto-detect from API
    VIPLevel        int     `json:"vip_level"`         // Manual VIP level (0-9)
}

// PaperTradingConfig controls paper trading simulation
type PaperTradingConfig struct {
    PaperBalanceUSD     float64 `json:"paper_balance_usd"`     // Starting balance (default: 10000)
    PaperMarginBuffer   float64 `json:"paper_margin_buffer"`   // Margin buffer % (default: 0.05)
    PaperFeeSimulation  bool    `json:"paper_fee_simulation"`  // Simulate fees (default: true)
}

// TPLevelsConfig controls take-profit behavior
type TPLevelsConfig struct {
    DefaultTPLevels       []float64 `json:"default_tp_levels"`       // TP % levels (default: [1,2,3,4])
    PositionTrailingPct   float64   `json:"position_trailing_pct"`   // Position mode (default: 3.0)
    SwingTrailingPct      float64   `json:"swing_trailing_pct"`      // Swing mode (default: 2.0)
    ScalpTrailingPct      float64   `json:"scalp_trailing_pct"`      // Scalp mode (default: 1.0)
}

// TechnicalThresholdsConfig controls indicator thresholds
type TechnicalThresholdsConfig struct {
    ADXThreshold          float64 `json:"adx_threshold"`           // Trend strength (default: 15.0)
    MaxFundingRate        float64 `json:"max_funding_rate"`        // Max funding (default: 0.001)
    HighFundingMultiplier float64 `json:"high_funding_multiplier"` // Elevated threshold (default: 2.0)
    VolatilityLow         float64 `json:"volatility_low"`          // Very low vol (default: 0.5)
    VolatilityHigh        float64 `json:"volatility_high"`         // Very high vol (default: 3.0)
}

// PositionSizingConfig controls position size thresholds
type PositionSizingConfig struct {
    MinRecommendedUSD   float64 `json:"min_recommended_usd"`   // Warning threshold (default: 100)
    OptimalMinUSD       float64 `json:"optimal_min_usd"`       // Recommended min (default: 200)
    BreakevenBufferPct  float64 `json:"breakeven_buffer_pct"`  // Above entry (default: 0.1)
}

// DefaultTradingParameters returns safe default values
func DefaultTradingParameters() TradingParameters {
    return TradingParameters{
        ProfitBooking: ProfitBookingConfig{
            EarlyProfitMinROI:  3.0,
            EarlyProfitMaxROI:  15.0,
            MinProfitThreshold: 0.1,
            Enabled:            true,
        },
        OrderExecution: OrderExecutionConfig{
            SlippageBufferPct:    0.1,
            MaxOrderRetries:      3,
            OrderTimeoutMs:       5000,
            ProtectionTimeoutSec: 30,
            MaxHealAttempts:      3,
        },
        Fees: FeeConfig{
            TakerFeePercent: 0.05,
            MakerFeePercent: 0.02,
            AutoDetectVIP:   false,
            VIPLevel:        0,
        },
        PaperTrading: PaperTradingConfig{
            PaperBalanceUSD:    10000.0,
            PaperMarginBuffer:  0.05,
            PaperFeeSimulation: true,
        },
        TPLevels: TPLevelsConfig{
            DefaultTPLevels:     []float64{1.0, 2.0, 3.0, 4.0},
            PositionTrailingPct: 3.0,
            SwingTrailingPct:    2.0,
            ScalpTrailingPct:    1.0,
        },
        TechnicalThresholds: TechnicalThresholdsConfig{
            ADXThreshold:          15.0,
            MaxFundingRate:        0.001,
            HighFundingMultiplier: 2.0,
            VolatilityLow:         0.5,
            VolatilityHigh:        3.0,
        },
        PositionSizing: PositionSizingConfig{
            MinRecommendedUSD:  100.0,
            OptimalMinUSD:      200.0,
            BreakevenBufferPct: 0.1,
        },
    }
}
```

### Task 2: Add Getter Methods to GinieAutopilot

```go
// internal/autopilot/ginie_autopilot.go

// getTradingParameters loads user's trading parameters from database
// Falls back to defaults if not found
func (ga *GinieAutopilot) getTradingParameters() TradingParameters {
    if ga.repo == nil || ga.userID == "" {
        return DefaultTradingParameters()
    }

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    params, err := ga.repo.GetUserTradingParameters(ctx, ga.userID)
    if err != nil {
        log.Printf("[GINIE] Failed to load trading params for user %s: %v, using defaults", ga.userID, err)
        return DefaultTradingParameters()
    }

    if params == nil {
        return DefaultTradingParameters()
    }

    return *params
}

// Helper methods for frequently accessed values
func (ga *GinieAutopilot) getSlippageBuffer() float64 {
    return ga.getTradingParameters().OrderExecution.SlippageBufferPct / 100.0
}

func (ga *GinieAutopilot) getMaxRetries() int {
    return ga.getTradingParameters().OrderExecution.MaxOrderRetries
}

func (ga *GinieAutopilot) getTakerFee() float64 {
    return ga.getTradingParameters().Fees.TakerFeePercent / 100.0
}

func (ga *GinieAutopilot) getMakerFee() float64 {
    return ga.getTradingParameters().Fees.MakerFeePercent / 100.0
}

func (ga *GinieAutopilot) getADXThreshold() float64 {
    return ga.getTradingParameters().TechnicalThresholds.ADXThreshold
}

func (ga *GinieAutopilot) getMaxFundingRate() float64 {
    return ga.getTradingParameters().TechnicalThresholds.MaxFundingRate
}

func (ga *GinieAutopilot) getEarlyProfitMinROI() float64 {
    return ga.getTradingParameters().ProfitBooking.EarlyProfitMinROI
}

func (ga *GinieAutopilot) getEarlyProfitMaxROI() float64 {
    return ga.getTradingParameters().ProfitBooking.EarlyProfitMaxROI
}
```

### Task 3: Replace Hardcoded Values

**Example replacements in ginie_autopilot.go:**

```go
// BEFORE (hardcoded):
const minEarlyThreshold = 3.0
const maxEarlyThreshold = 15.0

// AFTER (configurable):
minEarlyThreshold := ga.getEarlyProfitMinROI()
maxEarlyThreshold := ga.getEarlyProfitMaxROI()

// BEFORE (hardcoded):
closePrice = currentPrice * 0.999  // 0.1% buffer

// AFTER (configurable):
slippageBuffer := ga.getSlippageBuffer()
closePrice = currentPrice * (1.0 - slippageBuffer)

// BEFORE (hardcoded):
const maxTPRetries = 3

// AFTER (configurable):
maxTPRetries := ga.getMaxRetries()

// BEFORE (hardcoded):
minADX := 15.0

// AFTER (configurable):
minADX := ga.getADXThreshold()

// BEFORE (hardcoded):
var maxRate float64 = 0.001

// AFTER (configurable):
maxRate := ga.getMaxFundingRate()
```

### Task 4: Add API Endpoints

```go
// internal/api/handlers_settings.go

// handleGetTradingParameters returns user's trading parameters
// GET /api/futures/settings/trading-params
func (s *Server) handleGetTradingParameters(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        c.JSON(401, gin.H{"error": "Unauthorized"})
        return
    }

    ctx := context.Background()

    params, err := s.repo.GetUserTradingParameters(ctx, userID)
    if err != nil {
        log.Printf("[API] Failed to get trading params for user %s: %v", userID, err)
        c.JSON(500, gin.H{"error": "Failed to load trading parameters"})
        return
    }

    // Use defaults if not found
    if params == nil {
        defaultParams := autopilot.DefaultTradingParameters()
        params = &defaultParams
    }

    c.JSON(200, gin.H{
        "success": true,
        "params":  params,
        "source":  s.getParamsSource(ctx, userID),
    })
}

// handleUpdateTradingParameters updates user's trading parameters
// PUT /api/futures/settings/trading-params
func (s *Server) handleUpdateTradingParameters(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        c.JSON(401, gin.H{"error": "Unauthorized"})
        return
    }

    var req autopilot.TradingParameters
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "Invalid request body"})
        return
    }

    // Validate parameters
    if err := validateTradingParameters(&req); err != nil {
        c.JSON(400, gin.H{"error": fmt.Sprintf("Validation failed: %v", err)})
        return
    }

    ctx := context.Background()

    // Save to database
    if err := s.repo.SaveUserTradingParameters(ctx, userID, &req); err != nil {
        log.Printf("[API] Failed to save trading params for user %s: %v", userID, err)
        c.JSON(500, gin.H{"error": "Failed to save trading parameters"})
        return
    }

    // Invalidate cache
    s.settingsManager.InvalidateTradingParamsCache(userID)

    log.Printf("[API] Updated trading parameters for user %s", userID)

    c.JSON(200, gin.H{
        "success": true,
        "message": "Trading parameters updated",
        "params":  req,
    })
}

// handleResetTradingParameters resets to defaults
// POST /api/futures/settings/trading-params/reset
func (s *Server) handleResetTradingParameters(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        c.JSON(401, gin.H{"error": "Unauthorized"})
        return
    }

    ctx := context.Background()

    // Delete user's custom params (will fallback to defaults)
    if err := s.repo.DeleteUserTradingParameters(ctx, userID); err != nil {
        log.Printf("[API] Failed to reset trading params for user %s: %v", userID, err)
        c.JSON(500, gin.H{"error": "Failed to reset trading parameters"})
        return
    }

    // Invalidate cache
    s.settingsManager.InvalidateTradingParamsCache(userID)

    defaultParams := autopilot.DefaultTradingParameters()

    c.JSON(200, gin.H{
        "success": true,
        "message": "Trading parameters reset to defaults",
        "params":  defaultParams,
    })
}

// validateTradingParameters validates all parameter ranges
func validateTradingParameters(p *autopilot.TradingParameters) error {
    // Profit booking
    if p.ProfitBooking.EarlyProfitMinROI < 0 || p.ProfitBooking.EarlyProfitMinROI > 100 {
        return fmt.Errorf("early_profit_min_roi must be 0-100%%")
    }
    if p.ProfitBooking.EarlyProfitMaxROI < p.ProfitBooking.EarlyProfitMinROI {
        return fmt.Errorf("early_profit_max_roi must be >= early_profit_min_roi")
    }

    // Order execution
    if p.OrderExecution.MaxOrderRetries < 1 || p.OrderExecution.MaxOrderRetries > 10 {
        return fmt.Errorf("max_order_retries must be 1-10")
    }
    if p.OrderExecution.OrderTimeoutMs < 1000 || p.OrderExecution.OrderTimeoutMs > 60000 {
        return fmt.Errorf("order_timeout_ms must be 1000-60000")
    }
    if p.OrderExecution.SlippageBufferPct < 0 || p.OrderExecution.SlippageBufferPct > 5 {
        return fmt.Errorf("slippage_buffer_pct must be 0-5%%")
    }

    // Fees
    if p.Fees.TakerFeePercent < 0 || p.Fees.TakerFeePercent > 1 {
        return fmt.Errorf("taker_fee_percent must be 0-1%%")
    }
    if p.Fees.MakerFeePercent < 0 || p.Fees.MakerFeePercent > 1 {
        return fmt.Errorf("maker_fee_percent must be 0-1%%")
    }
    if p.Fees.VIPLevel < 0 || p.Fees.VIPLevel > 9 {
        return fmt.Errorf("vip_level must be 0-9")
    }

    // Paper trading
    if p.PaperTrading.PaperBalanceUSD < 100 || p.PaperTrading.PaperBalanceUSD > 1000000 {
        return fmt.Errorf("paper_balance_usd must be $100-$1,000,000")
    }

    // TP levels
    if len(p.TPLevels.DefaultTPLevels) < 1 || len(p.TPLevels.DefaultTPLevels) > 10 {
        return fmt.Errorf("default_tp_levels must have 1-10 levels")
    }
    for i, level := range p.TPLevels.DefaultTPLevels {
        if level <= 0 || level > 100 {
            return fmt.Errorf("tp_level[%d] must be 0-100%%", i)
        }
    }

    // Technical thresholds
    if p.TechnicalThresholds.ADXThreshold < 0 || p.TechnicalThresholds.ADXThreshold > 100 {
        return fmt.Errorf("adx_threshold must be 0-100")
    }
    if p.TechnicalThresholds.MaxFundingRate < 0 || p.TechnicalThresholds.MaxFundingRate > 0.1 {
        return fmt.Errorf("max_funding_rate must be 0-0.1 (10%%)")
    }

    return nil
}
```

---

## Files to Modify

| File | Changes |
|------|---------|
| `internal/autopilot/settings.go` | Add `TradingParameters` struct and defaults |
| `internal/autopilot/ginie_autopilot.go` | Replace 15+ hardcoded values with getter calls |
| `internal/autopilot/dynamic_sltp.go` | Replace volatility thresholds |
| `internal/api/handlers_mode.go` | Replace fee and position size hardcodes |
| `internal/api/handlers_futures.go` | Replace paper balance defaults |
| `internal/api/handlers_settings.go` | Add 3 new API endpoints |
| `internal/api/server.go` | Register new routes |
| `internal/database/repository.go` | Add trading params CRUD methods |
| `web/src/services/futuresApi.ts` | Add TypeScript types and API methods |

---

## API Reference

### Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/futures/settings/trading-params` | Get user's trading parameters |
| PUT | `/api/futures/settings/trading-params` | Update trading parameters |
| POST | `/api/futures/settings/trading-params/reset` | Reset to defaults |

### Response: GET Trading Parameters

```json
{
  "success": true,
  "source": "database",
  "params": {
    "profit_booking": {
      "early_profit_min_roi": 3.0,
      "early_profit_max_roi": 15.0,
      "min_profit_threshold": 0.1,
      "enabled": true
    },
    "order_execution": {
      "slippage_buffer_pct": 0.1,
      "max_order_retries": 3,
      "order_timeout_ms": 5000,
      "protection_timeout_sec": 30,
      "max_heal_attempts": 3
    },
    "fees": {
      "taker_fee_percent": 0.04,
      "maker_fee_percent": 0.018,
      "auto_detect_vip": false,
      "vip_level": 1
    },
    "paper_trading": {
      "paper_balance_usd": 50000.0,
      "paper_margin_buffer": 0.05,
      "paper_fee_simulation": true
    },
    "tp_levels": {
      "default_tp_levels": [0.5, 1.0, 2.0, 3.0],
      "position_trailing_pct": 2.5,
      "swing_trailing_pct": 1.5,
      "scalp_trailing_pct": 0.8
    },
    "technical_thresholds": {
      "adx_threshold": 20.0,
      "max_funding_rate": 0.0015,
      "high_funding_multiplier": 2.0,
      "volatility_low": 0.5,
      "volatility_high": 3.0
    },
    "position_sizing": {
      "min_recommended_usd": 150.0,
      "optimal_min_usd": 300.0,
      "breakeven_buffer_pct": 0.15
    }
  }
}
```

---

## Testing Requirements

### Test 1: Load Defaults for New User
```bash
curl http://localhost:8094/api/futures/settings/trading-params \
  -H "Authorization: Bearer $TOKEN" | jq '.source'
# Expected: "defaults"
```

### Test 2: Update and Verify Persistence
```bash
curl -X PUT http://localhost:8094/api/futures/settings/trading-params \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"fees": {"taker_fee_percent": 0.04, "vip_level": 1}}'

# Verify saved
curl http://localhost:8094/api/futures/settings/trading-params \
  -H "Authorization: Bearer $TOKEN" | jq '.params.fees.taker_fee_percent'
# Expected: 0.04
```

### Test 3: Validation Errors
```bash
curl -X PUT http://localhost:8094/api/futures/settings/trading-params \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"order_execution": {"max_order_retries": 100}}'
# Expected: 400 "max_order_retries must be 1-10"
```

### Test 4: Reset to Defaults
```bash
curl -X POST http://localhost:8094/api/futures/settings/trading-params/reset \
  -H "Authorization: Bearer $TOKEN"

curl http://localhost:8094/api/futures/settings/trading-params \
  -H "Authorization: Bearer $TOKEN" | jq '.source'
# Expected: "defaults"
```

### Test 5: Trading Uses New Values
```bash
# Set ADX threshold to 25
curl -X PUT http://localhost:8094/api/futures/settings/trading-params \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"technical_thresholds": {"adx_threshold": 25.0}}'

# Restart container
./scripts/docker-dev.sh

# Check logs show new ADX threshold being used
docker-compose logs trading-bot | grep "ADX threshold"
# Expected: Logs showing 25.0 being used
```

---

## Definition of Done

- [ ] `TradingParameters` struct defined with 7 config groups (28 fields)
- [ ] `DefaultTradingParameters()` returns safe defaults
- [ ] Database loading with fallback implemented
- [ ] All 20+ hardcoded values replaced with config reads
- [ ] GET endpoint returns current params (DB or defaults)
- [ ] PUT endpoint updates with validation
- [ ] POST /reset endpoint restores defaults
- [ ] Per-user isolation verified
- [ ] Changes take effect immediately (no restart)
- [ ] Cache invalidation works on update
- [ ] TypeScript types added to frontend
- [ ] All 5 tests pass
- [ ] Code review approved

---

## Related Stories

- **Story 5.2:** Scalp Reentry Database Wiring (pattern to follow)
- **Story 6.2:** User Settings Cache Write-Through (cache integration)
- **Story 9.2:** Trading Parameters UI Panel (next - frontend for this)
- **Story 9.3:** VIP Tier Auto-Detection (stretch goal implementation)

---

## Approval Sign-Off

- **Scrum Master (Bob)**: Pending
- **Developer (Amelia)**: Pending
- **Test Architect (Murat)**: Pending
- **Architect (Winston)**: Pending
- **Product Manager (John)**: Pending
