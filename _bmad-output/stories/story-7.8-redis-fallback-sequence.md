# Story 7.8: Redis Fallback for Sequence Generation
**Epic:** Epic 7 - Client Order ID & Trade Lifecycle Tracking
**Sprint:** Sprint 7
**Story Points:** 3
**Priority:** P0

## User Story
As a trading system, I want order placement to continue even if Redis is unavailable so that traders don't lose opportunities due to infrastructure issues.

## Acceptance Criteria
- [ ] If `IncrementDailySequence()` returns error, generate fallback ID
- [ ] Fallback format: `{MODE}-FALLBACK-{8-char-uuid}-{TYPE}`
- [ ] Example: `ULT-FALLBACK-a3f7c2e9-E`
- [ ] Log WARNING when fallback is used
- [ ] Order placement continues without blocking (no error thrown)
- [ ] Fallback IDs still parseable (ChainId = base without type)
- [ ] Trade Lifecycle tab displays fallback chains with warning icon
- [ ] Health check endpoint reports Redis connection status
- [ ] Fallback IDs still group correctly by chain base

## Technical Approach

1. **Fallback ID Generation**:
   ```go
   func (g *ClientOrderIdGenerator) Generate(userID string, mode TradingMode, orderType OrderType) (string, error) {
       now := time.Now().In(g.timezone)
       dateStr := strings.ToUpper(now.Format("02Jan")) // "06JAN"

       // Try Redis sequence
       seq, err := g.cache.IncrementDailySequence(userID, now)
       if err != nil {
           // Redis unavailable - use fallback
           log.Warn().Err(err).Msg("Redis unavailable, using fallback clientOrderId")

           // Generate UUID-based fallback
           uuid := generateShortUUID() // First 8 chars of UUID
           fallbackID := fmt.Sprintf("%s-FALLBACK-%s-%s", mode.Code(), uuid, orderType)
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

2. **Fallback Chain Grouping**:
   ```
   Chain: ULT-FALLBACK-a3f7c2e9
   ├── ULT-FALLBACK-a3f7c2e9-E    (Entry)
   ├── ULT-FALLBACK-a3f7c2e9-SL   (Stop Loss)
   └── ULT-FALLBACK-a3f7c2e9-TP1  (Take Profit)
   ```

3. **Parser Updates**:
   ```go
   func ParseClientOrderId(clientOrderId string) *ParsedOrderId {
       parts := strings.Split(clientOrderId, "-")

       // Handle fallback format: MODE-FALLBACK-UUID-TYPE
       if len(parts) >= 4 && parts[1] == "FALLBACK" {
           mode := parseTradingMode(parts[0])
           if mode == "" {
               return nil
           }

           orderType := OrderType(parts[3])
           chainId := strings.Join(parts[:3], "-") // "ULT-FALLBACK-a3f7c2e9"

           return &ParsedOrderId{
               Mode:       mode,
               Date:       time.Time{}, // No date for fallback
               Sequence:   0,           // No sequence for fallback
               OrderType:  orderType,
               ChainId:    chainId,
               Raw:        clientOrderId,
               IsFallback: true,        // Flag for UI warning
           }
       }

       // Normal format parsing...
   }
   ```

4. **Health Check Integration**:
   ```go
   // GET /health
   {
       "status": "healthy",
       "services": {
           "database": "connected",
           "redis": "unavailable",    // Warning indicator
           "binance": "connected"
       },
       "warnings": [
           "Redis connection failed - using fallback IDs"
       ]
   }
   ```

5. **UI Warning Display**:
   ```tsx
   <ChainCard chainId="ULT-FALLBACK-a3f7c2e9">
       {chain.isFallback && (
           <div className="fallback-warning">
               ⚠️ Fallback ID (Redis unavailable at creation)
           </div>
       )}
       {/* Rest of chain display */}
   </ChainCard>
   ```

6. **Logging Strategy**:
   - Log WARNING on first fallback use
   - Log ERROR if Redis remains down > 5 minutes
   - Alert monitoring system (future Epic)
   - Track fallback usage metrics

## Dependencies
- **Blocked By:**
  - Story 7.1: Client Order ID Generation (implements fallback)
  - Story 7.2: Daily Sequence Storage (error handling)
  - Story 7.4: Parse Client Order ID (parse fallback format)
- **Blocks:**
  - Story 7.5: Trade Lifecycle Tab UI (display warning)
  - Story 7.10: Edge Case Test Suite (fallback tests)

## Files to Create/Modify

### Files to Create:
- `internal/orders/fallback_id.go` - Fallback ID generation utilities
- `internal/health/redis_check.go` - Redis health check logic

### Files to Modify:
- `internal/orders/client_order_id.go` - Add fallback logic to Generate()
- `internal/orders/client_order_id_parser.go` - Recognize FALLBACK format
- `internal/orders/types.go` - Add IsFallback field to ParsedOrderId
- `internal/api/health_handlers.go` - Include Redis status in health check
- `web/src/components/TradeLifecycle/ChainCard.tsx` - Display fallback warning
- `web/src/types/tradeLifecycle.ts` - Add isFallback field to chain type

## Testing Requirements

### Unit Tests:
- Test fallback ID generation when Redis returns error
- Test fallback ID format matches specification
- Test fallback IDs still parse correctly
- Test fallback chain grouping (same UUID base)
- Test generateShortUUID() creates 8-char alphanumeric
- Test fallback IDs under 36 characters
- Test logging WARNING on fallback use

### Integration Tests:
- Test order placement continues when Redis down
- Test Binance accepts fallback ID format
- Test fallback orders retrieve correctly from Binance
- Test Trade Lifecycle displays fallback chains
- Test health check reports Redis unavailable
- Test multiple fallback orders in same chain

### Edge Case Tests:
- Test Redis connection restored mid-session
- Test concurrent fallback ID generation (unique UUIDs)
- Test fallback to normal transition seamless
- Test Trade Lifecycle handles mixed normal/fallback chains

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Documentation updated (fallback behavior explained)
- [ ] PO acceptance received
- [ ] Redis failure scenario tested
- [ ] Order placement never fails due to Redis
- [ ] Health check reflects Redis status
- [ ] UI displays fallback warning icon
- [ ] Fallback IDs group correctly in Trade Lifecycle
