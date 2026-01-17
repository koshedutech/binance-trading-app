# Epic 7: Client Order ID & Trade Lifecycle Tracking

## Epic Overview

**Goal:** Implement structured Client Order ID system that encodes trading mode, date, sequence, and order type into every Binance order, enabling complete trade lifecycle traceability from signal to exit across all displays (orders, positions, trade history).

**Business Value:** Full order traceability, mode-based analytics capability, simplified debugging, and foundation for Epic 8 (Daily Settlement & Mode Analytics).

**Priority:** HIGH - Core traceability system

**Estimated Complexity:** MEDIUM

**Depends On:** Epic 6 (Redis - for sequence storage)

---

## PREREQUISITE: Epic 6 Dependency Gate

**CRITICAL:** Epic 6 Stories 6.1-6.3 MUST be COMPLETED before starting Epic 7.

**Required Epic 6 Components:**
- [ ] Story 6.1: Redis container running (`binance-bot-redis`)
- [ ] Story 6.2: `CacheService` implemented with `IncrementDailySequence` method
- [ ] Story 6.3: Redis integrated with `main.go` and injected into services

**Verification Commands:**
```bash
# Check Redis container is running
docker ps | grep binance-bot-redis

# Test Redis connection
docker exec binance-bot-redis redis-cli PING
# Expected: PONG

# Test sequence increment
docker exec binance-bot-redis redis-cli INCR user:test:sequence:20260106
# Expected: (integer) 1
```

**Why This Gate Exists:**
- Story 7.1 (ID Generation) requires `CacheService.IncrementDailySequence()`
- Story 7.2 (Sequence Storage) requires Redis infrastructure
- Without Epic 6, Story 7.1 cannot generate valid sequence numbers

**Do NOT proceed with Epic 7 implementation until all three Epic 6 stories are verified complete.**

---

## Problem Statement

### Current Issues

| Issue | Severity | Impact |
|-------|----------|--------|
| **No structured order identification** | HIGH | Cannot trace orders through lifecycle |
| **Mode not tracked in orders** | HIGH | Cannot analyze performance by mode |
| **Related orders not linked** | HIGH | Entry/SL/TP/Hedge orders appear unrelated |
| **Manual debugging required** | MEDIUM | Hours to trace order flow |
| **No centralized lifecycle view** | MEDIUM | Must check multiple places |

### Current State

```
CURRENT STATE:
┌─────────────────────────────────────────────────────────────────┐
│ ORDER PLACEMENT:                                                │
│                                                                 │
│  Ginie places order → clientOrderId = "" or auto-generated     │
│                                                                 │
│  Problems:                                                      │
│  - No mode information encoded                                  │
│  - No date tracking                                             │
│  - Entry/SL/TP orders not linked                                │
│  - Hedge orders appear as separate trades                       │
│  - Cannot aggregate by mode                                     │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Target State

### Structured Client Order ID System

```
TARGET STATE:
┌─────────────────────────────────────────────────────────────────┐
│ CLIENT ORDER ID FORMAT:                                         │
│                                                                 │
│  [MODE]-[DDMMM]-[NNNNN]-[TYPE]                                  │
│                                                                 │
│  Examples:                                                      │
│  ULT-06JAN-00001-E     Ultra mode, Entry order                  │
│  ULT-06JAN-00001-SL    Ultra mode, Stop Loss (same chain)       │
│  ULT-06JAN-00001-TP1   Ultra mode, Take Profit 1                │
│  ULT-06JAN-00001-H     Ultra mode, Hedge entry                  │
│  SCA-06JAN-00042-E     Scalp mode, 42nd trade of day            │
│                                                                 │
│  Benefits:                                                      │
│  - Mode visible in every order                                  │
│  - Date tracking built-in                                       │
│  - Related orders share same base ID (chain)                    │
│  - Binance stores this - retrieve anytime                       │
│  - Parse and display in all UI locations                        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Trade Lifecycle Tab

```
┌──────────────────────────────────────────────────────────────────┐
│  Orders | Order History | Trade Log | Trade Lifecycle | Positions│
│  ─────────────────────────────────────────────────────────────── │
│                                                                  │
│  Filter: [All Modes ▼] [All Dates ▼] [Search Chain ID...]        │
│                                                                  │
│  Chain ID: ULT-06JAN-00001                                       │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ Mode: ULTRA | Symbol: BTCUSDT | Direction: LONG            │  │
│  │ Start: 06-Jan 09:15:32 | End: 06-Jan 14:22:18 | Duration: 5h│  │
│  ├────────────────────────────────────────────────────────────┤  │
│  │ Stage          │ Time     │ Price    │ Status   │ Details  │  │
│  │ ─────────────────────────────────────────────────────────  │  │
│  │ 🔔 Signal      │ 09:15:30 │ 97,450   │ ✅       │ Conf: 85%│  │
│  │ 📥 Entry (E)   │ 09:15:32 │ 97,455   │ ✅ Filled│ Slip: +5 │  │
│  │ 🛡️ SL Placed   │ 09:15:33 │ 96,500   │ ✅ Active│ -1%      │  │
│  │ 🎯 TP1 Placed  │ 09:15:33 │ 98,000   │ ✅ Hit   │ +0.5%    │  │
│  │ 🎯 TP2 Placed  │ 09:15:33 │ 98,500   │ ⏳ Active│ +1%      │  │
│  │ 🔄 Hedge (H)   │ 11:30:45 │ 97,200   │ ✅ Filled│ SHORT    │  │
│  │ 📤 Exit        │ 14:22:18 │ 98,200   │ ✅ Closed│ TP2 hit  │  │
│  ├────────────────────────────────────────────────────────────┤  │
│  │ SUMMARY: P&L: +$245.00 (+2.1%) | Fees: $12.50              │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                  │
│  Chain ID: SCA-06JAN-00042                                       │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ Mode: SCALP | Symbol: ETHUSDT | Direction: SHORT           │  │
│  │ ... (collapsed, click to expand)                           │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

---

## Client Order ID Specification

### Format Definition

```
FORMAT: [MODE]-[DDMMM]-[NNNNN]-[TYPE]

COMPONENTS:
┌─────────────────────────────────────────────────────────────────┐
│ [MODE] - 3 characters, uppercase                                │
│   ULT = Ultra Fast mode                                         │
│   SCA = Scalp mode                                              │
│   SCR = Scalp Reentry mode (NEW)                                │
│   SWI = Swing mode                                              │
│   POS = Position mode                                           │
├─────────────────────────────────────────────────────────────────┤
│ [DDMMM] - 5 characters, date in user's timezone                 │
│   06JAN = January 6th                                           │
│   25DEC = December 25th                                         │
│   01FEB = February 1st                                          │
├─────────────────────────────────────────────────────────────────┤
│ [NNNNN] - 5 digits, daily sequence number (00001-99999)         │
│   Resets to 00001 at user's timezone midnight                   │
│   Stored in Redis: user:{id}:sequence:{YYYYMMDD}                │
├─────────────────────────────────────────────────────────────────┤
│ [TYPE] - 1-3 characters, order type suffix                      │
│   E    = Entry order                                            │
│   SL   = Stop Loss                                              │
│   TP1  = Take Profit Level 1                                    │
│   TP2  = Take Profit Level 2                                    │
│   TP3  = Take Profit Level 3                                    │
│   TP4  = Take Profit Level 4                                    │
│   H    = Hedge Entry                                            │
│   HSL  = Hedge Stop Loss                                        │
│   HTP  = Hedge Take Profit                                      │
│   DCA1 = DCA Level 1                                            │
│   DCA2 = DCA Level 2                                            │
│   DCA3 = DCA Level 3                                            │
└─────────────────────────────────────────────────────────────────┘

EXAMPLES:
  ULT-06JAN-00001-E     = Ultra, Jan 6, trade #1, Entry
  ULT-06JAN-00001-SL    = Ultra, Jan 6, trade #1, Stop Loss
  ULT-06JAN-00001-TP1   = Ultra, Jan 6, trade #1, Take Profit 1
  ULT-06JAN-00001-H     = Ultra, Jan 6, trade #1, Hedge Entry
  SCA-06JAN-00042-E     = Scalp, Jan 6, trade #42, Entry
  SCR-06JAN-00015-E     = Scalp Reentry, Jan 6, trade #15, Entry
  SWI-06JAN-00003-TP2   = Swing, Jan 6, trade #3, Take Profit 2
  POS-07JAN-00001-DCA2  = Position, Jan 7, trade #1, DCA Level 2

CHARACTER COUNT:
  ULT-06JAN-00001-E   = 18 characters
  ULT-06JAN-00001-DCA3 = 20 characters
  Maximum: 20 characters (well under Binance 36-char limit)
```

### Order Chain Concept

```
ORDER CHAIN - All orders sharing same base ID belong to same trade

Chain: ULT-06JAN-00001
├── ULT-06JAN-00001-E    (Entry - LONG)
├── ULT-06JAN-00001-SL   (Stop Loss)
├── ULT-06JAN-00001-TP1  (Take Profit 1 - 25%)
├── ULT-06JAN-00001-TP2  (Take Profit 2 - 25%)
├── ULT-06JAN-00001-TP3  (Take Profit 3 - 25%)
├── ULT-06JAN-00001-TP4  (Take Profit 4 - 25%)
├── ULT-06JAN-00001-H    (Hedge Entry - SHORT)
├── ULT-06JAN-00001-HSL  (Hedge Stop Loss)
└── ULT-06JAN-00001-HTP  (Hedge Take Profit)

