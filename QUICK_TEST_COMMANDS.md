# Quick Mode Switch Test Commands

Copy-paste these commands to test the mode switch fixes.

## Prerequisites
```bash
# 1. Build the application
cd D:\Apps\binance-trading-bot
go build -o binance-bot

# 2. Start the server
./binance-bot

# 3. In another terminal, verify it's running
curl http://localhost:8088/health
```

---

## Test Commands (Copy & Paste)

### Test 1: Get Current Mode
```bash
curl -s http://localhost:8088/api/settings/trading-mode | jq
```

### Test 2: Switch to LIVE Mode (Paper → Live)
```bash
curl -w "\n✓ Time: %{time_total}s\n" -X POST http://localhost:8088/api/settings/trading-mode \
  -H "Content-Type: application/json" \
  -d '{"dry_run": false}' | jq '.success, .mode, .message'
```

### Test 3: Verify Mode Changed
```bash
curl -s http://localhost:8088/api/settings/trading-mode | jq '.dry_run, .mode'
```

### Test 4: Switch Back to PAPER (Live → Paper)
```bash
curl -w "\n✓ Time: %{time_total}s\n" -X POST http://localhost:8088/api/settings/trading-mode \
  -H "Content-Type: application/json" \
  -d '{"dry_run": true}' | jq '.success, .mode, .message'
```

### Test 5: Mode Switch With Ginie Running (CRITICAL TEST)

**5a. Start Ginie:**
```bash
curl -s -X POST http://localhost:8088/api/futures/ginie/autopilot/start \
  -H "Content-Type: application/json" | jq '.success, .message'
```

**5b. Verify Ginie is running:**
```bash
curl -s http://localhost:8088/api/futures/ginie/autopilot/status | jq '.running'
```

**5c. Switch mode while Ginie is running (should auto-stop Ginie):**
```bash
curl -w "\n✓ Time: %{time_total}s\n" -X POST http://localhost:8088/api/settings/trading-mode \
  -H "Content-Type: application/json" \
  -d '{"dry_run": false}' | jq '.success, .mode, .message'
```

**5d. Verify Ginie was stopped:**
```bash
curl -s http://localhost:8088/api/futures/ginie/autopilot/status | jq '.running'
```

### Test 6: Rapid Mode Switches (Stress Test)

**Run this script (5 successive switches):**
```bash
#!/bin/bash
echo "Starting rapid mode switch test..."
for i in {1..5}; do
  if [ $((i % 2)) -eq 0 ]; then
    MODE='false'
    NAME='LIVE'
  else
    MODE='true'
    NAME='PAPER'
  fi

  echo ""
  echo "=== Switch $i to $NAME ==="
  START=$(date +%s%N)
  RESPONSE=$(curl -s -X POST http://localhost:8088/api/settings/trading-mode \
    -H "Content-Type: application/json" \
    -d "{\"dry_run\": $MODE}")
  END=$(date +%s%N)
  DURATION=$(( ($END - $START) / 1000000 ))

  echo "$RESPONSE" | jq '.mode, .message'
  echo "Duration: ${DURATION}ms"

  if [ $DURATION -gt 2000 ]; then
    echo "⚠ WARNING: Exceeded 2 second limit!"
  else
    echo "✓ Within timeout"
  fi

  sleep 0.5
done
```

### Test 7: Mode Persistence After Restart

**7a. Set mode to LIVE:**
```bash
curl -s -X POST http://localhost:8088/api/settings/trading-mode \
  -H "Content-Type: application/json" \
  -d '{"dry_run": false}' | jq '.mode'
```

**7b. Stop the server:**
```
Press Ctrl+C in the terminal running the bot
Wait 2 seconds for clean shutdown
```

**7c. Restart the server:**
```bash
./binance-bot
```

**7d. Verify mode persisted:**
```bash
curl -s http://localhost:8088/api/settings/trading-mode | jq '.dry_run, .mode'
```

Expected: Should still be `false` (LIVE), not `true` (PAPER)

---

## Expected Results Summary

### ✓ You Should See:

1. **All mode switches respond with HTTP 200**
2. **All responses complete in < 2 seconds** (< 5s at most)
3. **No "timeout" errors in responses**
4. **Logs show `[MODE-SWITCH]` debug messages:**
   ```
   [MODE-SWITCH] Starting trading mode switch to dry_run=false
   [MODE-SWITCH] Trading mode switch completed successfully
   ```
5. **When Ginie running, logs show auto-stop:**
   ```
   [MODE-SWITCH] Ginie autopilot is running, stopping it before mode switch...
   [MODE-SWITCH] Ginie autopilot stopped successfully
   ```
6. **Mode persists after restart** (same value before and after)

### ❌ You Should NOT See:

```
"Futures client switch TIMEOUT"
"Failed to update trading mode"
"panic"
"deadlock"
"timeout waiting for lock"
"Mode inconsistency detected"
```

---

## Timing Expectations

```
GET mode:              100-500ms
POST mode switch:      300-1500ms (depends on Ginie state)
Rapid switch (5x):     each < 2000ms
TIMEOUT PROTECTION:    5000ms hard limit
```

---

## Log Inspection

### To see the [MODE-SWITCH] logs in action:

1. Keep the bot running terminal visible
2. Execute the test commands
3. Watch for lines starting with `[MODE-SWITCH]`

### To save logs to file:

```bash
# Start bot and redirect logs
./binance-bot > bot.log 2>&1

# In another terminal, watch logs in real-time
tail -f bot.log | grep MODE-SWITCH
```

---

## Example Successful Test Output

```bash
$ curl -s http://localhost:8088/api/settings/trading-mode | jq
{
  "dry_run": true,
  "mode": "paper",
  "mode_label": "Paper Trading",
  "can_switch": true
}

$ curl -w "Time: %{time_total}s\n" -X POST http://localhost:8088/api/settings/trading-mode \
  -H "Content-Type: application/json" \
  -d '{"dry_run": false}' | jq
{
  "success": true,
  "dry_run": false,
  "mode": "live",
  "mode_label": "Live Trading",
  "can_switch": true,
  "message": "Trading mode updated successfully"
}
Time: 0.523s

$ curl -s http://localhost:8088/api/settings/trading-mode | jq '.mode'
"live"
```

✓ All tests passed! Mode switching is working correctly.

---

## If Tests Fail

### Check these in order:

1. **Server running?**
   ```bash
   curl http://localhost:8088/health
   ```

2. **Check application logs for errors:**
   ```
   Look for lines with:
   - ERROR
   - PANIC
   - timeout
   - deadlock
   ```

3. **Verify the fixes are applied:**
   ```bash
   git log --oneline -1
   # Should show: 390d775 fix: Resolve Ginie paper mode lock...
   ```

4. **Rebuild if needed:**
   ```bash
   go build -o binance-bot
   ```

---

## Success = ✓

When all tests pass:
- ✓ No timeout errors
- ✓ Mode changes complete < 2s
- ✓ [MODE-SWITCH] logs visible
- ✓ Mode persists after restart
- ✓ Ginie auto-stops before switch
- ✓ Rapid switches work without issues

**Then the fixes are working correctly!**
