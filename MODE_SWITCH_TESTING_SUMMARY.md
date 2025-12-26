# Mode Switch Fixes - Testing Summary

## 🎯 What Was Fixed

Three critical issues preventing paper mode lock and futures timeout have been resolved:

### Fix #1: Force Mode Persistence ✓
**File:** `main.go:1387-1416`
- Removed early return that skipped consistency checks
- Always persists settings even when mode unchanged
- Ensures DryRunMode, GinieDryRunMode, SpotDryRunMode stay synchronized

### Fix #2: Auto-Stop Ginie Before Mode Switch ✓
**File:** `handlers_settings.go:99-124`
- Automatically stops Ginie if running before mode change
- Prevents lock contention during client switching
- 500ms cleanup wait with debug logging

### Fix #3: Timeout Protection for Client Switching ✓
**File:** `main.go:1428-1501`
- 5-second timeout context prevents infinite hangs
- Client switch runs in background goroutine
- Gracefully continues with settings persistence even if timeout

---

## 📋 Test Execution Checklist

### Prerequisites
- [ ] Application compiled successfully with `go build`
- [ ] Server running on `http://localhost:8088`
- [ ] Database accessible and migrations applied
- [ ] Application logs visible (watch for `[MODE-SWITCH]` prefix)

### Manual Test Suite

#### Test 1: Get Current Mode (Baseline)
```bash
curl http://localhost:8088/api/settings/trading-mode
```
- [ ] HTTP 200 response
- [ ] Contains `dry_run` field (boolean)
- [ ] Response time < 500ms
- [ ] No timeout warnings

#### Test 2: Switch Paper → Live
```bash
curl -X POST http://localhost:8088/api/settings/trading-mode \
  -H "Content-Type: application/json" \
  -d '{"dry_run": false}'
```
- [ ] HTTP 200 response
- [ ] Returns `"mode": "live"`
- [ ] Response time < 2 seconds
- [ ] **NO timeout errors**
- [ ] Logs show `[MODE-SWITCH]` messages

#### Test 3: Verify Mode Changed
```bash
curl http://localhost:8088/api/settings/trading-mode
```
- [ ] Returns `"dry_run": false`
- [ ] Confirms change was applied

#### Test 4: Switch Live → Paper
```bash
curl -X POST http://localhost:8088/api/settings/trading-mode \
  -H "Content-Type: application/json" \
  -d '{"dry_run": true}'
```
- [ ] HTTP 200 response
- [ ] Returns `"mode": "paper"`
- [ ] Response time < 2 seconds
- [ ] No timeout errors

#### Test 5: Mode Switch with Ginie Running (Critical Test)
```bash
# Start Ginie
curl -X POST http://localhost:8088/api/futures/ginie/autopilot/start

# Verify it's running
curl http://localhost:8088/api/futures/ginie/autopilot/status

# Switch mode while Ginie is running
curl -X POST http://localhost:8088/api/settings/trading-mode \
  -H "Content-Type: application/json" \
  -d '{"dry_run": false}'
```
- [ ] Ginie successfully stopped (auto)
- [ ] Mode switch completes
- [ ] Response time < 3 seconds
- [ ] No timeout errors (< 5s)
- [ ] Logs show Ginie stop sequence

**Expected log output:**
```
[MODE-SWITCH] Ginie autopilot is running, stopping it before mode switch...
[MODE-SWITCH] Ginie autopilot stopped successfully, waiting for cleanup...
[MODE-SWITCH] Cleanup complete, proceeding with mode switch
```

#### Test 6: Rapid Mode Switches (Stress Test)
- [ ] Perform 5 successive mode switches
- [ ] Each completes in < 2 seconds
- [ ] No errors or timeouts
- [ ] All switches succeed

#### Test 7: Mode Persistence After Restart
- [ ] Set mode to LIVE: `curl -X POST ... -d '{"dry_run": false}'`
- [ ] Restart server (Ctrl+C, then run again)
- [ ] Check mode: `curl http://localhost:8088/api/settings/trading-mode`
- [ ] Mode is still LIVE (not reverted)
- [ ] Settings file was persisted correctly

---

## 🔍 What to Look for in Logs

### ✅ Success Indicators (You SHOULD see these)

```
[MODE-SWITCH] Starting trading mode switch to dry_run=false
[MODE-SWITCH] Ginie autopilot is running, stopping it before mode switch...
[MODE-SWITCH] Ginie autopilot stopped successfully, waiting for cleanup...
[MODE-SWITCH] Cleanup complete, proceeding with mode switch
[MODE-SWITCH] Trading mode switch completed successfully

"Updated FuturesController dry_run", "dry_run": false
"Set futures controller client", "mode": "LIVE", "client_type": "real"
"Successfully saved trading mode to settings file"
```

### ❌ Failure Indicators (You should NOT see these)

```
"Futures client switch TIMEOUT - exceeded 5 seconds"
"Failed to update trading mode"
"panic"
"deadlock"
"Mode inconsistency detected"
"timeout waiting for lock"
"i/o timeout"
```

---

