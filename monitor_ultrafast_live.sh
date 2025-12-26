#!/bin/bash

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║         ULTRA FAST LIVE TRADING MONITOR                        ║"
echo "║         Testing Live Trade Execution                           ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

LAST_TRADES=0
LAST_PNL=0
MONITOR_COUNT=0

while true; do
    MONITOR_COUNT=$((MONITOR_COUNT + 1))
    TIMESTAMP=$(date '+%H:%M:%S')
    
    # Get ultra_fast config and stats
    ULTRAFAST_JSON=$(curl -s --max-time 5 "http://localhost:8094/api/futures/ultrafast/config" 2>/dev/null)
    
    if [ $? -eq 0 ]; then
        # Extract stats using simple grep/sed parsing
        ENABLED=$(echo "$ULTRAFAST_JSON" | grep -o '"enabled":true' | head -1)
        TODAY_TRADES=$(echo "$ULTRAFAST_JSON" | grep -o '"today_trades":[0-9]*' | cut -d':' -f2)
        DAILY_PNL=$(echo "$ULTRAFAST_JSON" | grep -o '"daily_pnl":[0-9]*' | cut -d':' -f2)
        WIN_RATE=$(echo "$ULTRAFAST_JSON" | grep -o '"win_rate":[0-9]*' | cut -d':' -f2)
        
        # Check for new trades
        if [ "$TODAY_TRADES" -gt "$LAST_TRADES" ]; then
            echo "[$TIMESTAMP] 🚨 ULTRA FAST TRADE EXECUTED!"
            echo "               Total Trades Today: $TODAY_TRADES"
            echo "               Daily PnL: \$${DAILY_PNL}"
            echo "               Win Rate: ${WIN_RATE}%"
            echo ""
            LAST_TRADES=$TODAY_TRADES
            LAST_PNL=$DAILY_PNL
        elif [ $((MONITOR_COUNT % 3)) -eq 0 ]; then
            # Print status every 3 checks (roughly every 30 seconds)
            echo "[$TIMESTAMP] ✓ Monitoring... Ultra Fast: ${TODAY_TRADES} trades | PnL: \$${DAILY_PNL} | Enabled: $ENABLED"
        fi
    else
        echo "[$TIMESTAMP] ✗ Unable to connect to API"
    fi
    
    # Get current orders count
    ORDERS=$(curl -s --max-time 5 "http://localhost:8094/api/futures/orders/all" 2>/dev/null)
    if [ $? -eq 0 ]; then
        TOTAL_ALGO=$(echo "$ORDERS" | grep -o '"total_algo":[0-9]*' | cut -d':' -f2)
        echo "[$TIMESTAMP]   Active Orders: $TOTAL_ALGO algo orders"
    fi
    
    sleep 10
done
