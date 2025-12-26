#!/bin/bash

# SQDUSDT Live Monitoring Script
# Monitors SQDUSDT position ROI in real-time until it reaches 8.0% threshold

THRESHOLD=8.0
FEE_RATE=0.0004
CHECK_INTERVAL=15

echo "=================================================="
echo "SQDUSDT LIVE ROI MONITORING"
echo "Start Time: $(date '+%Y-%m-%d %H:%M:%S')"
echo "Check Interval: $CHECK_INTERVAL seconds"
echo "Target Threshold: $THRESHOLD%"
echo "=================================================="
echo ""

LAST_ROI=0
START_TIME=$(date +%s)

while true; do
    TIMESTAMP=$(date '+%H:%M:%S')

    # Fetch position
    RESPONSE=$(curl -s "http://localhost:8094/api/futures/ginie/autopilot/positions" --max-time 10 2>/dev/null)

    # Check if SQDUSDT exists using Python
    RESULT=$(echo "$RESPONSE" | python3 << 'PYEOF'
import json, sys

try:
    data = json.load(sys.stdin)
    positions = data.get('positions', [])

    for pos in positions:
        if pos['symbol'] == 'SQDUSDT':
            entry = pos['entry_price']
            current = pos['highest_price']
            qty = pos['remaining_qty']
            leverage = pos['leverage']
            side = pos['side']

            # Calculate ROI
            if side == "LONG":
                gross = (current - entry) * qty
            else:
                gross = (entry - current) * qty

            notional = qty * entry
            fees = (notional * 0.0004) + (current * qty * 0.0004)
            net = gross - fees
            roi = (net * leverage / notional * 100) if notional > 0 else 0

            print(f"{roi:.2f}")
            sys.exit(0)

    print("CLOSED")
    sys.exit(0)
except:
    print("ERROR")
    sys.exit(1)
PYEOF
)

    if [ "$RESULT" = "CLOSED" ]; then
        echo "[$TIMESTAMP] [SUCCESS] SQDUSDT position has been CLOSED!"
        echo ""
        echo "=========== POSITION CLOSED ==========="
        break
    elif [ "$RESULT" = "ERROR" ]; then
        echo "[$TIMESTAMP] [ERROR] Could not fetch position data"
    else
        ROI=$RESULT

        # Determine status
        if (( $(echo "$ROI >= $THRESHOLD" | bc -l) )); then
            echo "[$TIMESTAMP] [HIT] ROI: $ROI% >= $THRESHOLD% THRESHOLD! Position should close..."
        else
            GAP=$(echo "$THRESHOLD - $ROI" | bc -l)
            echo "[$TIMESTAMP] ROI: $ROI% | Gap: $GAP% | Progress: $(echo "scale=0; $ROI * 100 / $THRESHOLD" | bc)%"
        fi

        LAST_ROI=$ROI
    fi

    sleep $CHECK_INTERVAL
done

echo ""
echo "Monitoring completed at $(date '+%Y-%m-%d %H:%M:%S')"