All 9 orders are part of ONE trade lifecycle.
```

---

## Requirements Traceability

### Functional Requirements

| ID | Requirement | Stories |
|----|-------------|---------|
| FR-1 | Generate structured clientOrderId for all orders | 7.1 |
| FR-2 | Daily sequence number with timezone-aware reset | 7.2 |
| FR-3 | Link related orders via chain ID (E/SL/TP/H) | 7.3 |
| FR-4 | Parse clientOrderId from Binance API responses | 7.4 |
| FR-5 | Display Trade Lifecycle tab with chain visualization | 7.5 |
| FR-6 | User timezone preference for date component | 7.6 |
| FR-7 | Support hedge order suffixes (-H, -HSL, -HTP) | 7.7 |
| FR-8 | Fallback ID generation when Redis unavailable | 7.8 |
| FR-9 | Backend API for Trade Lifecycle data retrieval | 7.9 |
| FR-10 | Support Scalp Reentry mode (SCR) | 7.1 |
| FR-11 | Track position state when entry order fills | 7.11 |
| FR-12 | Log all SL/TP modifications with LLM reasoning | 7.12 |
| FR-13 | Display modification history in tree structure UI | 7.13 |
| FR-14 | Backend integration: Merge position states with orders | 7.14 |
| FR-15 | Tree structure UI: Entry → Position → TP/SL hierarchy | 7.15 |

### Non-Functional Requirements

| ID | Requirement | Stories |
|----|-------------|---------|
| NFR-1 | clientOrderId ≤ 36 characters (Binance limit) | 7.1, 7.10 |
| NFR-2 | Sequence increment atomic (no duplicates) | 7.2, 7.10 |
| NFR-3 | Parsing handles malformed IDs gracefully | 7.4, 7.10 |
| NFR-4 | Lifecycle tab loads within 2 seconds | 7.5 |
| NFR-5 | Order placement continues if Redis fails | 7.8 |
| NFR-6 | Edge cases tested comprehensively | 7.10 |

---

## Stories

### Story 7.1: Client Order ID Generation

**Goal:** Create structured clientOrderId for every order placed.

**Acceptance Criteria:**
- [ ] `ClientOrderIdGenerator` service with `Generate(mode, orderType)` method
- [ ] Mode codes: ULT, SCA, SCR, SWI, POS (5 modes total)
- [ ] Date format: DDMMM (user's timezone)
- [ ] Sequence: 5 digits, zero-padded
- [ ] Type suffixes: E, SL, TP1-TP4, H, HSL, HTP, DCA1-DCA3
- [ ] Validation: Output ≤ 36 characters
- [ ] Integration with all order placement code paths

**Technical Notes:**
```go
// internal/orders/client_order_id.go
type ClientOrderIdGenerator struct {
    cache    *cache.CacheService
    timezone *time.Location
}

type OrderType string

const (
    OrderTypeEntry      OrderType = "E"
    OrderTypeStopLoss   OrderType = "SL"
    OrderTypeTakeProfit1 OrderType = "TP1"
    OrderTypeTakeProfit2 OrderType = "TP2"
    OrderTypeTakeProfit3 OrderType = "TP3"
    OrderTypeTakeProfit4 OrderType = "TP4"
    OrderTypeHedge      OrderType = "H"
    OrderTypeHedgeSL    OrderType = "HSL"
    OrderTypeHedgeTP    OrderType = "HTP"
    OrderTypeDCA1       OrderType = "DCA1"
    OrderTypeDCA2       OrderType = "DCA2"
    OrderTypeDCA3       OrderType = "DCA3"
)

func (g *ClientOrderIdGenerator) Generate(userID string, mode TradingMode, orderType OrderType) (string, error) {
    // Get date in user's timezone
    now := time.Now().In(g.timezone)
    dateStr := strings.ToUpper(now.Format("02Jan")) // "06JAN"

    // Get/increment sequence from Redis
    seq, err := g.cache.IncrementDailySequence(userID, now)
    if err != nil {
        return "", err
    }

    // Format: ULT-06JAN-00001-E
    return fmt.Sprintf("%s-%s-%05d-%s", mode.Code(), dateStr, seq, orderType), nil
}

// For related orders in same chain, reuse base ID
func (g *ClientOrderIdGenerator) GenerateRelated(baseID string, orderType OrderType) string {
    // baseID = "ULT-06JAN-00001"
    return fmt.Sprintf("%s-%s", baseID, orderType)
}
```

---

### Story 7.2: Daily Sequence Storage in Redis

**Goal:** Atomic sequence counter in Redis with timezone-aware daily reset.

**Acceptance Criteria:**
- [ ] Redis key: `user:{user_id}:sequence:{YYYYMMDD}`
- [ ] Atomic INCR operation (no duplicates under load)
- [ ] Key TTL: 48 hours (auto-cleanup)
- [ ] Reset to 1 at user's timezone midnight
- [ ] Handle sequence rollover (99999 → 00001 with date change)
- [ ] Graceful fallback if Redis unavailable (see Story 7.8)

**Technical Notes:**
```go
// internal/cache/sequence.go
func (c *CacheService) IncrementDailySequence(userID string, now time.Time) (int64, error) {
    dateKey := now.Format("20060102") // "20260106"
    key := fmt.Sprintf("user:%s:sequence:%s", userID, dateKey)

    // Atomic increment
    seq, err := c.redis.Incr(ctx, key).Result()
    if err != nil {
        return 0, err
    }

    // Set TTL on first use (48 hours)
    if seq == 1 {
        c.redis.Expire(ctx, key, 48*time.Hour)
    }

    return seq, nil
}

func (c *CacheService) GetCurrentSequence(userID string, now time.Time) (int64, error) {
    dateKey := now.Format("20060102")
    key := fmt.Sprintf("user:%s:sequence:%s", userID, dateKey)
    return c.redis.Get(ctx, key).Int64()
}
```

---

### Story 7.3: Order Chain Tracking

**Goal:** Link all related orders (entry, SL, TP, hedge) via chain ID.

**Acceptance Criteria:**
- [ ] Entry order generates new chain ID (base + -E)
- [ ] SL/TP orders use same base ID with different suffix
- [ ] Hedge orders use same base ID with -H/-HSL/-HTP
- [ ] DCA orders use same base ID with -DCA1/-DCA2/-DCA3
- [ ] Chain ID passed through order placement flow
- [ ] Store chain ID with position/trade data

**Order Flow:**
```
1. Signal received for BTCUSDT LONG
2. Generate chain base: ULT-06JAN-00001
3. Place Entry: ULT-06JAN-00001-E
4. Place SL:    ULT-06JAN-00001-SL  (same base)
5. Place TP1:   ULT-06JAN-00001-TP1 (same base)
6. Place TP2:   ULT-06JAN-00001-TP2 (same base)
7. [If hedge triggered]
   Place Hedge: ULT-06JAN-00001-H   (same base)
   Place HSL:   ULT-06JAN-00001-HSL
   Place HTP:   ULT-06JAN-00001-HTP
```

---

### Story 7.4: Parse Client Order ID from Binance Responses

**Goal:** Extract mode, date, sequence, and type from returned clientOrderId.

**Acceptance Criteria:**
- [ ] `ClientOrderIdParser` with `Parse(clientOrderId)` method
- [ ] Returns: mode, date, sequence, orderType, chainId
- [ ] Handles malformed IDs gracefully (returns nil, no error)
- [ ] Recognizes legacy/unstructured IDs (skip parsing)
- [ ] Integration with order/trade response processing

**Technical Notes:**
```go
// internal/orders/client_order_id_parser.go
type ParsedOrderId struct {
    Mode      TradingMode
    Date      time.Time
    Sequence  int
    OrderType OrderType
    ChainId   string // "ULT-06JAN-00001" (without type suffix)
    Raw       string // Original full ID
}

func ParseClientOrderId(clientOrderId string) *ParsedOrderId {
    // Pattern: MODE-DDMMM-NNNNN-TYPE
    // Example: ULT-06JAN-00001-E

    parts := strings.Split(clientOrderId, "-")
    if len(parts) < 4 {
        return nil // Not our format
    }

    mode := parseTradingMode(parts[0])
    if mode == "" {
        return nil
    }

    date, err := time.Parse("02Jan", parts[1])
    if err != nil {
        return nil
    }

    seq, err := strconv.Atoi(parts[2])
    if err != nil {
        return nil
    }

    orderType := OrderType(parts[3])

    return &ParsedOrderId{
        Mode:      mode,
        Date:      date,
        Sequence:  seq,
        OrderType: orderType,
        ChainId:   strings.Join(parts[:3], "-"), // "ULT-06JAN-00001"
        Raw:       clientOrderId,
    }
}
```

---

### Story 7.5: Trade Lifecycle Tab UI

**Goal:** New tab displaying complete trade journeys with chain visualization.

**Acceptance Criteria:**
- [ ] New tab: "Trade Lifecycle" alongside Orders, Order History, Trade Log, Positions
- [ ] Group orders by chain ID
- [ ] Collapsible chain cards showing all stages
- [ ] Timeline visualization within each chain
- [ ] Filters: Mode, Date range, Symbol, Status
- [ ] Search by chain ID
- [ ] Summary per chain: Total P&L, Duration, Fees
- [ ] Color coding: Entry (blue), SL (red), TP (green), Hedge (yellow)

**UI Components:**
```
web/src/components/
├── TradeLifecycle/
│   ├── TradeLifecycleTab.tsx      # Main tab component
│   ├── ChainCard.tsx              # Individual chain display
│   ├── ChainTimeline.tsx          # Timeline within chain
│   ├── ChainFilters.tsx           # Filter controls
│   ├── ChainSearch.tsx            # Search by chain ID
│   └── ChainSummary.tsx           # P&L summary per chain
```

**Data Flow:**
```
1. Fetch orders from Binance API (or cache)
2. Parse clientOrderId for each order
3. Group orders by chainId
4. Sort chains by most recent activity
5. Render collapsible chain cards
6. Calculate P&L per chain from fills
```

---

### Story 7.6: User Timezone Settings

**Goal:** Allow users to configure timezone for date component in clientOrderId.

**Acceptance Criteria:**
- [ ] User settings: timezone preference
- [ ] Default: Asia/Kolkata (GMT+5:30) - from Docker container
- [ ] Preset options: India (IST), Cambodia (ICT)
- [ ] Custom option: Full IANA timezone list
- [ ] Timezone used for:
  - Date component in clientOrderId (DDMMM)
  - Sequence reset timing (midnight)
  - Trade Lifecycle display timestamps
- [ ] Docker container TZ variable as fallback

**Database Schema:**
```sql
ALTER TABLE users ADD COLUMN timezone VARCHAR(50) DEFAULT 'Asia/Kolkata';