## 📊 Performance Benchmarks

### Expected Response Times

| Operation | Expected | Maximum |
|-----------|----------|---------|
| Get mode | 100-300ms | 500ms |
| Switch mode (no Ginie) | 300-800ms | 2000ms |
| Switch mode (Ginie running) | 800-1500ms | 3000ms |
| Single rapid switch | 500-2000ms | 2000ms |
| **Timeout threshold** | - | **5000ms** |

---

## 🚀 Quick Start Testing

### Option 1: Manual Testing (Recommended for First Run)

1. Start the bot:
```bash
cd D:\Apps\binance-trading-bot
./binance-bot
```

2. Open another terminal and run tests:
```bash
# Test 1: Get current mode
curl http://localhost:8088/api/settings/trading-mode | jq

# Test 2: Switch to live
curl -X POST http://localhost:8088/api/settings/trading-mode \
  -H "Content-Type: application/json" \
  -d '{"dry_run": false}' | jq

# Test 3: Verify change
curl http://localhost:8088/api/settings/trading-mode | jq '.dry_run'

# Test 4: Switch back to paper
curl -X POST http://localhost:8088/api/settings/trading-mode \
  -H "Content-Type: application/json" \
  -d '{"dry_run": true}' | jq
```

### Option 2: Automated Testing

**For PowerShell (Windows):**
```powershell
cd "D:\Apps\binance-trading-bot"
.\test_mode_switch.ps1
```

**For Bash/Shell (Linux/Mac):**
```bash
cd D:\Apps\binance-trading-bot
bash test_mode_switch.sh
```

---

## 📝 Test Report Template

Use this template to document your test results:

```markdown
# Mode Switch Testing Report

**Date:** [TODAY]
**Commit:** 390d775
**Tested By:** [YOUR_NAME]

## Test Results

| Test | Status | Duration | Notes |
|------|--------|----------|-------|
| Get Mode | ✓ PASS | XXXms | - |
| Paper → Live | ✓ PASS | XXXms | No timeout |
| Verify Change | ✓ PASS | XXXms | - |
| Live → Paper | ✓ PASS | XXXms | No timeout |
| With Ginie | ✓ PASS | XXXms | Auto-stopped |
| Rapid Switches | ✓ PASS | XXXms | 5/5 success |
| Persistence | ✓ PASS | - | Survived restart |

## Performance Summary

- Average mode switch time: XXXms
- Maximum time observed: XXXms
- Timeout occurrences: 0
- Error count: 0

## Log Analysis

- [MODE-SWITCH] messages: ✓ Present
- Panic errors: ✓ None
- Deadlock errors: ✓ None
- Consistency warnings: ✓ None

## Conclusion

✓ ALL TESTS PASSED - Mode switching is working correctly without timeout issues.
```

---

## 🐛 Troubleshooting

### Issue: "Server not responding on port 8088"

**Check:**
1. Is the bot process running?
   ```bash
   ps aux | grep binance-bot
   ```

2. What port is it actually on?
   ```bash
   netstat -an | grep LISTEN | grep bot
   ```

3. Check application startup logs for port binding errors

### Issue: Timeout errors in mode switch

**This indicates the fix isn't applied correctly:**
- Verify commit 390d775 is checked out: `git log --oneline -1`
- Rebuild the project: `go build`
- Restart the server

### Issue: Mode reverts to PAPER after restart

**This indicates settings not persisting:**
1. Check `autopilot_settings.json` exists and is writable
2. Verify `settings.DryRunMode` field is being saved
3. Check disk space and file permissions

### Issue: Ginie won't stop before mode switch

**Check logs for:**
```
[MODE-SWITCH] Failed to stop Ginie before mode switch
```

- Verify Ginie is actually running
- Check if Ginie's Stop() method is working
- Look for goroutine leaks in logs

---

## 🎯 Success Criteria

### All Tests Pass When:

- ✓ Every mode switch completes in < 2 seconds
- ✓ No timeout errors in any test
- ✓ Mode changes persist after restart
- ✓ Ginie auto-stops before mode switch
- ✓ Rapid switches (5x) all succeed
- ✓ Logs show clear [MODE-SWITCH] progress
- ✓ No panic, deadlock, or consistency errors

---

## 📞 Support

If tests fail:

1. **Check detailed logs** - Look for `[MODE-SWITCH]` prefix
2. **Verify database connectivity** - Mode changes need to persist to disk
3. **Check file permissions** - Settings file must be writable
4. **Review error messages** - Timeouts vs. other errors need different solutions

---

## ✅ Final Verification Checklist

Before considering the fix complete:

- [ ] Test 1-7 all pass
- [ ] No timeout errors observed
- [ ] [MODE-SWITCH] logs visible and informative
- [ ] Mode persists correctly after restart
- [ ] Ginie auto-stops during mode switch
- [ ] Performance meets benchmarks
- [ ] No panic or deadlock errors
- [ ] Can switch modes rapidly without issues

---

**Status:** Ready for Testing ✓

All fixes have been committed (390d775). Tests can now be executed.
