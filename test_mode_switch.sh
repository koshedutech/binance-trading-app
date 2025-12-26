#!/bin/bash

# Mode Switch Testing Script
# Tests mode switching without timeout errors

API_URL="http://localhost:8088"
LOG_FILE="mode_switch_test.log"

echo "========================================" | tee -a $LOG_FILE
echo "Mode Switch Testing Script" | tee -a $LOG_FILE
echo "API URL: $API_URL" | tee -a $LOG_FILE
echo "========================================" | tee -a $LOG_FILE
echo "" | tee -a $LOG_FILE

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test 1: Get current trading mode
echo -e "${YELLOW}[TEST 1] Getting current trading mode...${NC}" | tee -a $LOG_FILE
START_TIME=$(date +%s%N)
RESPONSE=$(curl -s -w "\n%{http_code}" "$API_URL/api/settings/trading-mode")
END_TIME=$(date +%s%N)
DURATION=$((($END_TIME - $START_TIME) / 1000000))

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

echo "HTTP Status: $HTTP_CODE" | tee -a $LOG_FILE
echo "Duration: ${DURATION}ms" | tee -a $LOG_FILE
echo "Response: $BODY" | tee -a $LOG_FILE

if [ "$HTTP_CODE" != "200" ]; then
    echo -e "${RED}✗ Failed to get current mode${NC}" | tee -a $LOG_FILE
    exit 1
fi

CURRENT_MODE=$(echo "$BODY" | grep -o '"dry_run":[^,}]*' | grep -o '[^:]*$')
echo -e "${GREEN}✓ Current mode: $CURRENT_MODE${NC}" | tee -a $LOG_FILE
echo "" | tee -a $LOG_FILE

# Test 2: Switch to opposite mode (Paper to Live if currently Paper)
if [ "$CURRENT_MODE" = "true" ]; then
    NEW_MODE="false"
    MODE_NAME="LIVE"
else
    NEW_MODE="true"
    MODE_NAME="PAPER"
fi

echo -e "${YELLOW}[TEST 2] Switching to $MODE_NAME mode...${NC}" | tee -a $LOG_FILE
START_TIME=$(date +%s%N)
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/settings/trading-mode" \
    -H "Content-Type: application/json" \
    -d "{\"dry_run\": $NEW_MODE}")
END_TIME=$(date +%s%N)
DURATION=$((($END_TIME - $START_TIME) / 1000000))

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

echo "HTTP Status: $HTTP_CODE" | tee -a $LOG_FILE
echo "Duration: ${DURATION}ms" | tee -a $LOG_FILE
echo "Response: $BODY" | tee -a $LOG_FILE

if [ "$HTTP_CODE" != "200" ]; then
    echo -e "${RED}✗ Failed to switch mode${NC}" | tee -a $LOG_FILE
    exit 1
fi

echo -e "${GREEN}✓ Mode switch completed in ${DURATION}ms${NC}" | tee -a $LOG_FILE

# Check if timeout occurred (duration > 5 seconds)
if [ $DURATION -gt 5000 ]; then
    echo -e "${RED}⚠ WARNING: Mode switch took longer than expected (${DURATION}ms > 5000ms)${NC}" | tee -a $LOG_FILE
else
    echo -e "${GREEN}✓ Mode switch completed within timeout limit (${DURATION}ms < 5000ms)${NC}" | tee -a $LOG_FILE
fi
echo "" | tee -a $LOG_FILE

# Test 3: Verify mode was applied
echo -e "${YELLOW}[TEST 3] Verifying mode change was applied...${NC}" | tee -a $LOG_FILE
START_TIME=$(date +%s%N)
RESPONSE=$(curl -s -w "\n%{http_code}" "$API_URL/api/settings/trading-mode")
END_TIME=$(date +%s%N)
DURATION=$((($END_TIME - $START_TIME) / 1000000))

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

echo "HTTP Status: $HTTP_CODE" | tee -a $LOG_FILE
echo "Duration: ${DURATION}ms" | tee -a $LOG_FILE
echo "Response: $BODY" | tee -a $LOG_FILE

VERIFIED_MODE=$(echo "$BODY" | grep -o '"dry_run":[^,}]*' | grep -o '[^:]*$')

if [ "$VERIFIED_MODE" = "$NEW_MODE" ]; then
    echo -e "${GREEN}✓ Mode change verified: $MODE_NAME${NC}" | tee -a $LOG_FILE
else
    echo -e "${RED}✗ Mode change NOT applied! Expected: $NEW_MODE, Got: $VERIFIED_MODE${NC}" | tee -a $LOG_FILE
    exit 1
