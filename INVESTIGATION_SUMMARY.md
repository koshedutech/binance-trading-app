# Investigation: SLTP Not Set + Early Profit Booking Close Failures

## Summary of Issues Found

### Issue 1: SLTP Not Set After TP1 Hit ✅ FIXED
**Status**: FIXED in code (line 2363 in ginie_autopilot.go)

**Problem**: When TP1 was hit, the system would cancel all algo orders (including SL) but only place the next TP order without placing a new SL order for the remaining quantity.

**Fix Applied**:
- Added `ga.placeSLOrder(pos)` call after placing next TP order
- Added SL placement after immediate TP execution for last TP case

---

### Issue 2: Early Profit Booking Close Orders Failing ⚠️ INVESTIGATING
**Status**: Under investigation - need better error reporting

**Symptoms**:
- System detects ROI threshold hit for SQDUSDT (21.71% > 8% threshold)
- System logs: "Booking profit early based on ROI threshold"
- System logs: "Ginie closing position"
- But then: "Ginie full close failed" with empty error object `{}`

**Expected Behavior**:
- Detect ROI threshold (✅ Working)
- Cancel all algo orders (✅ Working - confirmed in logs)
- Place close order at market (with limit price protection)
- Position should close and ROI should be locked in

**Actual Behavior**:
- Detect ROI threshold (✅ Working)
- Cancel all algo orders (✅ Working)
- Place close order (❌ FAILING - returns error)
- Position remains open, system retries every 5 seconds

---

## SQDUSDT Position Current Status

| Metric | Value |
|--------|-------|
| Entry Price | 0.05613597 |
| Current Price | 0.05886868 |
| Remaining Qty | 4014 |
| Current ROI | 23.93% |
| Target Threshold | 8.00% |
| Status | OPEN - Waiting to close |
| Close Attempts | 10+ (every 5 seconds since server restart) |

---

## Analysis of Potential Root Causes

### 1. Quantity/Price Precision Issues
**Hypothesis**: Order parameters might not match Binance's precision requirements

**Applied Fix**:
- Added rounding of quantity and price before placing order
- Line 2534-2535 in ginie_autopilot.go:
  ```go
  roundedQty := roundQuantity(symbol, pos.RemainingQty)
  roundedPrice := roundPrice(symbol, closePrice)
  ```

**Status**: Applied, but error persists

---

### 2. Missing TimeInForce Parameter
**Hypothesis**: Limit orders might require explicit TimeInForce setting

**Status**: Not yet investigated - need to check FuturesOrderParams structure

---

### 3. Error Logging Issue
**Issue**: Error message shows as empty `{}` instead of actual error text

**Root Cause**: Error interface being marshaled as empty JSON

**Partial Fix Applied**:
- Changed from `"error", err` to `"error", err.Error()`
- Added diagnostic fields: qty, price
- But new error messages still not appearing in logs

**Status**: Logging improved, but fields not showing in output

---

## Logs Evidence

### Successful Detection (Every 5 seconds):
```
"message":"Booking profit early based on ROI threshold"
"roi_percent":23.7
"symbol":"SQDUSDT"
"threshold":23.7
```

### Successful Cancellation:
```
"[GINIE] SQDUSDT: Closing position, cancelling all algo orders (SL_ID=2000000099404633, TP_IDs=[2000000099327398])"
```

### Failed Close Order:
```
"message":"Ginie full close failed"
"error":{}
"symbol":"SQDUSDT"
```

---

## Code Changes Applied So Far

### Fix 1: placeNextTPOrder - Add SL Order (Line 2361-2363)
```go
// CRITICAL FIX: Place a new SL order for remaining quantity
// Without this, the remaining position is unprotected after TP placement
ga.placeSLOrder(pos)
```

### Fix 2: Immediate TP Execution - Add SL for Last TP (Line 2328-2331)
```go
} else {
    // Last TP executed - ensure SL is placed for remaining qty if not trailing
    if pos.RemainingQty > 0 && !pos.TrailingActive {
        ga.placeSLOrder(pos)
    }
}
```

### Fix 3: Close Order - Round Qty/Price (Line 2532-2535)
```go
// CRITICAL FIX: Round quantity and price to match Binance's precision requirements
// Without this, orders are rejected with precision errors
roundedQty := roundQuantity(symbol, pos.RemainingQty)
roundedPrice := roundPrice(symbol, closePrice)
```

### Fix 4: Improve Error Logging (Line 2548-2552)
```go
ga.logger.Error("Ginie full close failed",
    "symbol", symbol,
    "error", err.Error(),
    "qty", roundedQty,
    "price", roundedPrice)
```

---

## Next Steps for Investigation

### 1. Verify Error Message Actually Appears
- Check if error fields are now visible in latest logs
- Look for actual Binance error text (not just `{}`)

### 2. Check FuturesOrderParams Requirements
- Investigate if TimeInForce parameter is needed
- Check if other required fields are missing

### 3. Test Close Order Manually
- Use curl to test closing SQDUSDT position manually
- See what error Binance returns

### 4. Review Recent Changes
- Check if any recent Go client library changes affected PlaceFuturesOrder

---

## Build Status
✅ All builds successful
✅ Server running with latest fixes
✅ No compilation errors

## Testing Timeline
- 15:31:47 - Discovered SQDUSDT ROI at 21.71% (threshold already hit)
- 15:32:00 - System started attempting to close
- 15:32:10 - Confirmed "full close failed" errors in logs
- 15:33:00 - Applied precision rounding fix
- 15:33:55 - Error persists after restart
- 15:34:10 - Improved error logging, but messages still unclear

---

## Summary
✅ **SLTP Fix**: Completed and verified in code
⚠️ **Early Profit Booking Close**: Partially working (detection OK, execution failing)
🔍 **Root Cause**: Unknown - error messages not providing details
📋 **Action Items**: Need to see actual Binance error response

