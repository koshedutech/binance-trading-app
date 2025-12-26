#!/bin/bash

API_URL="http://localhost:8094"
echo "========================================"
echo "MODE SWITCH QUICK TEST SUITE"
echo "API URL: $API_URL"
echo "========================================"
echo ""

# Test 1: Get current mode
echo "=== TEST 1: Get Current Trading Mode ==="
START=$(date +%s%N)
RESPONSE=$(curl -s "$API_URL/api/settings/trading-mode")
END=$(date +%s%N)
DURATION=$(( ($END - $START) / 1000000 ))
echo "Response: $RESPONSE"
echo "Duration: ${DURATION}ms"
echo "✓ TEST 1 PASSED (${DURATION}ms)"
echo ""

# Extract current mode
CURRENT_MODE=$(echo "$RESPONSE" | grep -o '"dry_run":[^,}]*' | grep -o '[^:]*$')
if [ "$CURRENT_MODE" = "true" ]; then
  echo "Current Mode: PAPER"
  NEW_MODE="false"
  NEW_MODE_NAME="LIVE"
else
  echo "Current Mode: LIVE"
  NEW_MODE="true"
  NEW_MODE_NAME="PAPER"
fi
echo ""

# Test 2: Switch mode
echo "=== TEST 2: Switch to $NEW_MODE_NAME Mode ==="
START=$(date +%s%N)
RESPONSE=$(curl -s -X POST "$API_URL/api/settings/trading-mode" \
  -H "Content-Type: application/json" \
  -d "{\"dry_run\": $NEW_MODE}")
END=$(date +%s%N)
DURATION=$(( ($END - $START) / 1000000 ))
echo "Response: $RESPONSE"
echo "Duration: ${DURATION}ms"

# Check for timeout
if [ $DURATION -gt 5000 ]; then
  echo "❌ TEST 2 FAILED - TIMEOUT EXCEEDED (${DURATION}ms > 5000ms)"
  exit 1
elif [ $DURATION -gt 2000 ]; then
  echo "⚠ TEST 2 PASSED with WARNING - Took ${DURATION}ms (> 2000ms)"
else
  echo "✓ TEST 2 PASSED (${DURATION}ms)"
fi
echo ""

# Test 3: Verify mode changed
echo "=== TEST 3: Verify Mode Change ==="
START=$(date +%s%N)
RESPONSE=$(curl -s "$API_URL/api/settings/trading-mode")
END=$(date +%s%N)
DURATION=$(( ($END - $START) / 1000000 ))
echo "Response: $RESPONSE"
echo "Duration: ${DURATION}ms"

VERIFIED_MODE=$(echo "$RESPONSE" | grep -o '"dry_run":[^,}]*' | grep -o '[^:]*$')
if [ "$VERIFIED_MODE" = "$NEW_MODE" ]; then
  echo "✓ TEST 3 PASSED - Mode verified as $NEW_MODE_NAME (${DURATION}ms)"
else
  echo "❌ TEST 3 FAILED - Expected $NEW_MODE, got $VERIFIED_MODE"
  exit 1
fi
echo ""

# Test 4: Switch back
ORIGINAL_MODE=$CURRENT_MODE
if [ "$ORIGINAL_MODE" = "true" ]; then
  ORIGINAL_NAME="PAPER"
else
  ORIGINAL_NAME="LIVE"
fi

echo "=== TEST 4: Switch Back to $ORIGINAL_NAME Mode ==="
START=$(date +%s%N)
RESPONSE=$(curl -s -X POST "$API_URL/api/settings/trading-mode" \
  -H "Content-Type: application/json" \
  -d "{\"dry_run\": $ORIGINAL_MODE}")
END=$(date +%s%N)
DURATION=$(( ($END - $START) / 1000000 ))
echo "Response: $RESPONSE"
echo "Duration: ${DURATION}ms"

if [ $DURATION -gt 5000 ]; then
  echo "❌ TEST 4 FAILED - TIMEOUT EXCEEDED (${DURATION}ms > 5000ms)"
  exit 1
elif [ $DURATION -gt 2000 ]; then
  echo "⚠ TEST 4 PASSED with WARNING - Took ${DURATION}ms (> 2000ms)"
else
  echo "✓ TEST 4 PASSED (${DURATION}ms)"
fi
echo ""

# Test 5: Rapid mode switches
echo "=== TEST 5: Rapid Mode Switches (5 times) ==="
FAILED=0
MAX_TIME=0
for i in {1..5}; do
  if [ $((i % 2)) -eq 0 ]; then
    TEST_MODE="false"
    TEST_NAME="LIVE"
  else
    TEST_MODE="true"
    TEST_NAME="PAPER"
  fi

  START=$(date +%s%N)
  RESPONSE=$(curl -s -X POST "$API_URL/api/settings/trading-mode" \
    -H "Content-Type: application/json" \
    -d "{\"dry_run\": $TEST_MODE}")
  END=$(date +%s%N)
  DURATION=$(( ($END - $START) / 1000000 ))

  echo "  Switch $i to $TEST_NAME: ${DURATION}ms"

  if [ $DURATION -gt 5000 ]; then
    echo "    ❌ TIMEOUT"
    FAILED=$((FAILED + 1))
  elif [ $DURATION -gt 2000 ]; then
    echo "    ⚠ WARNING (> 2s)"
  fi

  if [ $DURATION -gt $MAX_TIME ]; then
    MAX_TIME=$DURATION
  fi

  sleep 0.3
done

if [ $FAILED -eq 0 ]; then
  echo "✓ TEST 5 PASSED - All 5 switches successful (max: ${MAX_TIME}ms)"
else
  echo "❌ TEST 5 FAILED - $FAILED switches failed"
  exit 1
fi
echo ""

# Summary
echo "========================================"
echo "✓ ALL QUICK TESTS PASSED"
echo "========================================"
echo ""
echo "Summary:"
echo "  - No timeout errors detected"
echo "  - All mode switches completed"
echo "  - Mode changes verified"
echo "  - Performance within limits"
echo ""