CREATE TABLE timezone_presets (
    id SERIAL PRIMARY KEY,
    display_name VARCHAR(100),      -- "India Standard Time (IST)"
    tz_identifier VARCHAR(50),      -- "Asia/Kolkata"
    gmt_offset VARCHAR(10),         -- "+05:30"
    is_default BOOLEAN DEFAULT false
);

INSERT INTO timezone_presets (display_name, tz_identifier, gmt_offset, is_default) VALUES
    ('India Standard Time (IST)', 'Asia/Kolkata', '+05:30', true),
    ('Indochina Time (ICT)', 'Asia/Phnom_Penh', '+07:00', false);
```

---

### Story 7.7: Hedge Order Suffixes

**Goal:** Support hedge order identification in clientOrderId.

**Acceptance Criteria:**
- [ ] Suffix -H for hedge entry order
- [ ] Suffix -HSL for hedge stop loss
- [ ] Suffix -HTP for hedge take profit
- [ ] Hedge orders share same chain base as original trade
- [ ] Trade Lifecycle shows hedge as distinct stage with yellow color
- [ ] P&L calculation includes hedge P&L in chain total

**Example Chain with Hedge:**
```
Chain: ULT-06JAN-00001
├── ULT-06JAN-00001-E    → LONG Entry @ 97,450
├── ULT-06JAN-00001-SL   → Stop Loss @ 96,500
├── ULT-06JAN-00001-TP1  → TP1 @ 98,000 (HIT)
├── ULT-06JAN-00001-H    → HEDGE SHORT Entry @ 97,200
├── ULT-06JAN-00001-HSL  → Hedge SL @ 97,800
├── ULT-06JAN-00001-HTP  → Hedge TP @ 96,500 (HIT)
└── Final Exit           → Position closed

Timeline:
09:15 → Entry LONG
10:30 → TP1 Hit (+$100)
11:30 → Hedge SHORT triggered (price reversing)
12:45 → Hedge TP Hit (+$80)
13:00 → Remaining position closed (+$65)
TOTAL P&L: +$245
```

---

### Story 7.8: Redis Fallback for Sequence Generation

**Goal:** Ensure order placement continues even if Redis is unavailable.

**Acceptance Criteria:**
- [ ] If `IncrementDailySequence()` returns error, generate fallback ID
- [ ] Fallback format: `{MODE}-FALLBACK-{8-char-uuid}`
- [ ] Example: `ULT-FALLBACK-a3f7c2e9-E`
- [ ] Log WARNING when fallback is used
- [ ] Order placement continues without blocking
- [ ] Fallback IDs still parseable (ChainId = base without type)
- [ ] Trade Lifecycle tab displays fallback chains with warning icon
- [ ] Health check endpoint reports Redis status

**Technical Notes:**
```go
// internal/orders/client_order_id.go
func (g *ClientOrderIdGenerator) Generate(userID string, mode TradingMode, orderType OrderType) (string, error) {
    // Get date in user's timezone
    now := time.Now().In(g.timezone)
    dateStr := strings.ToUpper(now.Format("02Jan")) // "06JAN"

    // Try to get sequence from Redis
    seq, err := g.cache.IncrementDailySequence(userID, now)
    if err != nil {
        // Redis unavailable - use fallback
        log.Warn().Err(err).Msg("Redis unavailable, using fallback clientOrderId")

        // Generate UUID-based fallback
        uuid := generateShortUUID() // First 8 chars of UUID
        fallbackID := fmt.Sprintf("%s-FALLBACK-%s-%s", mode.Code(), uuid, orderType)

        // Still return valid ID, order placement continues
        return fallbackID, nil
    }

    // Format: ULT-06JAN-00001-E
    return fmt.Sprintf("%s-%s-%05d-%s", mode.Code(), dateStr, seq, orderType), nil
}

func generateShortUUID() string {
    uuid := uuid.New().String()
    return strings.ReplaceAll(uuid[:8], "-", "") // "a3f7c2e9"
}
```

**Fallback Chain Example:**
```
Chain: ULT-FALLBACK-a3f7c2e9
├── ULT-FALLBACK-a3f7c2e9-E    (Entry)
├── ULT-FALLBACK-a3f7c2e9-SL   (Stop Loss)
└── ULT-FALLBACK-a3f7c2e9-TP1  (Take Profit)

Note: Fallback IDs still allow chain grouping and lifecycle tracking.
```

**Why This Matters:**
- Order placement must never fail due to Redis being down
- Traders don't lose opportunities due to infrastructure issues
- Degraded but functional service > complete failure
- Fallback IDs still provide traceability

---

### Story 7.9: Backend API for Trade Lifecycle Tab

**Goal:** Provide REST API endpoints for Trade Lifecycle tab data retrieval.

**Acceptance Criteria:**
- [ ] GET `/api/futures/trade-lifecycle` - List all trade chains
- [ ] GET `/api/futures/trade-lifecycle/:chainId` - Get single chain details
- [ ] Query parameters for list endpoint:
  - `mode` - Filter by trading mode (ULT, SCA, SCR, SWI, POS)
  - `startDate` - Filter trades after this date (ISO 8601)
  - `endDate` - Filter trades before this date (ISO 8601)
  - `symbol` - Filter by trading symbol (BTCUSDT, etc.)
  - `status` - Filter by status (active, closed, partial)
  - `limit` - Pagination limit (default 50, max 200)
  - `offset` - Pagination offset
- [ ] Response includes all orders in chain with parsed metadata
- [ ] Response includes P&L calculation per chain
- [ ] Response includes chain duration and summary stats
- [ ] Efficient query: Index on clientOrderId base pattern
- [ ] Cache frequently accessed chains (Redis)

**API Specification:**
```typescript
// GET /api/futures/trade-lifecycle
// Query: ?mode=ULT&startDate=2026-01-01&endDate=2026-01-07&limit=50&offset=0

interface TradeLifecycleListResponse {
  chains: TradeChainSummary[];
  total: number;
  limit: number;
  offset: number;
}

interface TradeChainSummary {
  chainId: string;           // "ULT-06JAN-00001"
  mode: string;              // "ULT"
  symbol: string;            // "BTCUSDT"
  direction: "LONG" | "SHORT";
  status: "active" | "closed" | "partial";
  startTime: string;         // ISO 8601
  endTime: string | null;    // ISO 8601 or null if active
  duration: number | null;   // Seconds or null if active
  pnl: number;               // Total P&L in USDT
  pnlPercent: number;        // Total P&L percentage
  fees: number;              // Total fees paid
  orderCount: number;        // Number of orders in chain
  isFallback: boolean;       // True if FALLBACK ID
}

// GET /api/futures/trade-lifecycle/:chainId
// Example: GET /api/futures/trade-lifecycle/ULT-06JAN-00001

interface TradeChainDetailResponse {
  chain: TradeChainSummary;
  orders: TradeChainOrder[];
  timeline: TradeChainEvent[];
}

interface TradeChainOrder {
  orderId: string;
  clientOrderId: string;     // "ULT-06JAN-00001-E"
  orderType: string;         // "E", "SL", "TP1", "H", etc.
  symbol: string;
  side: "BUY" | "SELL";
  type: string;              // "MARKET", "LIMIT", "STOP_MARKET"
  status: string;            // "NEW", "FILLED", "CANCELED"
  price: number;
  quantity: number;
  executedQty: number;
  createdAt: string;         // ISO 8601
  updatedAt: string;         // ISO 8601
  fills: OrderFill[];
}

interface OrderFill {
  price: number;
  quantity: number;
  commission: number;
  commissionAsset: string;
  time: string;              // ISO 8601
}

interface TradeChainEvent {
  time: string;              // ISO 8601
  stage: string;             // "SIGNAL", "ENTRY", "SL_PLACED", "TP_HIT", "HEDGE", "EXIT"
  description: string;
  price: number | null;
  status: "success" | "pending" | "failed";
  details: Record<string, any>;
}
```

**Backend Implementation:**
```go
// internal/api/futures_lifecycle_handlers.go

func (h *FuturesLifecycleHandler) ListTradeChains(w http.ResponseWriter, r *http.Request) {
    // Parse query params
    mode := r.URL.Query().Get("mode")
    startDate := r.URL.Query().Get("startDate")
    endDate := r.URL.Query().Get("endDate")
    symbol := r.URL.Query().Get("symbol")
    status := r.URL.Query().Get("status")
    limit := parseIntParam(r.URL.Query().Get("limit"), 50, 200)
    offset := parseIntParam(r.URL.Query().Get("offset"), 0, 0)

    // Query orders from Binance API or database
    orders, err := h.binanceClient.GetAllOrders(ctx, symbol, startDate, endDate)

    // Group by chainId
    chains := groupOrdersByChain(orders)

    // Filter by mode, status
    filtered := filterChains(chains, mode, status)

    // Calculate summaries
    summaries := calculateChainSummaries(filtered)

    // Apply pagination
    paginated := paginateResults(summaries, limit, offset)

    // Return response
    json.NewEncoder(w).Write(TradeLifecycleListResponse{
        Chains: paginated,
        Total: len(summaries),
        Limit: limit,
        Offset: offset,
    })
}

