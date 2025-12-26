# Mode Switch Testing Guide

## Overview
This guide provides comprehensive testing steps to verify that the mode switch fixes resolve the timeout and paper mode persistence issues.

## Prerequisites
- Application built and running
- API server accessible at `http://localhost:8088`
- Ability to monitor application logs for `[MODE-SWITCH]` debug messages

## Test Environment Setup

### Step 1: Start the Application
```bash
cd D:\Apps\binance-trading-bot
./binance-bot
```

The server should start and display logs. Look for:
- API listening on port 8088
- No error messages during initialization

### Step 2: Verify API is Running
```bash
curl http://localhost:8088/health
```

Expected response: `{"status":"ok"}` or similar health check response

---

## Test Scenarios

### Test 1: Get Current Trading Mode (Baseline)

**Purpose:** Establish baseline mode before any changes

**Steps:**
```bash
curl -s http://localhost:8088/api/settings/trading-mode | jq
```

**Expected Output:**
```json
{
  "dry_run": true,
  "mode": "paper",
  "mode_label": "Paper Trading",
  "can_switch": true
}
```

**Success Criteria:**
- ✓ HTTP 200 response
- ✓ `dry_run` field is boolean (true for PAPER, false for LIVE)
- ✓ No timeout errors
- ✓ Response time < 500ms

**Logs to Check:**
- No error messages
- No timeout warnings

---

### Test 2: Switch from PAPER to LIVE Mode

**Purpose:** Test mode switch without Ginie running - baseline scenario

**Steps:**
```bash
# Time the request
curl -w "\nTime: %{time_total}s\n" -X POST http://localhost:8088/api/settings/trading-mode \
  -H "Content-Type: application/json" \
  -d "{\"dry_run\": false}" | jq
```

**Expected Output:**
```json
{
  "success": true,
  "dry_run": false,
  "mode": "live",
  "mode_label": "Live Trading",
  "can_switch": true,
  "message": "Trading mode updated successfully"
}

Time: 0.523s
```

**Success Criteria:**
- ✓ HTTP 200 response
- ✓ `success`: true
- ✓ `mode`: "live"
- ✓ Response time < 2 seconds (well under 5s timeout)
- ✓ No timeout errors

**Logs to Check:**
```
[MODE-SWITCH] Starting trading mode switch to dry_run=false
[MODE-SWITCH] Trading mode switch completed successfully
"Updated FuturesController dry_run", "dry_run": false
"Successfully saved trading mode to settings file"
```

**⚠️ Critical Check:**
- Should NOT see: "Futures client switch TIMEOUT"
- Should NOT see: "Failed to update trading mode"
- Should NOT see: "panic"

---

### Test 3: Verify Mode Change Persisted

**Purpose:** Confirm mode change was actually applied

**Steps:**
```bash
curl -s http://localhost:8088/api/settings/trading-mode | jq '.dry_run'
```

**Expected Output:**
```
false
```

**Success Criteria:**
- ✓ Returns `false` (LIVE mode)
- ✓ Matches the previous mode switch request

**Logs to Check:**
- "Futures controller client updated"
- "Set futures controller client", "mode": "LIVE"

---

### Test 4: Switch Back to PAPER Mode

**Purpose:** Test reverse mode switch

**Steps:**
```bash
curl -w "\nTime: %{time_total}s\n" -X POST http://localhost:8088/api/settings/trading-mode \
  -H "Content-Type: application/json" \
  -d "{\"dry_run\": true}" | jq
```

**Expected Output:**
```json
{
  "success": true,
  "dry_run": true,
  "mode": "paper",
  "mode_label": "Paper Trading",
  "can_switch": true,
  "message": "Trading mode updated successfully"
}

Time: 0.487s
```

**Success Criteria:**
- ✓ HTTP 200 response
- ✓ Response time < 2 seconds
- ✓ No timeout errors
- ✓ `mode`: "paper"

---

### Test 5: Mode Switch with Ginie Running (Advanced)

