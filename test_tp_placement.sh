#!/bin/bash

# TP Placement Test Monitor Script
# Monitors server logs for multi-level TP placement activity
# Usage: ./test_tp_placement.sh

LOG_FILE="./server.log"
LAST_LINE=0

echo "========================================"
echo "TP Placement Activity Monitor"
echo "========================================"
echo "Log file: $LOG_FILE"
echo ""
echo "This script will highlight TP-related events:"
echo "  [OPENED]  - New position opened"
echo "  [TP-HIT]  - TP level was hit"
echo "  [FUNC]    - placeNextTPOrder called"
echo "  [SUCCESS] - TP order placed on Binance"
echo "  [ERROR]   - TP order failed"
echo ""
echo "Press Ctrl+C to stop"
echo ""

while true; do
    if [ -f "$LOG_FILE" ]; then
        # Count lines in file
        CURRENT_LINES=$(wc -l < "$LOG_FILE")

        # If new lines exist
        if [ "$CURRENT_LINES" -gt "$LAST_LINE" ]; then
            # Extract new lines
            tail -n +$((LAST_LINE + 1)) "$LOG_FILE" | tail -n +1 | while IFS= read -r line; do
                # Color code different types of messages
                if echo "$line" | grep -q "TP level hit - placing next TP order"; then
                    echo -e "\033[33m[TP-HIT]\033[0m $line"
                elif echo "$line" | grep -q "placeNextTPOrder called"; then
                    echo -e "\033[36m[FUNC]\033[0m $line"
                elif echo "$line" | grep -q "Next take profit order placed"; then
                    echo -e "\033[32m[SUCCESS]\033[0m $line"
                elif echo "$line" | grep -q "Failed to place next take profit"; then
                    echo -e "\033[31m[ERROR]\033[0m $line"
                elif echo "$line" | grep -q "Created new Ginie position\|Ginie position opened"; then
                    echo -e "\033[32m[OPENED]\033[0m $line"
                elif echo "$line" | grep -qi "ginie partial close"; then
                    echo -e "\033[35m[PARTIAL-CLOSE]\033[0m $line"
                fi
            done

            LAST_LINE="$CURRENT_LINES"
        fi
    fi

    sleep 1
done