func (h *FuturesLifecycleHandler) GetTradeChainDetail(w http.ResponseWriter, r *http.Request) {
    chainId := chi.URLParam(r, "chainId")

    // Query all orders with this chainId base
    orders, err := h.repository.GetOrdersByChainId(ctx, chainId)

    // Build timeline events
    timeline := buildTimeline(orders)

    // Calculate summary
    summary := calculateChainSummary(orders)

    // Return response
    json.NewEncoder(w).Write(TradeChainDetailResponse{
        Chain: summary,
        Orders: orders,
        Timeline: timeline,
    })
}
```

**Database Query Optimization:**
```sql
-- Index for efficient chain lookup
CREATE INDEX idx_orders_client_order_id_prefix ON orders
  USING btree (substring(client_order_id, 1, 17));
  -- "ULT-06JAN-00001" = 17 characters (base without type)

-- Query all orders in a chain
SELECT * FROM orders
WHERE substring(client_order_id, 1, 17) = 'ULT-06JAN-00001'
ORDER BY created_at ASC;
```

---

### Story 7.10: Edge Case Test Suite

**Goal:** Comprehensive test coverage for edge cases in clientOrderId system.

**Acceptance Criteria:**
- [ ] **Midnight Rollover Test**: Verify sequence resets at user's timezone midnight
- [ ] **Redis Failure Handling**: Test fallback ID generation when Redis is down
- [ ] **Binance Acceptance Test**: Verify Binance accepts all ID formats (normal + fallback)
- [ ] **Malformed ID Parsing**: Test parser handles invalid/legacy IDs gracefully
- [ ] **Year Boundary Test**: Verify date format works across Dec 31 → Jan 1
- [ ] **Concurrent Sequence Test**: Simulate concurrent requests, verify no duplicates
- [ ] **Maximum Sequence Test**: Verify behavior at sequence 99999
- [ ] **Fallback Chain Grouping**: Verify fallback IDs still group correctly in UI
- [ ] **Mode Code Validation**: Test all 5 modes (ULT, SCA, SCR, SWI, POS)
- [ ] **All Order Types**: Test all type suffixes (E, SL, TP1-TP4, H, HSL, HTP, DCA1-DCA3)

**Test Implementation:**
```go
// internal/orders/client_order_id_test.go

func TestMidnightRollover(t *testing.T) {
    // Test that sequence resets at midnight in user's timezone
    // Use Asia/Kolkata timezone (GMT+5:30)
    loc, _ := time.LoadLocation("Asia/Kolkata")

    // 11:59 PM on Jan 6
    time1 := time.Date(2026, 1, 6, 23, 59, 0, 0, loc)
    id1, _ := generator.GenerateAtTime(userID, ModeUltraFast, OrderTypeEntry, time1)
    // Should be: ULT-06JAN-00001-E

    // 12:01 AM on Jan 7
    time2 := time.Date(2026, 1, 7, 0, 1, 0, 0, loc)
    id2, _ := generator.GenerateAtTime(userID, ModeScalp, OrderTypeEntry, time2)
    // Should be: SCA-07JAN-00001-E (sequence reset to 1)

    assert.Contains(t, id1, "06JAN-00001")
    assert.Contains(t, id2, "07JAN-00001")
}

func TestRedisFallback(t *testing.T) {
    // Simulate Redis down
    generator.cache.Close()

    id, err := generator.Generate(userID, ModeUltraFast, OrderTypeEntry)

    // Should return fallback ID, not error
    assert.NoError(t, err)
    assert.Contains(t, id, "FALLBACK")
    assert.Regexp(t, `ULT-FALLBACK-[a-f0-9]{8}-E`, id)
}

func TestBinanceAcceptance(t *testing.T) {
    // Test that Binance API accepts our ID formats
    testCases := []struct {
        name string
        id   string
    }{
        {"Normal ID", "ULT-06JAN-00001-E"},
        {"Fallback ID", "ULT-FALLBACK-a3f7c2e9-E"},
        {"Long Type", "POS-06JAN-00042-DCA3"},
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            order, err := client.NewCreateOrderService().
                Symbol("BTCUSDT").
                Side(binance.SideTypeBuy).
                Type(binance.OrderTypeMarket).
                Quantity("0.001").
                NewClientOrderID(tc.id).
                Do(ctx)

            assert.NoError(t, err)
            assert.Equal(t, tc.id, order.ClientOrderID)
        })
    }
}

func TestMalformedParsing(t *testing.T) {
    testCases := []struct {
        name string
        id   string
        want *ParsedOrderId
    }{
        {"Valid ID", "ULT-06JAN-00001-E", &ParsedOrderId{Mode: ModeUltraFast, ...}},
        {"Legacy ID", "myorder123", nil},
        {"Empty", "", nil},
        {"Too few parts", "ULT-06JAN", nil},
        {"Invalid mode", "XXX-06JAN-00001-E", nil},
        {"Invalid date", "ULT-99ABC-00001-E", nil},
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            parsed := ParseClientOrderId(tc.id)
            if tc.want == nil {
                assert.Nil(t, parsed)
            } else {
                assert.NotNil(t, parsed)
                assert.Equal(t, tc.want.Mode, parsed.Mode)
            }
        })
    }
}

func TestYearBoundary(t *testing.T) {
    loc, _ := time.LoadLocation("Asia/Kolkata")

    // Dec 31, 2026
    time1 := time.Date(2026, 12, 31, 23, 59, 0, 0, loc)
    id1, _ := generator.GenerateAtTime(userID, ModeSwing, OrderTypeEntry, time1)
    assert.Contains(t, id1, "31DEC")

    // Jan 1, 2027
    time2 := time.Date(2027, 1, 1, 0, 1, 0, 0, loc)
    id2, _ := generator.GenerateAtTime(userID, ModeSwing, OrderTypeEntry, time2)
    assert.Contains(t, id2, "01JAN")
}

func TestConcurrentSequence(t *testing.T) {
    // Simulate 100 concurrent requests
    const goroutines = 100
    ids := make(chan string, goroutines)

    for i := 0; i < goroutines; i++ {
        go func() {
            id, _ := generator.Generate(userID, ModeScalp, OrderTypeEntry)
            ids <- id
        }()
    }

    // Collect all IDs
    uniqueIds := make(map[string]bool)
    for i := 0; i < goroutines; i++ {
        id := <-ids
        uniqueIds[id] = true
    }

    // All IDs must be unique (no duplicate sequences)
    assert.Equal(t, goroutines, len(uniqueIds))
}

func TestMaxSequence(t *testing.T) {
    // Set sequence to 99999
    cache.SetDailySequence(userID, time.Now(), 99999)

    // Next increment should reset to 1 with date change warning
    id, err := generator.Generate(userID, ModePosition, OrderTypeEntry)

    // Should either wrap to 1 or generate fallback
    assert.NoError(t, err)
    // Implementation decision: Either allow 6-digit sequence or use fallback
}

func TestFallbackChainGrouping(t *testing.T) {
    baseID := "ULT-FALLBACK-a3f7c2e9"

    entryID := fmt.Sprintf("%s-E", baseID)
    slID := fmt.Sprintf("%s-SL", baseID)
    tpID := fmt.Sprintf("%s-TP1", baseID)

    parsedEntry := ParseClientOrderId(entryID)
    parsedSL := ParseClientOrderId(slID)
    parsedTP := ParseClientOrderId(tpID)

    // All should have same chainId
    assert.Equal(t, baseID, parsedEntry.ChainId)
    assert.Equal(t, baseID, parsedSL.ChainId)
    assert.Equal(t, baseID, parsedTP.ChainId)
}

func TestAllModes(t *testing.T) {
    modes := []TradingMode{
        ModeUltraFast,
        ModeScalp,
        ModeScalpReentry,
        ModeSwing,
        ModePosition,
    }

    for _, mode := range modes {
        id, err := generator.Generate(userID, mode, OrderTypeEntry)
        assert.NoError(t, err)
        assert.Contains(t, id, mode.Code())
    }
}