fi
echo "" | tee -a $LOG_FILE

# Test 4: Switch back to original mode
ORIGINAL_MODE=$CURRENT_MODE
if [ "$ORIGINAL_MODE" = "true" ]; then
    ORIG_MODE_NAME="PAPER"
else
    ORIG_MODE_NAME="LIVE"
fi

echo -e "${YELLOW}[TEST 4] Switching back to $ORIG_MODE_NAME mode...${NC}" | tee -a $LOG_FILE
START_TIME=$(date +%s%N)
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/settings/trading-mode" \
    -H "Content-Type: application/json" \
    -d "{\"dry_run\": $ORIGINAL_MODE}")
END_TIME=$(date +%s%N)
DURATION=$((($END_TIME - $START_TIME) / 1000000))

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

echo "HTTP Status: $HTTP_CODE" | tee -a $LOG_FILE
echo "Duration: ${DURATION}ms" | tee -a $LOG_FILE
echo "Response: $BODY" | tee -a $LOG_FILE

if [ "$HTTP_CODE" != "200" ]; then
    echo -e "${RED}✗ Failed to switch back to $ORIG_MODE_NAME mode${NC}" | tee -a $LOG_FILE
    exit 1
fi

echo -e "${GREEN}✓ Switch back completed in ${DURATION}ms${NC}" | tee -a $LOG_FILE

if [ $DURATION -gt 5000 ]; then
    echo -e "${RED}⚠ WARNING: Mode switch took longer than expected (${DURATION}ms > 5000ms)${NC}" | tee -a $LOG_FILE
else
    echo -e "${GREEN}✓ Mode switch completed within timeout limit${NC}" | tee -a $LOG_FILE
fi
echo "" | tee -a $LOG_FILE

# Test 5: Rapid mode switches (stress test)
echo -e "${YELLOW}[TEST 5] Stress test - Rapid mode switches (5 times)...${NC}" | tee -a $LOG_FILE
MAX_DURATION=0
MIN_DURATION=999999
TOTAL_DURATION=0
FAILED_COUNT=0

for i in {1..5}; do
    # Toggle mode
    if [ $((i % 2)) -eq 0 ]; then
        TEST_MODE="false"
        TEST_NAME="LIVE"
    else
        TEST_MODE="true"
        TEST_NAME="PAPER"
    fi

    START_TIME=$(date +%s%N)
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/settings/trading-mode" \
        -H "Content-Type: application/json" \
        -d "{\"dry_run\": $TEST_MODE}")
    END_TIME=$(date +%s%N)
    DURATION=$((($END_TIME - $START_TIME) / 1000000))

    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    BODY=$(echo "$RESPONSE" | head -n-1)

    echo "  Switch $i to $TEST_NAME: ${DURATION}ms - HTTP $HTTP_CODE" | tee -a $LOG_FILE

    if [ "$HTTP_CODE" != "200" ]; then
        FAILED_COUNT=$((FAILED_COUNT + 1))
        echo -e "${RED}    ✗ Failed${NC}" | tee -a $LOG_FILE
    else
        if [ $DURATION -gt $MAX_DURATION ]; then
            MAX_DURATION=$DURATION
        fi
        if [ $DURATION -lt $MIN_DURATION ]; then
            MIN_DURATION=$DURATION
        fi
        TOTAL_DURATION=$((TOTAL_DURATION + DURATION))
    fi

    # Small delay between switches
    sleep 0.5
done

SUCCESS_COUNT=$((5 - FAILED_COUNT))
AVG_DURATION=$((TOTAL_DURATION / 5))

echo "  Success: $SUCCESS_COUNT/5" | tee -a $LOG_FILE
echo "  Average Duration: ${AVG_DURATION}ms" | tee -a $LOG_FILE
echo "  Min/Max Duration: ${MIN_DURATION}ms / ${MAX_DURATION}ms" | tee -a $LOG_FILE

if [ $FAILED_COUNT -eq 0 ]; then
    echo -e "${GREEN}✓ All rapid switches successful${NC}" | tee -a $LOG_FILE
else
    echo -e "${RED}✗ $FAILED_COUNT switches failed${NC}" | tee -a $LOG_FILE
fi
echo "" | tee -a $LOG_FILE

# Summary
echo -e "${YELLOW}========================================${NC}" | tee -a $LOG_FILE
echo -e "${GREEN}MODE SWITCH TESTING COMPLETE${NC}" | tee -a $LOG_FILE
echo -e "${YELLOW}========================================${NC}" | tee -a $LOG_FILE
echo "Log saved to: $LOG_FILE" | tee -a $LOG_FILE
