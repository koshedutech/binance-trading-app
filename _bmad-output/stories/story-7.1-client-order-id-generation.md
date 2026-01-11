# Story 7.1: Client Order ID Generation
**Epic:** Epic 7 - Client Order ID & Trade Lifecycle Tracking
**Sprint:** Sprint 7
**Story Points:** 5
**Priority:** P0

## User Story
As a trading system, I want to generate structured clientOrderId for every order placed so that I can trace orders through their complete lifecycle and enable mode-based analytics.

## Acceptance Criteria
- [ ] `ClientOrderIdGenerator` service with `Generate(mode, orderType)` method implemented
- [ ] Mode codes supported: ULT (Ultra Fast), SCA (Scalp), SCR (Scalp Reentry), SWI (Swing), POS (Position)
- [ ] Date format: DDMMM (user's timezone) - e.g., "06JAN"
- [ ] Sequence: 5 digits, zero-padded (00001-99999)
- [ ] Type suffixes: E, SL, TP1-TP4, H, HSL, HTP, DCA1-DCA3
- [ ] Validation: Output ≤ 36 characters (Binance limit)
- [ ] Integration with all order placement code paths
- [ ] `GenerateRelated()` method for linking orders in same chain

## Technical Approach
Create a new `ClientOrderIdGenerator` service that:

1. **Format Structure**: `[MODE]-[DDMMM]-[NNNNN]-[TYPE]`
   - Example: `ULT-06JAN-00001-E` (18-20 characters max)

2. **Component Generation**:
   - Mode: 3-character uppercase code from `TradingMode` enum
   - Date: User's timezone-aware date in format "06JAN"
   - Sequence: Increment via Redis (Story 7.2 dependency)
   - Type: Order type suffix from `OrderType` enum

3. **Two Generation Methods**:
   - `Generate()`: Creates new chain base + type for entry orders
   - `GenerateRelated()`: Reuses base ID for SL/TP/Hedge orders

4. **Integration Points**:
   - `internal/autopilot/ginie_autopilot.go` - Use for all Ginie orders
   - `internal/autopilot/futures_controller.go` - Pass to order placement
   - `internal/binance/futures_client.go` - Accept `clientOrderId` parameter

## Dependencies
- **Blocked By:**
  - Story 6.1: Redis Container Setup
  - Story 6.2: CacheService Implementation
  - Story 6.3: Redis Integration in main.go
- **Blocks:**
  - Story 7.2: Daily Sequence Storage
  - Story 7.3: Order Chain Tracking
  - Story 7.4: Parse Client Order ID

## Files to Create/Modify

### Files to Create:
- `internal/orders/client_order_id.go` - Main generator service with Generate() and GenerateRelated() methods
- `internal/orders/types.go` - OrderType and TradingMode enum definitions with constants

### Files to Modify:
- `internal/autopilot/ginie_autopilot.go` - Inject ClientOrderIdGenerator, use for all orders
- `internal/autopilot/futures_controller.go` - Accept and pass clientOrderId to Binance client
- `internal/binance/futures_client.go` - Add newClientOrderId parameter to order methods
- `main.go` - Initialize ClientOrderIdGenerator with CacheService and timezone

## Testing Requirements

### Unit Tests:
- Test ID format validation (MODE-DDMMM-NNNNN-TYPE)
- Test all 5 mode codes (ULT, SCA, SCR, SWI, POS)
- Test all order type suffixes (E, SL, TP1-TP4, H, HSL, HTP, DCA1-DCA3)
- Test character count ≤ 36 for all combinations
- Test timezone date formatting (Asia/Kolkata default)
- Test GenerateRelated() maintains chain base
- Test error handling when Redis unavailable (uses fallback)

### Integration Tests:
- Test order placement with generated clientOrderId
- Test Binance API accepts our ID format
- Test round-trip: Generate → Place → Retrieve

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Documentation updated (inline comments for format specification)
- [ ] PO acceptance received
- [ ] All 5 trading modes tested end-to-end
- [ ] Character limit validation in place