func TestAllOrderTypes(t *testing.T) {
    types := []OrderType{
        OrderTypeEntry, OrderTypeStopLoss,
        OrderTypeTakeProfit1, OrderTypeTakeProfit2, OrderTypeTakeProfit3, OrderTypeTakeProfit4,
        OrderTypeHedge, OrderTypeHedgeSL, OrderTypeHedgeTP,
        OrderTypeDCA1, OrderTypeDCA2, OrderTypeDCA3,
    }

    for _, orderType := range types {
        id, err := generator.Generate(userID, ModeUltraFast, orderType)
        assert.NoError(t, err)
        assert.Contains(t, id, string(orderType))
        assert.LessOrEqual(t, len(id), 36) // Binance limit
    }
}
```

**Integration Test:**
```go
// integration_test.go
func TestFullLifecycle(t *testing.T) {
    // End-to-end test: Generate → Place → Retrieve → Parse → Display

    // 1. Generate ID
    chainBase := generator.Generate(userID, ModeUltraFast, OrderTypeEntry)
    entryID := fmt.Sprintf("%s-E", chainBase)

    // 2. Place order on Binance
    order, err := client.PlaceOrder(ctx, PlaceOrderRequest{
        Symbol: "BTCUSDT",
        Side: "BUY",
        Type: "MARKET",
        Quantity: 0.001,
        ClientOrderID: entryID,
    })
    assert.NoError(t, err)

    // 3. Retrieve order from Binance
    retrieved, err := client.GetOrder(ctx, "BTCUSDT", order.OrderID)
    assert.NoError(t, err)
    assert.Equal(t, entryID, retrieved.ClientOrderID)

    // 4. Parse retrieved ID
    parsed := ParseClientOrderId(retrieved.ClientOrderID)
    assert.NotNil(t, parsed)
    assert.Equal(t, ModeUltraFast, parsed.Mode)
    assert.Equal(t, OrderTypeEntry, parsed.OrderType)

    // 5. Group in UI (simulated)
    chains := groupOrdersByChain([]*Order{retrieved})
    assert.Equal(t, 1, len(chains))
    assert.Equal(t, chainBase, chains[0].ChainId)
}
```

---

### Story 7.11: Position State Tracking

**Goal:** Track the transition from Entry Order to Active Position as an explicit lifecycle stage, ensuring the entry order remains visible in the chain even after it fills.

**Problem Statement:**
Currently, when an entry order fills:
- Binance removes it from `GetOpenOrders()` API
- The order "disappears" from the Order Chain UI
- Users see only SL/TP orders without context of the original entry
- There's no explicit "Position Active" state in the lifecycle

**Acceptance Criteria:**
- [x] Detect when entry order status changes from NEW/PARTIALLY_FILLED to FILLED
- [x] Create `position_states` record linking to chain ID
- [x] Store entry fill details (price, quantity, timestamp, fees)
- [ ] Display "Position Active" as explicit stage in Trade Lifecycle timeline
- [ ] Preserve entry order in chain display even after it fills
- [x] Track position status transitions: ACTIVE → PARTIAL → CLOSED
- [ ] Calculate and display unrealized P&L for active positions
- [x] Handle partial fills (entry partially filled, position partially active)

**Implementation Status: CORE BACKEND COMPLETE (2026-01-17)**
- Database migration: `migrations/034_position_states.sql`
- Position Tracker Service: `internal/orders/position_tracker.go`
- Database Repository: `internal/database/repository_position_states.go`
- Ginie Autopilot Integration: `internal/autopilot/position_state_integration.go`
- API Endpoints: `internal/api/handlers_trade_lifecycle.go`
  - GET `/api/futures/position-states` - List position states by status
  - GET `/api/futures/position-states/recent` - Recent position states
  - GET `/api/futures/position-states/:chainId` - Get by chain ID
  - GET `/api/futures/position-states/symbol/:symbol` - Get by symbol

**Remaining Work:**
- Frontend UI components for Trade Lifecycle timeline display
- Unrealized P&L calculation and display

**Database Schema:**
```sql
-- Position state tracking table
CREATE TABLE position_states (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) NOT NULL,
    chain_id VARCHAR(30) NOT NULL,           -- "ULT-17JAN-00001"
    symbol VARCHAR(20) NOT NULL,             -- "BTCUSDT"

    -- Entry order reference
    entry_order_id BIGINT NOT NULL,          -- Binance order ID
    entry_client_order_id VARCHAR(40),       -- "ULT-17JAN-00001-E"

    -- Position entry details
    entry_side VARCHAR(10) NOT NULL,         -- "BUY" (LONG) or "SELL" (SHORT)
    entry_price DECIMAL(18, 8) NOT NULL,     -- Avg fill price
    entry_quantity DECIMAL(18, 8) NOT NULL,  -- Total filled quantity
    entry_value DECIMAL(18, 2) NOT NULL,     -- entry_price * entry_quantity
    entry_fees DECIMAL(18, 8) DEFAULT 0,     -- Commission paid
    entry_filled_at TIMESTAMP WITH TIME ZONE NOT NULL,

    -- Current position state
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',  -- ACTIVE, PARTIAL, CLOSED
    remaining_quantity DECIMAL(18, 8) NOT NULL,
    realized_pnl DECIMAL(18, 2) DEFAULT 0,         -- P&L from partial closes

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    closed_at TIMESTAMP WITH TIME ZONE,

    -- Constraints
    CONSTRAINT unique_chain_position UNIQUE (user_id, chain_id)
);

-- Index for efficient queries
CREATE INDEX idx_position_states_user_status ON position_states(user_id, status);
CREATE INDEX idx_position_states_chain ON position_states(chain_id);
CREATE INDEX idx_position_states_symbol ON position_states(user_id, symbol, status);
```

**Technical Implementation:**
```go
// internal/orders/position_tracker.go
type PositionTracker struct {
    db     *database.Repository
    cache  *cache.CacheService
}

type PositionState struct {
    ID                 int64     `json:"id"`
    UserID             int64     `json:"user_id"`
    ChainID            string    `json:"chain_id"`
    Symbol             string    `json:"symbol"`
    EntryOrderID       int64     `json:"entry_order_id"`
    EntryClientOrderID string    `json:"entry_client_order_id"`
    EntrySide          string    `json:"entry_side"`
    EntryPrice         float64   `json:"entry_price"`
    EntryQuantity      float64   `json:"entry_quantity"`
    EntryValue         float64   `json:"entry_value"`
    EntryFees          float64   `json:"entry_fees"`
    EntryFilledAt      time.Time `json:"entry_filled_at"`
    Status             string    `json:"status"`
    RemainingQuantity  float64   `json:"remaining_quantity"`
    RealizedPnL        float64   `json:"realized_pnl"`
    CreatedAt          time.Time `json:"created_at"`
    UpdatedAt          time.Time `json:"updated_at"`
    ClosedAt           *time.Time `json:"closed_at,omitempty"`
}

// Called when entry order fill is detected
func (pt *PositionTracker) OnEntryFilled(ctx context.Context, order *BinanceOrder) (*PositionState, error) {
    // Parse chain ID from client order ID
    parsed := ParseClientOrderId(order.ClientOrderID)
    if parsed == nil || parsed.OrderType != OrderTypeEntry {
        return nil, fmt.Errorf("not an entry order: %s", order.ClientOrderID)
    }

    // Calculate entry value
    entryValue := order.AvgPrice * order.ExecutedQty

    // Create position state
    position := &PositionState{
        UserID:             order.UserID,
        ChainID:            parsed.ChainId,
        Symbol:             order.Symbol,
        EntryOrderID:       order.OrderID,
        EntryClientOrderID: order.ClientOrderID,
        EntrySide:          order.Side,
        EntryPrice:         order.AvgPrice,
        EntryQuantity:      order.ExecutedQty,
        EntryValue:         entryValue,
        EntryFees:          order.Commission,
        EntryFilledAt:      time.UnixMilli(order.UpdateTime),
        Status:             "ACTIVE",
        RemainingQuantity:  order.ExecutedQty,
        RealizedPnL:        0,
    }

    // Persist to database
    err := pt.db.CreatePositionState(ctx, position)
    if err != nil {
        return nil, err
    }

    // Cache for quick access
    pt.cache.SetPositionState(ctx, position)

    return position, nil
}

// Called when partial take profit hits
func (pt *PositionTracker) OnPartialClose(ctx context.Context, chainID string, closedQty float64, closePnL float64) error {
    position, err := pt.db.GetPositionByChainID(ctx, chainID)
    if err != nil {
        return err
    }

    position.RemainingQuantity -= closedQty
    position.RealizedPnL += closePnL

    if position.RemainingQuantity <= 0 {
        position.Status = "CLOSED"
        now := time.Now()
        position.ClosedAt = &now
    } else {
        position.Status = "PARTIAL"
    }

    position.UpdatedAt = time.Now()

    return pt.db.UpdatePositionState(ctx, position)
}
```

**API Response Enhancement:**
```typescript
// Enhanced TradeChainOrder to include position state
interface TradeChainOrder {
    // ... existing fields ...

    // Position state (only for entry orders that have filled)
    positionState?: {
        status: "ACTIVE" | "PARTIAL" | "CLOSED";
        entryPrice: number;
        entryQuantity: number;
        entryValue: number;
        remainingQuantity: number;
        realizedPnl: number;
        entryFilledAt: string;  // ISO 8601
        closedAt?: string;      // ISO 8601
    };
}
```

**UI Display:**
```
Chain: ULT-17JAN-00001
├── 📥 Entry (E)    │ 09:15:32 │ $97,450 │ ✅ FILLED │ 0.01 BTC
├── 📈 POSITION     │ 09:15:33 │ $97,455 │ ✅ ACTIVE │ Unrealized: +$45.00
├── 🛡️ SL Placed    │ 09:15:34 │ $96,500 │ ⏳ Active │ -1.0%
└── 🎯 TP1 Placed   │ 09:15:34 │ $98,000 │ ⏳ Active │ +0.5%
```

**Integration Points:**
- Hook into WebSocket order updates to detect fills
- Hook into Ginie autopilot after entry order placement
- Update existing `GetAllOrders` handler to include position states
- Modify `TradeLifecycleTab.tsx` to display position stage

---

### Story 7.12: Order Modification Event Log

**Goal:** Capture every modification to SL/TP orders with full audit trail including price changes, dollar impact, and LLM decision reasoning.

**Problem Statement:**
Currently, when Ginie or the user modifies a Stop Loss or Take Profit:
- Only the current price is visible
- No history of previous prices
- No record of WHY the change was made
- No calculation of how the change affects potential P&L
- Users cannot audit or understand the AI's risk management decisions

**Acceptance Criteria:**
- [ ] Capture every SL/TP price modification event
- [ ] Store old price, new price, and price delta
- [ ] Calculate dollar impact based on position size
- [ ] Store LLM reasoning/decision for automated modifications
- [ ] Support manual modification tracking (user-initiated)
- [ ] Link modifications to the chain ID for grouping
- [ ] Provide API to retrieve modification history per order type
- [ ] Handle trailing stop modifications with special tracking
- [ ] Track modification source: LLM_AUTO, USER_MANUAL, TRAILING_STOP

**Database Schema:**
```sql
-- Order modification event log
CREATE TABLE order_modification_events (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) NOT NULL,
    chain_id VARCHAR(30) NOT NULL,              -- "ULT-17JAN-00001"
    order_type VARCHAR(10) NOT NULL,            -- "SL", "TP1", "TP2", etc.
    binance_order_id BIGINT,                    -- Binance order ID (if known)

    -- Event classification
    event_type VARCHAR(20) NOT NULL,            -- "PLACED", "MODIFIED", "CANCELLED", "FILLED"
    modification_source VARCHAR(20),            -- "LLM_AUTO", "USER_MANUAL", "TRAILING_STOP"
    version INTEGER NOT NULL DEFAULT 1,         -- Incrementing version per order

    -- Price tracking
    old_price DECIMAL(18, 8),                   -- NULL for initial placement
    new_price DECIMAL(18, 8) NOT NULL,
    price_delta DECIMAL(18, 8),                 -- new_price - old_price (can be negative)
    price_delta_percent DECIMAL(8, 4),          -- Percentage change

    -- Position context (at time of modification)
    position_quantity DECIMAL(18, 8),           -- Current position size
    position_entry_price DECIMAL(18, 8),        -- Entry price for reference

    -- Dollar impact calculation
    dollar_impact DECIMAL(18, 2),               -- How much this change affects potential P&L
    impact_direction VARCHAR(10),               -- "BETTER" or "WORSE" for risk

    -- LLM decision tracking
    modification_reason TEXT,                   -- Human-readable reason
    llm_decision_id VARCHAR(50),                -- Link to decision/event log
    llm_confidence DECIMAL(5, 2),               -- Confidence score (0-100)
    market_context JSONB,                       -- Price, trend, volatility at time of change

    -- Metadata
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Indexes
    CONSTRAINT pk_modification_event PRIMARY KEY (id)
);

