# Story 7.9: Backend API for Trade Lifecycle Tab
**Epic:** Epic 7 - Client Order ID & Trade Lifecycle Tracking
**Sprint:** Sprint 7
**Story Points:** 5
**Priority:** P1

## User Story
As a frontend developer, I want REST API endpoints to retrieve trade lifecycle data so that I can display complete trade chains in the Trade Lifecycle tab.

## Acceptance Criteria
- [ ] GET `/api/futures/trade-lifecycle` - List all trade chains
- [ ] GET `/api/futures/trade-lifecycle/:chainId` - Get single chain details
- [ ] Query parameters: mode, startDate, endDate, symbol, status, limit, offset
- [ ] Response includes all orders in chain with parsed metadata
- [ ] Response includes P&L calculation per chain
- [ ] Response includes chain duration and summary stats
- [ ] Efficient query with index on clientOrderId prefix
- [ ] Cache frequently accessed chains in Redis (optional)
- [ ] Error handling for invalid parameters
- [ ] Authentication required (user can only see own chains)

## Technical Approach

1. **API Endpoints Specification**:

   **List Trade Chains**:
   ```
   GET /api/futures/trade-lifecycle
   Query Parameters:
     - mode: string (ULT, SCA, SCR, SWI, POS) - optional
     - startDate: ISO 8601 date - optional
     - endDate: ISO 8601 date - optional
     - symbol: string (BTCUSDT, etc.) - optional
     - status: string (active, closed, partial) - optional
     - limit: integer (default 50, max 200)
     - offset: integer (default 0)

   Response:
   {
     "chains": TradeChainSummary[],
     "total": number,
     "limit": number,
     "offset": number
   }
   ```

   **Get Chain Detail**:
   ```
   GET /api/futures/trade-lifecycle/:chainId

   Response:
   {
     "chain": TradeChainSummary,
     "orders": TradeChainOrder[],
     "timeline": TradeChainEvent[]
   }
   ```

2. **Data Models**:
   ```go
   type TradeChainSummary struct {
       ChainId     string    `json:"chainId"`
       Mode        string    `json:"mode"`
       Symbol      string    `json:"symbol"`
       Direction   string    `json:"direction"` // "LONG" or "SHORT"
       Status      string    `json:"status"`    // "active", "closed", "partial"
       StartTime   time.Time `json:"startTime"`
       EndTime     *time.Time `json:"endTime"`
       Duration    *int64    `json:"duration"`  // Seconds
       PnL         float64   `json:"pnl"`
       PnLPercent  float64   `json:"pnlPercent"`
       Fees        float64   `json:"fees"`
       OrderCount  int       `json:"orderCount"`
       IsFallback  bool      `json:"isFallback"`
   }

   type TradeChainOrder struct {
       OrderId          string      `json:"orderId"`
       ClientOrderId    string      `json:"clientOrderId"`
       OrderType        string      `json:"orderType"`
       Symbol           string      `json:"symbol"`
       Side             string      `json:"side"`
       Type             string      `json:"type"`
       Status           string      `json:"status"`
       Price            float64     `json:"price"`
       Quantity         float64     `json:"quantity"`
       ExecutedQty      float64     `json:"executedQty"`
       CreatedAt        time.Time   `json:"createdAt"`
       UpdatedAt        time.Time   `json:"updatedAt"`
       Fills            []OrderFill `json:"fills"`
   }

   type TradeChainEvent struct {
       Time        time.Time              `json:"time"`
       Stage       string                 `json:"stage"`
       Description string                 `json:"description"`
       Price       *float64               `json:"price"`
       Status      string                 `json:"status"`
       Details     map[string]interface{} `json:"details"`
   }
   ```

3. **Backend Implementation**:
   ```go
   func (h *FuturesLifecycleHandler) ListTradeChains(w http.ResponseWriter, r *http.Request) {
       userID := getUserIDFromContext(r.Context())

       // Parse query params
       filters := parseFilters(r)

       // Fetch orders (from Binance API or database)
       orders, err := h.orderService.GetOrders(userID, filters)

       // Group by chain base ID
       chains := groupOrdersByChainId(orders)

       // Calculate summaries
       summaries := make([]TradeChainSummary, 0, len(chains))
       for _, chain := range chains {
           summary := calculateChainSummary(chain)
           summaries = append(summaries, summary)
       }

       // Apply pagination
       paginated := paginateResults(summaries, filters.limit, filters.offset)

       respondJSON(w, TradeLifecycleListResponse{
           Chains: paginated,
           Total:  len(summaries),
           Limit:  filters.limit,
           Offset: filters.offset,
       })
   }

   func (h *FuturesLifecycleHandler) GetTradeChainDetail(w http.ResponseWriter, r *http.Request) {
       userID := getUserIDFromContext(r.Context())
       chainId := chi.URLParam(r, "chainId")

       // Query all orders with this chain base
       orders, err := h.orderRepository.GetOrdersByChainId(userID, chainId)

       // Build timeline
       timeline := buildChainTimeline(orders)

       // Calculate summary
       summary := calculateChainSummary(orders)

       respondJSON(w, TradeChainDetailResponse{
           Chain:    summary,
           Orders:   orders,
           Timeline: timeline,
       })
   }
   ```