**Purpose:** Test the critical fix - mode switch should auto-stop Ginie to prevent lock contention

**Steps:**

1. First, verify Ginie can start:
```bash
curl -X POST http://localhost:8088/api/futures/ginie/autopilot/start \
  -H "Content-Type: application/json" | jq
```

2. Verify Ginie is running:
```bash
curl -s http://localhost:8088/api/futures/ginie/autopilot/status | jq '.running'
```
Expected: `true`

3. Now attempt mode switch while Ginie is running:
```bash
curl -w "\nTime: %{time_total}s\n" -X POST http://localhost:8088/api/settings/trading-mode \
  -H "Content-Type: application/json" \
  -d "{\"dry_run\": false}" | jq
```

**Expected Output:**
```json
{
  "success": true,
  "dry_run": false,
  "mode": "live",
  ...
}

Time: 1.234s
```

**Success Criteria:**
- ✓ HTTP 200 response
- ✓ Response time < 3 seconds (may be slightly longer due to Ginie stop)
- ✓ No timeout errors (< 5 seconds)
- ✓ Mode change succeeds even with Ginie running

**Logs to Check:**
```
[MODE-SWITCH] Ginie autopilot is running, stopping it before mode switch...
[MODE-SWITCH] Ginie autopilot stopped successfully, waiting for cleanup...
[MODE-SWITCH] Cleanup complete, proceeding with mode switch
[MODE-SWITCH] Starting trading mode switch to dry_run=false
[MODE-SWITCH] Trading mode switch completed successfully
```

4. Verify Ginie was stopped:
```bash
curl -s http://localhost:8088/api/futures/ginie/autopilot/status | jq '.running'
```
Expected: `false`

---

### Test 6: Rapid Mode Switches (Stress Test)

**Purpose:** Test the timeout protection under rapid successive switches

**Steps:**
```bash
#!/bin/bash
for i in {1..5}; do
  echo "=== Switch $i ==="
  if [ $((i % 2)) -eq 0 ]; then
    MODE="false"
  else
    MODE="true"
  fi

  START=$(date +%s%N)
  curl -s -X POST http://localhost:8088/api/settings/trading-mode \
    -H "Content-Type: application/json" \
    -d "{\"dry_run\": $MODE}" | jq '.mode, .message'
  END=$(date +%s%N)
  DURATION=$(( ($END - $START) / 1000000 ))
  echo "Duration: ${DURATION}ms"
  echo ""

  sleep 0.5
done
```

**Expected Output:**
```
=== Switch 1 ===
"paper"
"Trading mode updated successfully"
Duration: 523ms

=== Switch 2 ===
"live"
"Trading mode updated successfully"
Duration: 487ms

... (all should complete in < 2s)
```

**Success Criteria:**
- ✓ All 5 switches complete without errors
- ✓ Each switch response time < 2 seconds
- ✓ No timeout errors across all switches
- ✓ Final mode matches last requested mode

**Logs to Check:**
- Should see 5 sets of `[MODE-SWITCH]` messages
- Should see 5 successful client updates
- No panics or errors

---

### Test 7: Mode Persistence After Server Restart

**Purpose:** Verify Fix #1 - Mode persists correctly after restart

**Steps:**

1. Set mode to LIVE:
```bash
curl -X POST http://localhost:8088/api/settings/trading-mode \
  -H "Content-Type: application/json" \
  -d "{\"dry_run\": false}" | jq '.mode'
```
Expected: `"live"`

2. Stop the server:
```bash
# Press Ctrl+C in the terminal running the bot
```

3. Wait 2 seconds for clean shutdown

4. Restart the server:
```bash
./binance-bot
```

5. Check the mode:
```bash
curl -s http://localhost:8088/api/settings/trading-mode | jq '.dry_run'
```

**Expected Output:**
```
false
```

**Success Criteria:**
- ✓ Mode is still `false` (LIVE) after restart
- ✓ Did NOT revert to default (paper)
- ✓ Settings were properly persisted

