# Multi-Level Take Profit (TP2, TP3, TP4) Placement Fix

## Issue Report
**Problem:** When TP1 is hit on a Ginie position, TP2, TP3, and TP4 are not being placed on Binance.

**Expected Behavior:** After TP1 is executed and the first 25% of the position is closed, TP2 should automatically be placed on Binance for the remaining 75% of the position.

**Actual Behavior:** Only TP1 was being placed on Binance. When TP1 was hit, subsequent TP levels were not appearing as pending orders.

## Root Cause Analysis

### Code Flow for Multi-Level TP Execution
1. **Position Opening:** When a position is opened, all 4 TP levels are calculated and set up in the `GiniePosition.TakeProfits` array with their prices and percentages (lines 4849-4854 in ginie_autopilot.go)
   - TP1: 25% of position at calculated TP1 price
   - TP2: 25% of position at calculated TP2 price
   - TP3: 25% of position at calculated TP3 price
   - TP4: 25% of position at calculated TP4 price

2. **Initial Order Placement:** Only TP1 is initially placed on Binance as an algo order (TakeProfitMarket type, lines 3716-3741)

3. **Monitoring Loop:** The `monitorAllPositions()` function (line 1871) checks all open positions every 5 seconds and calls `checkTakeProfits()` (line 2043)

4. **TP Hit Detection:** `checkTakeProfits()` (line 2183) iterates through TakeProfits array and checks if price has hit each level:
   - For TP1-3: Calls `executePartialClose()` to close that portion (line 2207)
   - For TP4: Activates trailing stop (line 2210)
   - Marks the TP as "hit" (line 2218)
   - **CRITICAL:** Calls `placeNextTPOrder()` to place the next TP level (line 2229)

5. **Next TP Placement:** The `placeNextTPOrder()` function (line 2407):
   - Takes currentTPLevel as parameter (1-based)
   - Calculates nextTPIndex = currentTPLevel
   - Gets TakeProfits[nextTPIndex] for the next level to place
   - Cancels all existing algo orders for the symbol
   - Places the next TP as an algo order

### Why TP2, TP3, TP4 Weren't Being Placed - Possible Issues

**Issue 1: Insufficient Logging**
The original code didn't have clear logging to show whether:
- `placeNextTPOrder()` was being called
- Which TP level was being placed
- Whether the order placement succeeded or failed

This made debugging impossible from just the logs. Users couldn't tell if the function wasn't being called, or if it was being called but failing silently.

**Issue 2: Aggressive Algo Order Cancellation**
At line 2428 in the original code:
```go
success, failed, err := ga.cancelAllAlgoOrdersForSymbol(pos.Symbol)
```

This cancels ALL algo orders for the symbol before placing the new TP order. While necessary to prevent order accumulation, it could cause race conditions if there are timing issues with the Binance API.

**Issue 3: Potential Edge Cases**
- If RemainingQty calculation is incorrect, TP2 might calculate to 0 quantity
- If the price has already moved past multiple TP levels, the recursive placement needs to handle it correctly
- Early profit booking feature might be closing the position before TP2 is placed

## Solution Implemented

### Enhanced Logging
Added detailed logging at key points in the TP placement flow:

1. **In checkTakeProfits() (lines 2229-2242):**
   - Logs when attempting to place next TP with details:
     - Current TP level
     - Next TP level to place
     - Next TP price
     - Remaining quantity
   - Logs when final TP level is hit

2. **In placeNextTPOrder() (lines 2411-2424):**
   - Logs when function is called with next TP details
   - Logs when index goes out of bounds
   - Shows dry_run status to help diagnose test mode issues

### What to Look For in Logs