-- Indexes for efficient queries
CREATE INDEX idx_mod_events_chain ON order_modification_events(chain_id, order_type);
CREATE INDEX idx_mod_events_user_time ON order_modification_events(user_id, created_at DESC);
CREATE INDEX idx_mod_events_source ON order_modification_events(modification_source);
```

**Technical Implementation:**
```go
// internal/orders/modification_tracker.go
type ModificationTracker struct {
    db     *database.Repository
    cache  *cache.CacheService
}

type OrderModificationEvent struct {
    ID                  int64          `json:"id"`
    UserID              int64          `json:"user_id"`
    ChainID             string         `json:"chain_id"`
    OrderType           string         `json:"order_type"`
    BinanceOrderID      *int64         `json:"binance_order_id,omitempty"`
    EventType           string         `json:"event_type"`
    ModificationSource  string         `json:"modification_source"`
    Version             int            `json:"version"`
    OldPrice            *float64       `json:"old_price,omitempty"`
    NewPrice            float64        `json:"new_price"`
    PriceDelta          *float64       `json:"price_delta,omitempty"`
    PriceDeltaPercent   *float64       `json:"price_delta_percent,omitempty"`
    PositionQuantity    float64        `json:"position_quantity"`
    PositionEntryPrice  float64        `json:"position_entry_price"`
    DollarImpact        float64        `json:"dollar_impact"`
    ImpactDirection     string         `json:"impact_direction"`
    ModificationReason  string         `json:"modification_reason"`
    LLMDecisionID       string         `json:"llm_decision_id,omitempty"`
    LLMConfidence       *float64       `json:"llm_confidence,omitempty"`
    MarketContext       map[string]any `json:"market_context,omitempty"`
    CreatedAt           time.Time      `json:"created_at"`
}

// Called when SL/TP is first placed
func (mt *ModificationTracker) OnOrderPlaced(ctx context.Context, req PlaceOrderEvent) error {
    event := &OrderModificationEvent{
        UserID:             req.UserID,
        ChainID:            req.ChainID,
        OrderType:          req.OrderType,
        BinanceOrderID:     req.BinanceOrderID,
        EventType:          "PLACED",
        ModificationSource: req.Source,
        Version:            1,
        OldPrice:           nil,
        NewPrice:           req.Price,
        PriceDelta:         nil,
        PositionQuantity:   req.PositionQty,
        PositionEntryPrice: req.EntryPrice,
        DollarImpact:       mt.calculateImpact(req.EntryPrice, req.Price, req.PositionQty, req.OrderType),
        ImpactDirection:    "INITIAL",
        ModificationReason: req.Reason,
        LLMDecisionID:      req.DecisionID,
        LLMConfidence:      req.Confidence,
        MarketContext:      req.MarketContext,
    }

    return mt.db.CreateModificationEvent(ctx, event)
}

// Called when SL/TP price is modified
func (mt *ModificationTracker) OnOrderModified(ctx context.Context, req ModifyOrderEvent) error {
    // Get previous version
    prevVersion, err := mt.db.GetLatestModificationVersion(ctx, req.ChainID, req.OrderType)
    if err != nil {
        prevVersion = 0
    }

    // Calculate deltas
    priceDelta := req.NewPrice - req.OldPrice
    priceDeltaPercent := (priceDelta / req.OldPrice) * 100

    // Calculate dollar impact
    dollarImpact := mt.calculateDollarImpact(
        req.EntryPrice,
        req.OldPrice,
        req.NewPrice,
        req.PositionQty,
        req.OrderType,
        req.Side,
    )

    // Determine if change is better or worse for trader
    impactDirection := mt.determineImpactDirection(req.OrderType, priceDelta, req.Side)

    event := &OrderModificationEvent{
        UserID:             req.UserID,
        ChainID:            req.ChainID,
        OrderType:          req.OrderType,
        BinanceOrderID:     req.BinanceOrderID,
        EventType:          "MODIFIED",
        ModificationSource: req.Source,
        Version:            prevVersion + 1,
        OldPrice:           &req.OldPrice,
        NewPrice:           req.NewPrice,
        PriceDelta:         &priceDelta,
        PriceDeltaPercent:  &priceDeltaPercent,
        PositionQuantity:   req.PositionQty,
        PositionEntryPrice: req.EntryPrice,
        DollarImpact:       dollarImpact,
        ImpactDirection:    impactDirection,
        ModificationReason: req.Reason,
        LLMDecisionID:      req.DecisionID,
        LLMConfidence:      req.Confidence,
        MarketContext:      req.MarketContext,
    }

    return mt.db.CreateModificationEvent(ctx, event)
}

// Calculate dollar impact of price change
func (mt *ModificationTracker) calculateDollarImpact(
    entryPrice, oldPrice, newPrice, quantity float64,
    orderType, side string,
) float64 {
    // For LONG position:
    //   SL moved UP = more risk locked in (BETTER - less potential loss)
    //   SL moved DOWN = more potential loss (WORSE)
    //   TP moved UP = more profit potential (BETTER)
    //   TP moved DOWN = less profit potential (WORSE)

    oldDistance := math.Abs(oldPrice - entryPrice) * quantity
    newDistance := math.Abs(newPrice - entryPrice) * quantity

    return newDistance - oldDistance  // Positive = more P&L potential
}

// Determine if modification improves or worsens position
func (mt *ModificationTracker) determineImpactDirection(orderType string, priceDelta float64, side string) string {
    isLong := side == "BUY"
    isSL := orderType == "SL"

    if isSL {
        if isLong {
            // LONG: SL moved up = tighter (less loss), down = wider (more loss)
            if priceDelta > 0 {
                return "TIGHTER"  // SL closer to entry = locked profit
            }
            return "WIDER"
        } else {
            // SHORT: opposite
            if priceDelta < 0 {
                return "TIGHTER"
            }
            return "WIDER"
        }
    } else {
        // For TP: further = better, closer = worse
        if isLong {
            if priceDelta > 0 {
                return "BETTER"  // TP higher = more profit
            }
            return "WORSE"
        } else {
            if priceDelta < 0 {
                return "BETTER"
            }
            return "WORSE"
        }
    }
}
```

**API Endpoint:**
```go
// GET /api/futures/trade-lifecycle/:chainId/modifications?orderType=SL
func (h *FuturesLifecycleHandler) GetOrderModificationHistory(w http.ResponseWriter, r *http.Request) {
    chainID := chi.URLParam(r, "chainId")
    orderType := r.URL.Query().Get("orderType")

    events, err := h.db.GetModificationEvents(ctx, chainID, orderType)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    json.NewEncoder(w).Encode(ModificationHistoryResponse{
        ChainID:   chainID,
        OrderType: orderType,
        Events:    events,
        Summary: ModificationSummary{
            TotalModifications: len(events) - 1,
            NetPriceChange:     calculateNetChange(events),
            NetDollarImpact:    calculateNetImpact(events),
        },
    })
}
```

**Integration with Ginie Autopilot:**
```go
// In ginie_autopilot.go - when modifying SL
func (g *GinieAutopilot) updateStopLoss(ctx context.Context, position *Position, newSLPrice float64, reason string) error {
    oldSLPrice := position.CurrentSLPrice

    // Place new SL order on Binance
    newOrder, err := g.placeStopLossOrder(ctx, position, newSLPrice)
    if err != nil {
        return err
    }

    // Track modification event
    return g.modTracker.OnOrderModified(ctx, ModifyOrderEvent{
        UserID:         position.UserID,
        ChainID:        position.ChainID,
        OrderType:      "SL",
        BinanceOrderID: &newOrder.OrderID,
        OldPrice:       oldSLPrice,
        NewPrice:       newSLPrice,
        PositionQty:    position.Quantity,
        EntryPrice:     position.EntryPrice,
        Side:           position.Side,
        Source:         "LLM_AUTO",
        Reason:         reason,
        DecisionID:     g.currentDecisionID,
        Confidence:     g.currentConfidence,
        MarketContext: map[string]any{
            "current_price":   g.currentPrice,
            "price_change_1h": g.priceChange1h,
            "volatility":      g.currentVolatility,
        },
    })
}
```

---

### Story 7.13: Tree Structure UI for Modification History

**Goal:** Display SL/TP modification history in an expandable tree structure showing version history, price changes, dollar impact, and LLM reasoning.

**Problem Statement:**
Currently, the Trade Lifecycle tab shows orders as flat timeline events:
- Only current SL/TP price visible
- No indication that modifications occurred
- No way to see history of changes
- No visibility into AI decision reasoning

**Acceptance Criteria:**
- [ ] Expandable/collapsible nodes for SL and each TP level
- [ ] Badge showing modification count (e.g., "SL (3 changes)")
- [ ] Tree view showing all versions with timestamps
- [ ] Color coding: green for favorable changes, red for unfavorable
- [ ] Dollar impact display (+$125.50 or -$50.00)
- [ ] Price delta display (+$100 or -$50)
- [ ] LLM reasoning displayed for each modification
- [ ] Percentage change from previous version
- [ ] Quick comparison: Initial vs Current values
- [ ] Expandable market context for each modification
- [ ] Mobile-responsive tree display

**UI Component Structure:**
```
web/src/components/TradeLifecycle/
├── ModificationHistory/
│   ├── ModificationTree.tsx         # Main tree container
│   ├── ModificationNode.tsx         # Individual modification entry
│   ├── ModificationSummary.tsx      # Header with quick stats
│   ├── ImpactBadge.tsx              # +$125 / -$50 badge
│   ├── ReasoningTooltip.tsx         # LLM reasoning popup
│   └── types.ts                     # TypeScript interfaces
```

**TypeScript Interfaces:**
```typescript
interface ModificationEvent {
    id: number;
    chainId: string;
    orderType: string;
    eventType: "PLACED" | "MODIFIED" | "CANCELLED" | "FILLED";
    modificationSource: "LLM_AUTO" | "USER_MANUAL" | "TRAILING_STOP";
    version: number;