4. **Chain Grouping Logic**:
   ```go
   func groupOrdersByChainId(orders []*Order) map[string][]*Order {
       chains := make(map[string][]*Order)

       for _, order := range orders {
           parsed := ParseClientOrderId(order.ClientOrderId)
           if parsed == nil {
               continue // Skip unparseable orders
           }

           chainId := parsed.ChainId
           chains[chainId] = append(chains[chainId], order)
       }

       return chains
   }
   ```

5. **P&L Calculation**:
   ```go
   func calculateChainSummary(orders []*Order) TradeChainSummary {
       var totalPnL, totalFees float64
       var startTime, endTime time.Time
       var direction string

       for _, order := range orders {
           // Accumulate P&L from fills
           for _, fill := range order.Fills {
               if order.Side == "BUY" {
                   totalPnL -= fill.Price * fill.Quantity
               } else {
                   totalPnL += fill.Price * fill.Quantity
               }
               totalFees += fill.Commission
           }

           // Track timestamps
           if startTime.IsZero() || order.CreatedAt.Before(startTime) {
               startTime = order.CreatedAt
           }
           if order.UpdatedAt.After(endTime) {
               endTime = order.UpdatedAt
           }
       }

       // Determine direction from entry order
       entryOrder := findEntryOrder(orders)
       if entryOrder != nil {
           direction = entryOrder.Side // "BUY" → "LONG"
       }

       duration := int64(endTime.Sub(startTime).Seconds())

       return TradeChainSummary{
           ChainId:    orders[0].ChainId,
           PnL:        totalPnL,
           PnLPercent: (totalPnL / getEntryValue(orders)) * 100,
           Fees:       totalFees,
           Duration:   &duration,
           StartTime:  startTime,
           EndTime:    &endTime,
           Direction:  direction,
           OrderCount: len(orders),
       }
   }
   ```

6. **Database Query Optimization**:
   ```sql
   -- Index for efficient chain lookup
   CREATE INDEX idx_orders_client_order_id_prefix ON orders
     USING btree (substring(client_order_id, 1, 17));
     -- "ULT-06JAN-00001" = 17 characters

   -- Query all orders in a chain
   SELECT * FROM orders
   WHERE user_id = $1
     AND substring(client_order_id, 1, 17) = $2
   ORDER BY created_at ASC;
   ```

7. **Timeline Building**:
   ```go
   func buildChainTimeline(orders []*Order) []TradeChainEvent {
       events := make([]TradeChainEvent, 0)

       for _, order := range orders {
           parsed := ParseClientOrderId(order.ClientOrderId)
           if parsed == nil {
               continue
           }

           stage := getStageFromOrderType(parsed.OrderType)
           events = append(events, TradeChainEvent{
               Time:        order.CreatedAt,
               Stage:       stage,
               Description: fmt.Sprintf("%s order %s", parsed.OrderType, order.Status),
               Price:       &order.Price,
               Status:      mapOrderStatus(order.Status),
               Details:     map[string]interface{}{
                   "orderId":  order.OrderId,
                   "quantity": order.Quantity,
               },
           })
       }

       return events
   }
   ```

## Dependencies
- **Blocked By:**
  - Story 7.1: Client Order ID Generation
  - Story 7.3: Order Chain Tracking
  - Story 7.4: Parse Client Order ID
- **Blocks:**
  - Story 7.5: Trade Lifecycle Tab UI (consumes this API)

## Files to Create/Modify

### Files to Create:
- `internal/api/futures_lifecycle_handlers.go` - API handlers for trade lifecycle endpoints
- `internal/services/trade_lifecycle_service.go` - Business logic for chain grouping and calculations
- `internal/database/chain_repository.go` - Database queries for chain data
- `migrations/000X_add_chain_index.sql` - Database index for clientOrderId prefix

### Files to Modify:
- `internal/api/routes.go` - Register lifecycle endpoints
- `internal/database/order_repository.go` - Add GetOrdersByChainId method
- `main.go` - Initialize TradeLifecycleService

## Testing Requirements

### Unit Tests:
- Test chain grouping logic (same base grouped)
- Test P&L calculation (entry + TP + hedge)
- Test timeline building (chronological order)
- Test query parameter parsing (mode, date, symbol)
- Test pagination logic (limit, offset)
- Test filter application (status, mode)
- Test summary calculation (duration, fees, count)

### Integration Tests:
- Test API endpoint returns valid JSON
- Test authentication required (401 if not logged in)
- Test user isolation (can't see other users' chains)
- Test query with real Binance order data
- Test chain detail includes all orders
- Test timeline events in correct order
- Test P&L matches manual calculation

### API Tests:
- Test GET /api/futures/trade-lifecycle with filters
- Test GET /api/futures/trade-lifecycle/:chainId
- Test invalid chainId returns 404
- Test pagination (limit=10, offset=20)
- Test date range filtering
- Test mode filtering (ULT, SCA, etc.)
- Test empty result handling

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Documentation updated (API documentation with examples)
- [ ] PO acceptance received
- [ ] API endpoints registered and accessible
- [ ] Database index created and optimized
- [ ] Authentication enforced
- [ ] Error handling comprehensive
- [ ] P&L calculation verified accurate