**Logs to Check:**
- During startup: Mode should be loaded as LIVE
- No inconsistency warnings like:
  - "Mode inconsistency detected"
  - "expected dry_run=X, got Y"

---

## Performance Benchmarks

### Expected Response Times (Post-Fix)

| Scenario | Expected Time | Max Allowed |
|----------|---------------|------------|
| Get current mode | 100-300ms | 500ms |
| Switch mode (no Ginie) | 300-800ms | 2000ms |
| Switch mode (Ginie running) | 800-1500ms | 3000ms |
| Rapid switch (5x) | 500-2000ms each | 2000ms each |
| **Timeout threshold** | N/A | **5000ms** |

---

## Error Scenarios (What NOT to See)

### ❌ Do NOT See These Errors

1. **Timeout Error:**
   ```
   "error": "context deadline exceeded"
   "Futures client switch TIMEOUT - exceeded 5 seconds"
   ```

2. **Lock/Deadlock Errors:**
   ```
   "timeout waiting for lock"
   "deadlock detected"
   "i/o timeout"
   ```

3. **Paper Mode Reversion:**
   ```
   Settings verification FAILED
   Mode inconsistency detected
   expected dry_run=false, got true
   ```

4. **Panic Errors:**
   ```
   "panic during futures client switch"
   "panic during client selection"
   ```

---

## Success Criteria Summary

### ✓ All Tests Pass When:

1. **Test 1-4:** Each mode switch completes in < 2 seconds
2. **Test 5:** Mode switch works with Ginie running (auto-stops)
3. **Test 6:** All 5 rapid switches succeed with < 2s each
4. **Test 7:** Mode persists correctly after server restart
5. **All Tests:** No timeout errors in any response
6. **All Tests:** `[MODE-SWITCH]` logs visible and informative
7. **Logs:** No panic, deadlock, or consistency warnings

---

## Troubleshooting

### Symptom: "Server not responding" on Port 8088

**Solution:**
- Check if bot is running: `ps aux | grep binance-bot`
- Check actual port with: `netstat -an | grep LISTEN`
- Edit `test_mode_switch.ps1` to use correct port

### Symptom: Timeout Errors in Mode Switch

**What was fixed:**
- Added 5-second timeout context
- Ginie auto-stops before mode switch
- Settings persist even if timeout occurs

**Action:** Check logs for `[MODE-SWITCH]` messages to diagnose

### Symptom: Mode Reverts to Paper After Restart

**Indicates:** Settings not persisting correctly
- Check `autopilot_settings.json` file exists and is writable
- Verify `settings.DryRunMode` field is being saved
- Check disk space

---

## Reporting Test Results

### Template for Test Report:

```
MODE SWITCH FIX VERIFICATION REPORT
====================================

Date: [DATE]
Commit: 390d775
Build: [VERSION]

Test Results:
- Test 1 (Get Mode): ✓ PASS (XXXms)
- Test 2 (Switch P→L): ✓ PASS (XXXms)
- Test 3 (Verify Change): ✓ PASS (XXXms)
- Test 4 (Switch L→P): ✓ PASS (XXXms)
- Test 5 (With Ginie): ✓ PASS (XXXms)
- Test 6 (Rapid Switch): ✓ PASS (All 5 < 2s)
- Test 7 (Persistence): ✓ PASS (Survived restart)

Performance:
- Average mode switch time: XXXms
- Max timeout observed: XXXms
- No timeout errors: ✓ YES

Logs:
- [MODE-SWITCH] messages visible: ✓ YES
- No panic errors: ✓ YES
- No deadlock errors: ✓ YES

Conclusion: ✓ ALL TESTS PASSED - Mode switch fixes working correctly
```

---

## Next Steps

If all tests pass:
1. ✓ Fixes are working correctly
2. ✓ Mode switching is robust
3. ✓ Ginie can safely auto-stop during mode changes
4. ✓ Settings persist across restarts

If any test fails:
1. Check the detailed logs
2. Look for `[MODE-SWITCH]` debug messages
3. Verify database/settings file permissions
4. Check that Ginie cleanup completes properly