    oldPrice: number | null;
    newPrice: number;
    priceDelta: number | null;
    priceDeltaPercent: number | null;

    dollarImpact: number;
    impactDirection: "BETTER" | "WORSE" | "TIGHTER" | "WIDER" | "INITIAL";

    modificationReason: string;
    llmDecisionId?: string;
    llmConfidence?: number;

    marketContext?: {
        currentPrice: number;
        priceChange1h: number;
        volatility: number;
    };

    createdAt: string;
}

interface ModificationTreeProps {
    chainId: string;
    orderType: string;
    currentPrice: number;
    events: ModificationEvent[];
    isExpanded: boolean;
    onToggle: () => void;
}

interface ModificationSummaryStats {
    totalModifications: number;
    netPriceChange: number;
    netDollarImpact: number;
    initialPrice: number;
    currentPrice: number;
    lastModifiedAt: string;
}
```

**Visual Display Example:**
```
┌─────────────────────────────────────────────────────────────────────┐
│ 🛡️ Stop Loss                          (3 changes)    $96,200  +$75 │
├─────────────────────────────────────────────────────────────────────┤
│ Initial: $96,050 → Current: $96,200 | Net Impact: +$75.00           │
├─────────────────────────────────────────────────────────────────────┤
│ ● Current: $96,200                                                  │
│                                                                     │
│ ├─ v3  🤖 $96,200  +$100 (+0.10%)                    +$50   14:32:15│
│ │      💡 "Moved SL up to lock in profits after 1.5% gain..."       │
│ │                                                                   │
│ ├─ v2  🤖 $96,100  +$50 (+0.05%)                     +$25   12:15:42│
│ │      💡 "Trailing stop adjustment - price moved favorably"        │
│ │                                                                   │
│ └─ v1  ⚫ $96,050  (initial)                          N/A   09:15:34│
│        💡 "Initial SL placement at 1% below entry"                  │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│ 🎯 Take Profit 1                      (2 changes)    $98,000  -$25  │
├─────────────────────────────────────────────────────────────────────┤
│ Initial: $98,050 → Current: $98,000 | Net Impact: -$25.00           │
├─────────────────────────────────────────────────────────────────────┤
│ ● Current: $98,000                                                  │
│                                                                     │
│ ├─ v2  🤖 $98,000  -$50 (-0.05%)                     -$25   11:45:22│
│ │      💡 "Lowered TP1 for faster exit due to resistance level"     │
│ │                                                                   │
│ └─ v1  ⚫ $98,050  (initial)                          N/A   09:15:34│
│        💡 "Initial TP1 at 0.6% above entry"                         │
└─────────────────────────────────────────────────────────────────────┘
```

**Color Coding Legend:**
- 🟢 Green border/text: Favorable change (BETTER, TIGHTER for SL)
- 🔴 Red border/text: Unfavorable change (WORSE, WIDER for SL)
- ⚫ Gray: Initial placement (no comparison)
- 🤖 Purple: LLM automated modification
- 👤 Blue: User manual modification
- 📈 Yellow: Trailing stop modification

**Mobile Responsive Design:**
- Collapse tree to single-line summary on mobile
- Tap to expand full history
- Swipe gestures for navigation between orders

---

### Story 7.14: Order Chain Backend Integration

**Goal:** Create backend endpoint that returns order chains with position states and modification counts, ensuring entry orders remain visible after they fill.

**Problem Statement:**
The current `/api/futures/orders/all` endpoint only returns OPEN orders from Binance. When entry orders fill:
- They disappear from the response (Binance only returns OPEN orders)
- Position state data exists in `position_states` table but is never included
- Frontend cannot display entry order or position state in the chain

**Acceptance Criteria:**
- [ ] New endpoint `/api/futures/order-chains` returns orders with position states
- [ ] Include `positionState` field for chains where entry has filled
- [ ] Include `modificationCounts` per order type (SL: 3, TP1: 2)
- [ ] Merge Binance open orders with position_states from database
- [ ] Support filtering by status, mode, symbol
- [ ] Cache position states for performance
- [ ] Backward compatible - existing `/orders/all` unchanged

**Technical Implementation:**
```go
// internal/api/handlers_futures.go

type OrderChainWithState struct {
    ChainID             string                 `json:"chain_id"`
    ModeCode            string                 `json:"mode_code"`
    Symbol              string                 `json:"symbol"`
    PositionSide        string                 `json:"position_side"`
    Orders              []ChainOrder           `json:"orders"`
    PositionState       *PositionState         `json:"position_state,omitempty"`
    ModificationCounts  map[string]int         `json:"modification_counts"`
    Status              string                 `json:"status"`
    TotalValue          float64                `json:"total_value"`
    FilledValue         float64                `json:"filled_value"`
    CreatedAt           int64                  `json:"created_at"`
    UpdatedAt           int64                  `json:"updated_at"`
}

// GET /api/futures/order-chains
func (s *Server) handleGetOrderChainsWithState(c *gin.Context) {
    userID := getUserID(c)

    // 1. Get open orders from Binance
    openOrders, err := s.futuresClient.GetOpenOrders("")

    // 2. Group by chain ID
    chains := groupOrdersByChainID(openOrders)

    // 3. Fetch position states for all chain IDs
    chainIDs := extractChainIDs(chains)
    positionStates, err := s.db.GetPositionStatesByChainIDs(ctx, userID, chainIDs)

    // 4. Fetch modification counts
    modCounts, err := s.db.GetModificationCountsByChainIDs(ctx, chainIDs)

    // 5. Merge data
    result := make([]OrderChainWithState, 0, len(chains))
    for chainID, orders := range chains {
        chain := OrderChainWithState{
            ChainID:            chainID,
            Orders:             orders,
            PositionState:      positionStates[chainID],
            ModificationCounts: modCounts[chainID],
        }
        result = append(result, chain)
    }

    c.JSON(200, gin.H{"chains": result})
}
```

**Database Queries:**
```go
// internal/database/repository_position_states.go
func (r *Repository) GetPositionStatesByChainIDs(ctx context.Context, userID int64, chainIDs []string) (map[string]*PositionState, error) {
    query := `SELECT * FROM position_states WHERE user_id = $1 AND chain_id = ANY($2)`
    // ...
}

// internal/database/repository_modification_events.go
func (r *Repository) GetModificationCountsByChainIDs(ctx context.Context, chainIDs []string) (map[string]map[string]int, error) {
    query := `
        SELECT chain_id, order_type, COUNT(*) as count
        FROM order_modification_events
        WHERE chain_id = ANY($1)
        GROUP BY chain_id, order_type
    `
    // ...
}
```

**Files to Create/Modify:**
| File | Action | Changes |
|------|--------|---------|
| `internal/api/handlers_futures.go` | Modify | Add `handleGetOrderChainsWithState` |
| `internal/api/server.go` | Modify | Register new route |
| `internal/database/repository_position_states.go` | Modify | Add batch query method |
| `internal/database/repository_modification_events.go` | Modify | Add count query method |
| `web/src/services/futuresApi.ts` | Modify | Add `getOrderChainsWithState()` |

**Estimated Effort:** 3-4 hours

---

### Story 7.15: Order Chain Tree Structure UI

**Goal:** Restructure ChainCard from horizontal linear layout to hierarchical tree display showing Entry → Position → [TP/SL] with nested modification history.

**Problem Statement:**
Current `ChainCard.tsx` renders orders horizontally:
```
[Entry] -- [TP1] -- [TP2] -- [SL]  ← Linear, Entry disappears when filled
```

Expected display is a tree hierarchy:
```
├── Entry (E) - FILLED ✅
│   └── POSITION (active)
│       ├── TP1 [3 modifications]
│       ├── TP2
│       └── SL [5 modifications]
```

**Acceptance Criteria:**
- [ ] Entry order visible even after filling (from position_state.entry_*)
- [ ] Position state displayed as child of entry
- [ ] TP1, TP2, TP3, SL displayed as children of position (parallel, not sequential)
- [ ] Each TP/SL order expandable to show modification history
- [ ] Modification count badge on each order (e.g., "SL (3)")
- [ ] Tree connectors (├── └──) for visual hierarchy
- [ ] Collapsible sub-trees for cleaner display
- [ ] Timezone-aware timestamps using user's timezone setting
- [ ] Mobile-responsive tree layout

**New Frontend Types:**
```typescript
// web/src/components/TradeLifecycle/types.ts

export interface PositionState {
    id: number;
    chainId: string;
    symbol: string;
    entryOrderId: number;
    entryClientOrderId: string;
    entrySide: 'BUY' | 'SELL';
    entryPrice: number;
    entryQuantity: number;
    entryValue: number;
    entryFees: number;
    entryFilledAt: string;
    status: 'ACTIVE' | 'PARTIAL' | 'CLOSED';
    remainingQuantity: number;
    realizedPnl: number;
    createdAt: string;
    updatedAt: string;
    closedAt?: string;
}

export interface OrderChainWithState extends OrderChain {
    positionState?: PositionState;
    modificationCounts?: Record<string, number>;
}

