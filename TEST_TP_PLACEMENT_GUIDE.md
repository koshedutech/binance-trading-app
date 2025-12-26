# Multi-Level TP Placement Test - Comprehensive Guide
**Date:** 2025-12-25
**Server Status:** ✅ RUNNING
**Build:** Latest with Enhanced Logging

---

## Quick Status Summary

### Server Health ✅
- **Status:** Online at http://localhost:8094
- **Ginie Autopilot:** Enabled & Running
- **Mode:** LIVE (not dry-run)
- **Can Trade:** YES
- **Active Positions:** 0 (ready)

### TP Configuration ✅
- **Mode:** Multi-TP (4-level allocation)
- **TP1:** 25% of position at calculated price
- **TP2:** 25% of position at calculated price
- **TP3:** 25% of position at calculated price
- **TP4:** 25% of position at calculated price (with trailing)
- **Status:** LOADED & READY

### Enhanced Logging ✅
- **TP Hit Detection:** Enhanced with detailed logs
- **TP2-4 Placement:** Enhanced with function call logs
- **Status:** DEPLOYED

---

## What We're Testing

When a Ginie position opens and TP levels are hit:

### Test Scenario Flow

```
1. Position Opens
   ↓
   → TP1 is placed on Binance (algo order)
   → TP2, TP3, TP4 are calculated but NOT placed yet

2. Price Reaches TP1
   ↓
   → checkTakeProfits() detects TP1 hit
   → 25% of position is closed with market order
   → placeNextTPOrder() is called to place TP2

3. TP2 Placement (NEW - Enhanced Logging)
   ↓
   → placeNextTPOrder() cancels other algo orders
   → Places TP2 as new algo order on Binance
   → LOGS: "Next take profit order placed" with algo_id

4. Price Reaches TP2
   ↓
   → 25% of remaining position is closed
   → TP3 is placed

5. Price Reaches TP3
   ↓
   → 25% of remaining position is closed
   → TP4 is placed OR Trailing stop activated

6. Price Reaches TP4 / Trailing Stop
   ↓
   → Final 25% of position is closed
   → Position Complete
```

---

## How to Monitor the Test

### Method 1: Real-Time Log Monitoring (Recommended)

**In PowerShell (Windows):**
```powershell
# Run the monitoring script
powershell -ExecutionPolicy Bypass -File monitor_tp_placement.ps1
```

**In Bash (Linux/Mac or Git Bash):**
```bash
./test_tp_placement.sh
```

**In Terminal (Any OS):**
```bash
tail -f server.log | grep -iE "TP level hit|placeNextTPOrder|Next take profit|Failed to place next"
```

### Method 2: API Polling

Check trading activity every few seconds:
```bash
# Get current diagnostics
curl -s http://localhost:8094/api/futures/ginie/diagnostics | \
  python3 -m json.tool | grep -A 10 "profit_booking"

# Look for these values changing:
# - tp_hits_last_hour (should increase)
# - partial_closes_last_hour (should increase)
# - positions_with_pending_tp (should be > 0 when TPs pending)
```

### Method 3: Web Browser

Visit: http://localhost:8094

Check:
1. **Dashboard** - See open positions
2. **Ginie Panel** - See active positions and TP status
3. **Trade History** - See completed trades with TP levels

---

## Expected Log Messages

### When a Position Opens

```json
{
  "timestamp": "2025-12-25T10:30:00Z",
  "level": "INFO",
  "message": "Created new Ginie position",
  "component": "futures_autopilot",
  "fields": {
    "symbol": "BTCUSDT",
    "side": "LONG",
    "mode": "swing",
    "entry_price": 26200.50,
    "quantity": 100,
    "tp_levels": 4,
    "sl_price": 25706.20
  }
}
```

### When TP1 is Hit ✅ KEY TEST POINT

```json
{
  "timestamp": "2025-12-25T10:35:00Z",
  "level": "INFO",
  "message": "TP level hit - placing next TP order",
  "fields": {
    "symbol": "BTCUSDT",
    "current_tp_level": 1,
    "next_tp_level": 2,
    "remaining_qty": 75,
    "next_tp_price": 26450.30
  }
}
```

### When placeNextTPOrder() is Called ✅ PROVES TP2 LOGIC WORKS

```json
{
  "timestamp": "2025-12-25T10:35:01Z",
  "level": "INFO",
  "message": "placeNextTPOrder called",
  "fields": {
    "symbol": "BTCUSDT",
    "current_tp_level": 1,
    "next_tp_level": 2,
    "next_tp_price": 26450.30,
    "dry_run": false
  }
}
```

### When TP2 is Successfully Placed ✅ CONFIRMS WORKING

```json
{
  "timestamp": "2025-12-25T10:35:02Z",
  "level": "INFO",
  "message": "Next take profit order placed",
  "fields": {
    "symbol": "BTCUSDT",
    "tp_level": 2,
    "algo_id": "123456789",
    "trigger_price": 26450.30,
    "quantity": 18.75
  }
}
```

### If TP2 Placement Fails ❌ DEBUGGING

```json
{
  "timestamp": "2025-12-25T10:35:02Z",
  "level": "ERROR",
  "message": "Failed to place next take profit order",
  "fields": {
    "symbol": "BTCUSDT",
    "tp_level": 2,
    "tp_price": 26450.30,
    "error": "error message from Binance API"
  }
}
```

---

## What Success Looks Like

### ✅ Successful Test Results

When you see this log sequence, TP placement is working:

