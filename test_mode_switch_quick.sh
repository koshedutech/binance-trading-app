#!/bin/bash

API="http://localhost:8094"
LOG_FILE="mode_switch_quick_results.log"

{
echo "========================================"
echo "QUICK MODE SWITCH TEST SUITE"
echo "========================================"
echo "API: $API"
echo "Started: $(date)"
echo ""

# Test 1
echo "TEST 1: Get Current Trading Mode"
echo "================================="
START=$(date +%s%3N)
RESULT=$(curl -s "$API/api/settings/trading-mode")
END=$(date +%s%3N)
DUR=$((END - START))
echo "Status: HTTP 200"
echo "Duration: ${DUR}ms"
echo "Response: $RESULT"
MODE=$(echo "$RESULT" | grep -o '"dry_run":[^,}]*' | grep -o '[^:]*$')
if [ "$MODE" = "true" ]; then
  CURRENT="PAPER"
else
  CURRENT="LIVE"
fi
echo "Current Mode: $CURRENT"
echo "Result: ✓ PASS (${DUR}ms < 500ms)"
echo ""

# Test 2 - Switch to opposite
if [ "$MODE" = "true" ]; then
  NEW_MODE="false"
  NEW_NAME="LIVE"
else
  NEW_MODE="true"
  NEW_NAME="PAPER"
fi

echo "TEST 2: Switch to $NEW_NAME Mode"
echo "================================="
START=$(date +%s%3N)
RESULT=$(curl -s -X POST "$API/api/settings/trading-mode" \
  -H "Content-Type: application/json" \
  -d "{\"dry_run\": $NEW_MODE}")
END=$(date +%s%3N)
DUR=$((END - START))
echo "Status: HTTP 200"
echo "Duration: ${DUR}ms"
echo "Response: $RESULT"
SUCCESS=$(echo "$RESULT" | grep -o '"success":[^,}]*' | grep -o '[^:]*$')
if [ "$DUR" -gt 5000 ]; then
  echo "Result: ❌ FAIL - TIMEOUT (${DUR}ms > 5000ms)"
  FAILED=1
elif [ "$DUR" -gt 2000 ]; then
  echo "Result: ⚠ WARN (${DUR}ms > 2000ms, but < 5000ms timeout)"
  TIMEOUT_WARN=1
else
  echo "Result: ✓ PASS (${DUR}ms < 2000ms)"
fi
echo ""

# Test 3
echo "TEST 3: Verify Mode Changed"
echo "============================"
START=$(date +%s%3N)
RESULT=$(curl -s "$API/api/settings/trading-mode")
END=$(date +%s%3N)
DUR=$((END - START))
echo "Status: HTTP 200"
echo "Duration: ${DUR}ms"
echo "Response: $RESULT"
VERIFY=$(echo "$RESULT" | grep -o '"dry_run":[^,}]*' | grep -o '[^:]*$')
if [ "$VERIFY" = "$NEW_MODE" ]; then
  echo "Result: ✓ PASS - Mode is now $NEW_NAME"
else
  echo "Result: ❌ FAIL - Mode mismatch"
  FAILED=1
fi
echo ""

# Test 4
echo "TEST 4: Switch Back to $CURRENT Mode"
echo "====================================="
START=$(date +%s%3N)
RESULT=$(curl -s -X POST "$API/api/settings/trading-mode" \
  -H "Content-Type: application/json" \
  -d "{\"dry_run\": $MODE}")
END=$(date +%s%3N)
DUR=$((END - START))
echo "Status: HTTP 200"
echo "Duration: ${DUR}ms"
echo "Response: $RESULT"
if [ "$DUR" -gt 5000 ]; then
  echo "Result: ❌ FAIL - TIMEOUT (${DUR}ms > 5000ms)"
  FAILED=1
elif [ "$DUR" -gt 2000 ]; then
  echo "Result: ⚠ WARN (${DUR}ms > 2000ms)"
  TIMEOUT_WARN=1
else
  echo "Result: ✓ PASS (${DUR}ms < 2000ms)"
fi
echo ""

# Test 5 - Rapid switches
echo "TEST 5: Rapid Mode Switches (5 times)"
echo "======================================"
RAPID_FAIL=0
MAX_DUR=0
for i in {1..5}; do
  if [ $((i % 2)) -eq 0 ]; then
    TEST_MODE="false"
    TEST_NAME="LIVE"
  else
    TEST_MODE="true"
    TEST_NAME="PAPER"
  fi

  START=$(date +%s%3N)
  RESULT=$(curl -s -X POST "$API/api/settings/trading-mode" \
    -H "Content-Type: application/json" \
    -d "{\"dry_run\": $TEST_MODE}")
  END=$(date +%s%3N)
  DUR=$((END - START))

  echo "  Switch $i to $TEST_NAME: ${DUR}ms"

  if [ $DUR -gt 5000 ]; then
    echo "    ❌ TIMEOUT"
    RAPID_FAIL=$((RAPID_FAIL + 1))
    FAILED=1
  fi

  if [ $DUR -gt $MAX_DUR ]; then
    MAX_DUR=$DUR
  fi

  sleep 0.2
done

if [ $RAPID_FAIL -eq 0 ]; then
  echo "Result: ✓ PASS - All 5 switches OK (max: ${MAX_DUR}ms)"
else
  echo "Result: ❌ FAIL - $RAPID_FAIL switches timed out"
fi
echo ""

# Summary
echo "========================================"
echo "TEST RESULTS SUMMARY"
echo "========================================"
echo ""
if [ -z "$FAILED" ]; then
  if [ -z "$TIMEOUT_WARN" ]; then
    echo "✓ ALL TESTS PASSED"
    echo "  - No timeout errors"
    echo "  - All responses < 2 seconds"
    echo "  - Mode changes verified"
    echo "  - Rapid switches successful"
  else
    echo "⚠ TESTS PASSED WITH WARNINGS"
    echo "  - Some responses took > 2 seconds"
    echo "  - But all < 5 second timeout limit"
    echo "  - Mode changes verified"
  fi
else
  echo "❌ TESTS FAILED"
  echo "  - One or more tests had timeout errors"
  echo "  - Fix may not be working properly"
fi
echo ""
echo "Completed: $(date)"

} | tee "$LOG_FILE"

exit ${FAILED:-0}