// Add POSITION to display types
export const ORDER_TYPE_CONFIG = {
    // ... existing
    POSITION: {
        label: 'Position',
        color: 'text-purple-400',
        bgColor: 'bg-purple-500/20',
        description: 'Active position from filled entry'
    },
};
```

**New Component: OrderTreeNode.tsx**
```typescript
// web/src/components/TradeLifecycle/OrderTreeNode.tsx

interface OrderTreeNodeProps {
    type: 'ENTRY' | 'POSITION' | 'TP1' | 'TP2' | 'TP3' | 'SL';
    order?: ChainOrder;
    positionState?: PositionState;
    modificationCount?: number;
    modifications?: ModificationEvent[];
    isLast?: boolean;
    depth: number;
}

export function OrderTreeNode({ type, order, positionState, modificationCount, modifications, isLast, depth }: OrderTreeNodeProps) {
    const [expanded, setExpanded] = useState(false);

    return (
        <div className="tree-node">
            {/* Tree connector */}
            <div className="connector">
                {depth > 0 && (isLast ? '└──' : '├──')}
            </div>

            {/* Node content */}
            <div className={`node-content ${ORDER_TYPE_CONFIG[type].bgColor}`}>
                <span className={ORDER_TYPE_CONFIG[type].color}>{ORDER_TYPE_CONFIG[type].label}</span>

                {/* Price/Status */}
                {order && <span>{formatPrice(order.price)}</span>}
                {positionState && <span>{positionState.status}</span>}

                {/* Modification badge */}
                {modificationCount && modificationCount > 0 && (
                    <button onClick={() => setExpanded(!expanded)} className="mod-badge">
                        ({modificationCount} changes)
                    </button>
                )}
            </div>

            {/* Nested modification history */}
            {expanded && modifications && (
                <div className="nested-mods pl-6">
                    <ModificationTree events={modifications} />
                </div>
            )}
        </div>
    );
}
```

**Updated ChainCard Structure:**
```typescript
// web/src/components/TradeLifecycle/ChainCard.tsx (restructured)

<div className="chain-tree">
    {/* Entry Order - always visible */}
    <OrderTreeNode
        type="ENTRY"
        order={chain.entryOrder || buildEntryFromPositionState(chain.positionState)}
        depth={0}
    />

    {/* Position - child of entry (only if position state exists) */}
    {chain.positionState && (
        <div className="position-branch pl-4">
            <OrderTreeNode
                type="POSITION"
                positionState={chain.positionState}
                depth={1}
            />

            {/* TP/SL - children of position (parallel) */}
            <div className="exit-orders pl-4">
                {chain.tpOrders.map((tp, idx) => (
                    <OrderTreeNode
                        key={tp.orderId}
                        type={tp.orderType as any}
                        order={tp}
                        modificationCount={chain.modificationCounts?.[tp.orderType]}
                        depth={2}
                        isLast={idx === chain.tpOrders.length - 1 && !chain.slOrder}
                    />
                ))}

                {chain.slOrder && (
                    <OrderTreeNode
                        type="SL"
                        order={chain.slOrder}
                        modificationCount={chain.modificationCounts?.SL}
                        depth={2}
                        isLast={true}
                    />
                )}
            </div>
        </div>
    )}
</div>
```

**Files to Create/Modify:**
| File | Action | Changes |
|------|--------|---------|
| `web/src/components/TradeLifecycle/types.ts` | Modify | Add PositionState, OrderChainWithState |
| `web/src/components/TradeLifecycle/OrderTreeNode.tsx` | Create | New tree node component |
| `web/src/components/TradeLifecycle/ChainCard.tsx` | Modify | Replace horizontal with tree layout |
| `web/src/components/TradeLifecycle/TradeLifecycleTab.tsx` | Modify | Use new API endpoint |
| `web/src/styles/tree.css` | Create | Tree connector styles |

**CSS for Tree Connectors:**
```css
/* web/src/styles/tree.css */
.chain-tree {
    --tree-line-color: #4B5563;
}

.tree-node {
    display: flex;
    align-items: flex-start;
}

.connector {
    font-family: monospace;
    color: var(--tree-line-color);
    min-width: 3ch;
}

.position-branch {
    border-left: 1px solid var(--tree-line-color);
    margin-left: 1ch;
}

.exit-orders {
    border-left: 1px solid var(--tree-line-color);
    margin-left: 1ch;
}
```

**Estimated Effort:** 4-5 hours

---

## Dependencies

| Dependency | Type | Status |
|------------|------|--------|
| Epic 6 - Redis Infrastructure | Prerequisite | Story 7.2 needs Redis |
| Binance clientOrderId support | External | Available |
| User timezone settings | Database | To implement in 7.6 |

---

## Success Criteria

1. **ID Generation Working**: All orders have structured clientOrderId
2. **Parsing Accurate**: Can extract mode/date/seq/type from any order
3. **Chain Grouping**: Related orders correctly grouped in UI
4. **Sequence Atomic**: No duplicate sequences under concurrent load
5. **Timezone Correct**: Date matches user's timezone setting
6. **Trade Lifecycle Tab**: Shows complete trade journeys
7. **Hedge Support**: Hedge orders linked to parent chain
8. **Five Modes Supported**: ULT, SCA, SCR, SWI, POS all working
9. **Redis Fallback Working**: Orders continue if Redis fails
10. **API Endpoints Live**: Trade Lifecycle API returns correct data
11. **Edge Cases Covered**: All edge case tests passing
12. **Position State Tracked**: Entry orders persist as positions when filled
13. **Modification History Complete**: All SL/TP changes logged with reasoning
14. **Tree UI Functional**: Expandable modification history with dollar impact
15. **Order Chain Integration**: Position states merged with orders in API response (7.14)
16. **Tree Structure Display**: Entry → Position → TP/SL hierarchy renders correctly (7.15)

---

## Technical Considerations

### New Files

```
internal/orders/
├── client_order_id.go          # Generator (with fallback logic)
├── client_order_id_parser.go   # Parser
├── client_order_id_test.go     # Edge case tests (Story 7.10)
├── types.go                    # OrderType, TradingMode enums
├── chain_tracker.go            # Chain state management
├── position_tracker.go         # Position state tracking (Story 7.11)
└── modification_tracker.go     # Order modification event log (Story 7.12)

internal/api/
└── futures_lifecycle_handlers.go  # Trade Lifecycle API endpoints (Story 7.9)

internal/database/
├── repository_position_states.go      # Position state persistence (Story 7.11)
└── repository_modification_events.go  # Modification events persistence (Story 7.12)

web/src/components/TradeLifecycle/
├── TradeLifecycleTab.tsx
├── ChainCard.tsx
├── ChainTimeline.tsx
├── ChainFilters.tsx
├── ChainSummary.tsx
└── ModificationHistory/        # Story 7.13 components
    ├── ModificationTree.tsx
    ├── ModificationNode.tsx
    ├── ModificationSummary.tsx
    ├── ImpactBadge.tsx
    ├── ReasoningTooltip.tsx
    └── types.ts

web/src/services/
└── tradeLifecycleApi.ts        # API client for lifecycle data (Story 7.9)
```

### Modified Files

| File | Changes |
|------|---------|
| internal/autopilot/ginie_autopilot.go | Use ClientOrderIdGenerator, integrate PositionTracker & ModificationTracker |
| internal/autopilot/futures_controller.go | Pass clientOrderId to orders |
| internal/binance/futures_client.go | Accept clientOrderId param |
| web/src/pages/TradingDashboard.tsx | Add Trade Lifecycle tab |
| web/src/services/futuresApi.ts | Add lifecycle endpoints, modification history endpoints |
| internal/api/handlers_futures.go | Include position states in order chain response |
| web/src/components/TradeLifecycle/ChainCard.tsx | Integrate ModificationTree component |
| internal/database/migrations/ | Add position_states and order_modification_events tables |

### Binance API Integration

Orders are placed with `newClientOrderId` parameter:
```go
// Place order with structured ID
order, err := client.NewCreateOrderService().
    Symbol("BTCUSDT").
    Side(binance.SideTypeBuy).
    Type(binance.OrderTypeMarket).
    Quantity("0.001").
    NewClientOrderID("ULT-06JAN-00001-E").  // Our structured ID
    Do(ctx)
```

Orders returned from Binance include `clientOrderId`:
```json
{
    "orderId": 123456789,
    "symbol": "BTCUSDT",
    "clientOrderId": "ULT-06JAN-00001-E",
    "status": "FILLED",
    ...
}
```

---

## Testing Strategy

### Unit Tests
- ID generation format validation (Story 7.1)
- Parsing edge cases (malformed, legacy IDs) (Story 7.4, 7.10)
- Sequence increment atomicity (Story 7.2, 7.10)
- Redis fallback ID generation (Story 7.8, 7.10)
- All 5 modes (ULT, SCA, SCR, SWI, POS) (Story 7.10)
- All order types (E, SL, TP1-TP4, H, HSL, HTP, DCA1-DCA3) (Story 7.10)

### Integration Tests
- Full order placement with clientOrderId (Story 7.1)
- Binance accepts our ID format (normal + fallback) (Story 7.10)
- Round-trip: Generate → Place → Retrieve → Parse (Story 7.10)
- Midnight rollover sequence reset (Story 7.10)
- Year boundary handling (Story 7.10)
- Concurrent sequence generation (Story 7.10)

### API Tests
- Trade Lifecycle list endpoint (Story 7.9)
- Trade Lifecycle detail endpoint (Story 7.9)
- Query parameter filtering (mode, date, symbol, status) (Story 7.9)
- Pagination (limit, offset) (Story 7.9)

### UI Tests
- Trade Lifecycle tab rendering (Story 7.5)
- Chain grouping accuracy (Story 7.5)
- Filter and search functionality (Story 7.5)
- Fallback ID display with warning icon (Story 7.8)

---

## Author

**Created By:** BMAD Party Mode (Analyst: Mary, Architect: Winston, PM: John, Dev: Amelia)
**Date:** 2026-01-06
**Version:** 1.0
**Depends On:** Epic 6 (Redis)