```
[10:35:00] TP level hit - placing next TP order (TP1 hit)
[10:35:01] placeNextTPOrder called (TP2 function triggered)
[10:35:02] Next take profit order placed (TP2 on Binance)
           algo_id: 123456789, trigger_price: 26450.30
[10:40:00] TP level hit - placing next TP order (TP2 hit)
[10:40:01] placeNextTPOrder called (TP3 function triggered)
[10:40:02] Next take profit order placed (TP3 on Binance)
           algo_id: 987654321, trigger_price: 26700.50
[10:45:00] TP level hit - placing next TP order (TP3 hit)
[10:45:01] placeNextTPOrder called (TP4 function triggered)
[10:45:02] Next take profit order placed (TP4 on Binance)
           algo_id: 111222333, trigger_price: 26950.00
```

### ✅ Verification on Binance

When you log into Binance and check your Futures account:
1. **Initial:** 1 Algo Order (TP1)
2. **After TP1 hits:** 1 Algo Order (TP2) + Partial close filled
3. **After TP2 hits:** 1 Algo Order (TP3) + Partial close filled
4. **After TP3 hits:** 1 Algo Order (TP4) or Trailing SL + Partial close filled

---

## Troubleshooting

### Symptom 1: "TP level hit" appears but NOT "placeNextTPOrder called"

**Problem:** TP detection works but next TP placement function isn't being called
**Cause:** checkTakeProfits() isn't reaching the placeNextTPOrder() call
**Solution:**
1. Verify position still exists (wasn't closed prematurely)
2. Check if early profit booking feature closed the position
3. Verify TP index is in bounds (tpLevel < len(pos.TakeProfits))

### Symptom 2: "placeNextTPOrder called" appears but NOT "Next take profit order placed"

**Problem:** Function is called but order placement fails
**Check logs for:** "Failed to place next take profit order"
**Common causes:**
- Binance API rate limiting (place orders slower)
- Invalid quantity (rounding issue - very unlikely)
- API key permissions (check Binance settings)
- Network connectivity issue (retry logic may help)

### Symptom 3: Orders appear on Binance, but not all 4 levels

**Problem:** Some TP levels placed but not others
**Possible causes:**
- Price moved too fast (skipped levels)
- Trailing stop activated early (TP4 becomes trailing)
- Circuit breaker paused trading
- Daily max trades limit hit

### Symptom 4: Nothing appears in logs (no positions opened)

**Problem:** Ginie isn't opening positions
**Check:**
1. Ginie is enabled: `curl http://localhost:8094/api/futures/ginie/status`
2. Can trade: Check circuit breaker status
3. Confidence scores: Check if signals have high enough confidence (50%+ required)
4. Min confidence setting in logs: Should be 50 or lower

---

## Current System Status

**As of 2025-12-25 10:16 AM:**

```
Scanning Status:
- Symbols Scanned: 65/65 in last cycle
- Last Scan: 10 seconds ago
- All Modes: Enabled (scalp, swing, position)

Trading Activity (Last Hour):
- Signals Generated: 65
- Signals Executed: 20 (30.8% rate)
- Signals Rejected: 45
- Rejection Reasons: Mostly low confidence scores
- TP Hits: 0 (no open positions yet)
- Partial Closes: 0

Circuit Breaker Status:
- State: CLOSED (trading allowed)
- Hourly Loss: $0 / $100 limit
- Daily Loss: $0 / $500 limit
- Consecutive Losses: 0 / 15 max

Positions:
- Current Open: 0
- Max Allowed: 10
- Available Slots: 10
```

---

## Running the Full Test

### Step 1: Start Monitoring (Do this first!)

**Choose your monitoring method:**

Option A - PowerShell:
```powershell
powershell -ExecutionPolicy Bypass -File D:\Apps\binance-trading-bot\monitor_tp_placement.ps1
```

Option B - Terminal:
```bash
tail -f D:\Apps\binance-trading-bot\server.log | grep -iE "TP level hit|placeNextTPOrder|Next take profit|Failed to"
```

### Step 2: Wait for Trade

Leave monitoring running. Ginie will:
1. Scan 65 symbols continuously
2. Analyze for high-confidence signals
3. Open a position when conditions align
4. Estimated wait: 5-30 minutes (depends on market conditions)

### Step 3: Watch the Sequence

When a position opens:
- Look for "TP level hit" messages
- Count how many "Next take profit order placed" messages appear
- Verify you see messages for TP2, TP3, TP4

### Step 4: Verification

Verify on Binance:
1. Log into Binance Futures
2. Check "Orders" tab
3. See if algo orders appear as TP levels are hit
4. Confirm all 4 levels eventually appear (or trailing)

---

## Files Available

- **TP_MULTI_LEVEL_FIX_SUMMARY.md** - Detailed technical analysis of the issue and fix
- **TP_PLACEMENT_TEST_REPORT.md** - Current system status and what to expect
- **monitor_tp_placement.ps1** - PowerShell real-time monitor
- **test_tp_placement.sh** - Bash real-time monitor
- **server.log** - Full server logs (in real-time)

---

## Next Steps

1. ✅ Server is running
2. ✅ TP configuration is loaded
3. ✅ Enhanced logging is deployed
4. ⏳ **Start monitoring** (using provided scripts)
5. ⏳ **Wait for position to open** (watch for "Created new Ginie position")
6. ⏳ **Watch TP placement sequence** (look for the log messages above)
7. ⏳ **Verify on Binance** (check orders appear)

---

## Success Criteria

**The test is successful if:**
- ✅ Position opens and shows 4 TP levels calculated
- ✅ TP1 is placed on Binance initially
- ✅ When TP1 hits, you see "TP level hit - placing next TP order"
- ✅ When TP1 hits, you see "placeNextTPOrder called" with next_tp_level=2
- ✅ When TP1 hits, you see "Next take profit order placed" for TP2
- ✅ Repeat for TP2 → TP3 and TP3 → TP4
- ✅ All 4 levels appear on Binance orders

**You've proven the fix works!** 🎉
