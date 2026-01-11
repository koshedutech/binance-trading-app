# Story 7.4: Parse Client Order ID from Binance Responses
**Epic:** Epic 7 - Client Order ID & Trade Lifecycle Tracking
**Sprint:** Sprint 7
**Story Points:** 3
**Priority:** P0

## User Story
As a trading system, I want to parse clientOrderId from Binance API responses so that I can extract mode, date, sequence, and order type for display and analytics.

## Acceptance Criteria
- [ ] `ClientOrderIdParser` with `Parse(clientOrderId)` method implemented
- [ ] Returns structured data: mode, date, sequence, orderType, chainId
- [ ] Handles malformed IDs gracefully (returns nil, no error)
- [ ] Recognizes legacy/unstructured IDs and skips parsing
- [ ] Extracts chain base ID (without type suffix)
- [ ] Validates mode codes against known modes
- [ ] Validates date format (DDMMM)
- [ ] Integration with order/trade response processing

## Technical Approach

1. **Parser Implementation**:
   - Split clientOrderId by "-" delimiter
   - Validate 4+ parts: MODE-DDMMM-NNNNN-TYPE
   - Parse each component with validation
   - Return nil for non-matching formats (graceful)

2. **Parsed Data Structure**:
   ```go
   type ParsedOrderId struct {
       Mode      TradingMode  // ULT, SCA, SCR, SWI, POS
       Date      time.Time    // Parsed from DDMMM
       Sequence  int          // 00001-99999
       OrderType OrderType    // E, SL, TP1, H, etc.
       ChainId   string       // "ULT-06JAN-00001" (base)
       Raw       string       // Original full ID
   }
   ```

3. **Validation Logic**:
   - Mode: Must be one of 5 known codes (ULT, SCA, SCR, SWI, POS)
   - Date: Must parse as "02Jan" format (e.g., "06JAN")
   - Sequence: Must be numeric, 1-99999
   - Type: Must be recognized suffix (E, SL, TP1-TP4, H, HSL, HTP, DCA1-DCA3)

4. **Edge Cases**:
   - Empty string → nil
   - Legacy IDs (no dashes) → nil
   - Partial matches → nil
   - Future format changes → nil (safe forward compatibility)
   - Fallback IDs (`FALLBACK-{uuid}`) → special handling

5. **Integration Points**:
   - Parse Binance order responses
   - Parse Binance trade history
   - Parse position update events
   - Display in UI components

## Dependencies
- **Blocked By:**
  - Story 7.1: Client Order ID Generation (defines format)
  - Story 7.3: Order Chain Tracking (defines chain concept)
- **Blocks:**
  - Story 7.5: Trade Lifecycle Tab UI (needs parsed data)
  - Story 7.9: Backend API for Trade Lifecycle (uses parser)

## Files to Create/Modify

### Files to Create:
- `internal/orders/client_order_id_parser.go` - ParseClientOrderId function and ParsedOrderId struct
- `internal/orders/client_order_id_parser_test.go` - Comprehensive parsing tests

### Files to Modify:
- `internal/binance/futures_client.go` - Parse clientOrderId in order responses
- `internal/api/futures_handlers.go` - Return parsed data in API responses
- `internal/database/order_repository.go` - Store parsed metadata with orders

## Testing Requirements

### Unit Tests:
- Test valid ID parsing (all components extracted correctly)
- Test all 5 mode codes recognized
- Test all order type suffixes recognized
- Test chain base extraction (removes type suffix)
- Test malformed IDs return nil (no panic)
- Test empty string handling
- Test legacy IDs (no structure) return nil
- Test too few parts (missing components) return nil
- Test invalid mode codes return nil
- Test invalid date format return nil
- Test invalid sequence (non-numeric) return nil
- Test fallback ID format parsing

### Integration Tests:
- Test parsing real Binance order responses
- Test round-trip: Generate → Parse (matches original)
- Test parsing order history with mixed ID formats
- Test UI displays parsed data correctly

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Documentation updated (parser logic explained)
- [ ] PO acceptance received
- [ ] All edge cases tested (malformed, legacy, empty)
- [ ] No panics on invalid input
