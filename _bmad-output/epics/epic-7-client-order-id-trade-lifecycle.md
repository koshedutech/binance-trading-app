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

---

## Technical Considerations

### New Files

```
internal/orders/
├── client_order_id.go          # Generator (with fallback logic)
├── client_order_id_parser.go   # Parser
├── client_order_id_test.go     # Edge case tests (Story 7.10)
├── types.go                    # OrderType, TradingMode enums
└── chain_tracker.go            # Chain state management

internal/api/
└── futures_lifecycle_handlers.go  # Trade Lifecycle API endpoints (Story 7.9)

web/src/components/TradeLifecycle/
├── TradeLifecycleTab.tsx
├── ChainCard.tsx
├── ChainTimeline.tsx
├── ChainFilters.tsx
└── ChainSummary.tsx

web/src/services/
└── tradeLifecycleApi.ts        # API client for lifecycle data (Story 7.9)
```

### Modified Files

| File | Changes |
|------|---------|
| internal/autopilot/ginie_autopilot.go | Use ClientOrderIdGenerator |
| internal/autopilot/futures_controller.go | Pass clientOrderId to orders |
| internal/binance/futures_client.go | Accept clientOrderId param |
| web/src/pages/TradingDashboard.tsx | Add Trade Lifecycle tab |
| web/src/services/futuresApi.ts | Add lifecycle endpoints |

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