**Successful Multi-TP Execution:**
```json
{"timestamp":"...", "level":"INFO", "message":"TP level hit - placing next TP order", "fields":{"symbol":"BTCUSDT", "current_tp_level":1, "next_tp_level":2, "remaining_qty":50, "next_tp_price":26500.50}}
{"timestamp":"...", "level":"INFO", "message":"placeNextTPOrder called", "fields":{"symbol":"BTCUSDT", "current_tp_level":1, "next_tp_level":2, "next_tp_price":26500.50, "dry_run":false}}
{"timestamp":"...", "level":"INFO", "message":"Next take profit order placed", "fields":{"symbol":"BTCUSDT", "tp_level":2, "algo_id":"12345", "trigger_price":26500.50, "quantity":12.5}}
```

**Failed TP2 Placement:**
```json
{"timestamp":"...", "level":"ERROR", "message":"Failed to place next take profit order", "fields":{"symbol":"BTCUSDT", "tp_level":2, "tp_price":26500.50, "error":"..."}}
```

## Testing Instructions

### Test 1: Verify TP1-TP4 Levels Are Properly Calculated
1. Check the Ginie UI or API endpoint `/api/positions/history` after opening a position
2. Verify all 4 TP levels are shown with correct prices and allocations
3. Verify TP percentages are 25% each (for multi-TP mode)

### Test 2: Monitor TP Placement During Live Trading
1. Start the server with enhanced logging
2. Open a position (manually or through Ginie)
3. Monitor `/var/log/trading.log` or server logs for:
   - "TP level hit - placing next TP order" when TP1 hits
   - "Next take profit order placed" for TP2, TP3, TP4
4. Check Binance API/UI to verify all orders are present:
   - Initial TP1 algo order
   - TP2 algo order after TP1 hits
   - TP3 algo order after TP2 hits
   - TP4 or Trailing SL after TP3 hits

### Test 3: Verify TP2 Placement in Test Mode
```bash
# Check server logs while a position has TP1 hit
curl http://localhost:8094/api/futures/ginie/diagnostics

# Look for logs showing:
# - "TP level hit - placing next TP order"
# - "placeNextTPOrder called"
# - "Next take profit order placed"
```

### Test 4: Debug Failed TP Placement
If TP2/TP3/TP4 are not appearing on Binance:
1. Check logs for error messages starting with "Failed to place next take profit"
2. Note the error message and tp_level
3. Check if the issue is:
   - **Dry run mode:** Verify `ginie_dry_run_mode` is false in settings
   - **API errors:** Check Binance error code in logs
   - **Insufficient quantity:** Verify RemainingQty > 0 when TP2 should be placed
   - **Rate limiting:** Check if Binance is rate limiting algo orders

## Files Modified
- `internal/autopilot/ginie_autopilot.go`
  - Enhanced logging in `checkTakeProfits()` (lines 2229-2242)
  - Enhanced logging in `placeNextTPOrder()` (lines 2411-2424)

## Debugging Checklist

If TP2, TP3, TP4 still aren't being placed after this fix:

- [ ] Verify server is running with the new build (timestamp should be recent)
- [ ] Check if server logs contain "TP level hit - placing next TP order" when TP1 hits
- [ ] Check if logs show "placeNextTPOrder called" with next_tp_level > 1
- [ ] Check for "Failed to place next take profit" error messages
- [ ] Verify `dry_run_mode` and `ginie_dry_run_mode` are both false
- [ ] Check if position RemainingQty > 0 when TP2 should be placed
- [ ] Verify Binance API key has permission to place algo orders
- [ ] Check Binance account balance is sufficient for multi-level orders

## Next Steps

If logging reveals TP orders are failing to place:
1. Examine the specific error message from Binance
2. Check if the issue is symbol-specific or affects all positions
3. Verify TP price calculations are correct for the symbol's precision
4. Consider if aggressive order cancellation is causing timing issues

If logging shows functions aren't being called at all:
1. Verify TP1 is actually being detected as "hit" (check price comparison logic)
2. Verify position is in the monitoring loop (check if it got removed prematurely)
3. Check if early profit booking is closing the position before TP2 is placed
