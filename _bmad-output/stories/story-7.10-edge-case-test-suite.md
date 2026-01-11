# Story 7.10: Edge Case Test Suite
**Epic:** Epic 7 - Client Order ID & Trade Lifecycle Tracking
**Sprint:** Sprint 7
**Story Points:** 5
**Priority:** P1

## User Story
As a quality assurance engineer, I want comprehensive test coverage for edge cases in the clientOrderId system so that the system behaves correctly under all conditions.

## Acceptance Criteria
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
- [ ] **Character Limit Test**: Verify all IDs ≤ 36 characters
- [ ] **Timezone DST Test**: Test Daylight Saving Time transitions

## Technical Approach

Create comprehensive test suite covering all edge cases:

1. **Test File Structure**:
   ```
   internal/orders/
   ├── client_order_id_test.go          # Unit tests
   ├── client_order_id_parser_test.go   # Parser tests
   ├── edge_cases_test.go               # Edge case tests
   integration_test/
   └── trade_lifecycle_test.go          # End-to-end tests
   ```

2. **Test Categories**:
   - **Format Validation**: Character limits, structure
   - **Timezone Edge Cases**: Midnight, DST, year boundary
   - **Concurrency**: Parallel requests, race conditions
   - **Failure Scenarios**: Redis down, network errors
   - **Binance Integration**: API acceptance, round-trip
   - **Parsing Edge Cases**: Malformed, legacy, empty

3. **Mock/Stub Strategy**:
   - Mock Redis for failure simulation
   - Mock Binance API for acceptance tests
   - Time travel for midnight/DST tests
   - Concurrent goroutines for race tests

4. **Test Data**:
   - Valid IDs: All mode and type combinations
   - Invalid IDs: Too short, too long, wrong format
   - Legacy IDs: Unstructured formats
   - Fallback IDs: FALLBACK format variants
   - Edge timestamps: Midnight, DST, year boundary

## Dependencies
- **Blocked By:**
  - Story 7.1: Client Order ID Generation (implementation to test)
  - Story 7.2: Daily Sequence Storage (sequence logic)
  - Story 7.4: Parse Client Order ID (parser logic)
  - Story 7.8: Redis Fallback (fallback logic)
- **Blocks:**
  - None (testing story, no blocking dependencies)

## Files to Create/Modify

### Files to Create:
- `internal/orders/edge_cases_test.go` - Comprehensive edge case test suite
- `integration_test/trade_lifecycle_test.go` - End-to-end integration tests
- `test/fixtures/order_ids.json` - Test data fixtures
- `test/mocks/redis_mock.go` - Mock Redis for failure simulation
- `test/mocks/binance_mock.go` - Mock Binance API client

### Files to Modify:
- `internal/orders/client_order_id_test.go` - Add edge case tests
- `internal/orders/client_order_id_parser_test.go` - Add malformed ID tests

## Testing Requirements

### Edge Case Tests (Detailed Implementation):

**1. Midnight Rollover Test**:
```go
func TestMidnightRollover(t *testing.T) {
    loc, _ := time.LoadLocation("Asia/Kolkata")
    generator := NewClientOrderIdGenerator(cache, loc)

    // 11:59 PM on Jan 6
    time1 := time.Date(2026, 1, 6, 23, 59, 0, 0, loc)
    id1, _ := generator.GenerateAtTime(userID, ModeUltraFast, OrderTypeEntry, time1)
    assert.Contains(t, id1, "06JAN-00001")

    // 12:01 AM on Jan 7
    time2 := time.Date(2026, 1, 7, 0, 1, 0, 0, loc)
    id2, _ := generator.GenerateAtTime(userID, ModeScalp, OrderTypeEntry, time2)
    assert.Contains(t, id2, "07JAN-00001") // Sequence reset to 1
}
```

**2. Redis Failure Handling**:
```go
func TestRedisFallback(t *testing.T) {
    // Simulate Redis down
    generator.cache = &BrokenRedis{}

    id, err := generator.Generate(userID, ModeUltraFast, OrderTypeEntry)

    assert.NoError(t, err) // Should not error
    assert.Contains(t, id, "FALLBACK")
    assert.Regexp(t, `ULT-FALLBACK-[a-f0-9]{8}-E`, id)
}
```

**3. Binance Acceptance Test**:
```go
func TestBinanceAcceptance(t *testing.T) {
    testCases := []struct {
        name string
        id   string
    }{
        {"Normal ID", "ULT-06JAN-00001-E"},
        {"Fallback ID", "ULT-FALLBACK-a3f7c2e9-E"},
        {"Long Type", "POS-06JAN-00042-DCA3"},
        {"Hedge", "SCA-06JAN-00015-HSL"},
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            order, err := client.PlaceTestOrder(tc.id)
            assert.NoError(t, err)
            assert.Equal(t, tc.id, order.ClientOrderID)
        })
    }
}
```

**4. Malformed ID Parsing**:
```go
func TestMalformedParsing(t *testing.T) {
    testCases := []struct {
        name string
        id   string
        want *ParsedOrderId
    }{
        {"Valid ID", "ULT-06JAN-00001-E", &ParsedOrderId{Mode: ModeUltraFast}},
        {"Legacy ID", "myorder123", nil},
        {"Empty", "", nil},
        {"Too few parts", "ULT-06JAN", nil},
        {"Invalid mode", "XXX-06JAN-00001-E", nil},
        {"Invalid date", "ULT-99ABC-00001-E", nil},
        {"Invalid sequence", "ULT-06JAN-XXXXX-E", nil},
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
```

**5. Year Boundary Test**:
```go
func TestYearBoundary(t *testing.T) {
    loc, _ := time.LoadLocation("Asia/Kolkata")

    // Dec 31, 2026
    time1 := time.Date(2026, 12, 31, 23, 59, 0, 0, loc)
    id1, _ := generator.GenerateAtTime(userID, ModeSwing, OrderTypeEntry, time1)
    assert.Contains(t, id1, "31DEC")

    // Jan 1, 2027
    time2 := time.Date(2027, 1, 1, 0, 1, 0, 0, loc)
    id2, _ := generator.GenerateAtTime(userID, ModeSwing, OrderTypeEntry, time2)
    assert.Contains(t, id2, "01JAN-00001") // New year, sequence reset
}
```

**6. Concurrent Sequence Test**:
```go
func TestConcurrentSequence(t *testing.T) {
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
```

**7. Maximum Sequence Test**:
```go
func TestMaxSequence(t *testing.T) {
    // Set sequence to 99999
    cache.SetDailySequence(userID, time.Now(), 99999)

    // Next increment
    id, err := generator.Generate(userID, ModePosition, OrderTypeEntry)

    assert.NoError(t, err)
    // Should either wrap to 1, use 6 digits, or generate fallback
    // Implementation decision documented in result
}
```

**8. Fallback Chain Grouping**:
```go
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
```

**9. All Modes Test**:
```go
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
        assert.LessOrEqual(t, len(id), 36) // Character limit
    }
}
```

**10. All Order Types Test**:
```go
func TestAllOrderTypes(t *testing.T) {
    types := []OrderType{
        OrderTypeEntry, OrderTypeStopLoss,
        OrderTypeTakeProfit1, OrderTypeTakeProfit2,
        OrderTypeTakeProfit3, OrderTypeTakeProfit4,
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

**11. Character Limit Test**:
```go
func TestCharacterLimit(t *testing.T) {
    // Test longest possible combinations
    longCombos := []struct {
        mode      TradingMode
        orderType OrderType
    }{
        {ModePosition, OrderTypeDCA3},    // POS-06JAN-00001-DCA3
        {ModeScalpReentry, OrderTypeHedgeSL}, // SCR-06JAN-00001-HSL
    }

    for _, combo := range longCombos {
        id, _ := generator.Generate(userID, combo.mode, combo.orderType)
        assert.LessOrEqual(t, len(id), 36, "ID exceeds Binance limit: %s", id)
    }
}
```

**12. Timezone DST Test**:
```go
func TestTimezoneDST(t *testing.T) {
    // Test with timezone that observes DST (not Asia/Kolkata)
    loc, _ := time.LoadLocation("America/New_York")
    generator := NewClientOrderIdGenerator(cache, loc)

    // Spring forward: 2:00 AM → 3:00 AM
    before := time.Date(2026, 3, 8, 1, 59, 0, 0, loc)
    after := time.Date(2026, 3, 8, 3, 1, 0, 0, loc)

    id1, _ := generator.GenerateAtTime(userID, ModeUltraFast, OrderTypeEntry, before)
    id2, _ := generator.GenerateAtTime(userID, ModeUltraFast, OrderTypeEntry, after)

    // Both should have same date (08MAR)
    assert.Contains(t, id1, "08MAR")
    assert.Contains(t, id2, "08MAR")
}
```

**13. Full Lifecycle Integration Test**:
```go
func TestFullLifecycle(t *testing.T) {
    // 1. Generate ID
    chainBase := generator.Generate(userID, ModeUltraFast, OrderTypeEntry)

    // 2. Place order on Binance
    order, err := client.PlaceOrder(ctx, PlaceOrderRequest{
        Symbol:        "BTCUSDT",
        Side:          "BUY",
        Type:          "MARKET",
        Quantity:      0.001,
        ClientOrderID: chainBase,
    })
    assert.NoError(t, err)

    // 3. Retrieve order
    retrieved, err := client.GetOrder(ctx, "BTCUSDT", order.OrderID)
    assert.NoError(t, err)
    assert.Equal(t, chainBase, retrieved.ClientOrderID)

    // 4. Parse retrieved ID
    parsed := ParseClientOrderId(retrieved.ClientOrderID)
    assert.NotNil(t, parsed)
    assert.Equal(t, ModeUltraFast, parsed.Mode)

    // 5. Group in UI
    chains := groupOrdersByChain([]*Order{retrieved})
    assert.Equal(t, 1, len(chains))
}
```

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] All edge case tests implemented and passing
- [ ] Integration tests passing (>95% coverage)
- [ ] Documentation updated (edge cases documented)
- [ ] PO acceptance received
- [ ] Midnight rollover tested
- [ ] Redis failure tested
- [ ] Binance acceptance tested
- [ ] Malformed ID parsing tested
- [ ] Year boundary tested
- [ ] Concurrent sequence tested
- [ ] Maximum sequence tested
- [ ] Fallback chain grouping tested
- [ ] All 5 modes tested
- [ ] All 13 order types tested
- [ ] Character limit verified
- [ ] DST transition tested
- [ ] No panics or crashes under any condition
