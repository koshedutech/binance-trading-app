#!/bin/bash

# TP Hit Monitoring Script - Real-time position tracking
# Usage: ./tp_monitor.sh

API="http://localhost:8094/api/futures/ginie/autopilot/status"
LOG_FILE="./server.log"
INTERVAL=10
LAST_POSITION_COUNT=0
LAST_PNL=0

echo "╔════════════════════════════════════════════════════════╗"
echo "║     GINIE TP HIT LIVE MONITORING - ACTIVE              ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""
echo "API Endpoint: $API"
echo "Log File: $LOG_FILE"
echo "Update Interval: ${INTERVAL}s"
echo "Press Ctrl+C to stop"
echo ""

while true; do
    clear
    echo "╔════════════════════════════════════════════════════════╗"
    echo "║     GINIE TP HIT LIVE MONITORING - ACTIVE              ║"
    echo "║     $(date '+%Y-%m-%d %H:%M:%S')                           ║"
    echo "╚════════════════════════════════════════════════════════╝"
    echo ""

    # Fetch position data
    RESPONSE=$(curl -s --max-time 5 "$API" 2>/dev/null)

    if [ ! -z "$RESPONSE" ]; then
        # Extract position count
        POSITION_COUNT=$(echo "$RESPONSE" | grep -o '"symbol":"[^"]*"' | wc -l)

        echo "📊 POSITIONS: $POSITION_COUNT active"
        echo ""

        # Extract and display each position
        echo "$RESPONSE" | python3 << 'PYTHON'
import json
import sys

try:
    data = json.load(sys.stdin)
    positions = data.get('positions', [])
    stats = data.get('stats', {})

    for pos in positions:
        symbol = pos.get('symbol', 'N/A')
        side = pos.get('side', 'N/A')
        entry = pos.get('entry_price', 0)
        qty = pos.get('remaining_qty', 0)
        orig_qty = pos.get('original_qty', 0)
        mode = pos.get('mode', 'N/A')
        current_tp = pos.get('current_tp_level', 0)
        unrealized = pos.get('unrealized_pnl', 0)
        realized = pos.get('realized_pnl', 0)
        tps = pos.get('take_profits', [])

        # Print position header
        print(f"\n{'─' * 56}")
        print(f"📍 {symbol:15} | {side:6} | {mode.upper():8}")
        print(f"{'─' * 56}")

        # Print entry and quantity
        print(f"Entry:  ${entry:.8f} | Qty: {qty:.4f}/{orig_qty:.4f}")

        # Print TP progression line
        tp_line = "TP:     "
        for tp in tps:
            level = tp.get('level', 0)
            status = tp.get('status', 'pending')
            price = tp.get('price', 0)

            if status == 'hit':
                tp_line += f"[TP{level}✓] "
            elif current_tp + 1 == level:
                tp_line += f"[TP{level}⚠] "
            else:
                tp_line += f"[TP{level}○] "

        print(tp_line)

        # Print TP prices
        tp_prices = "        "
        for tp in tps:
            price = tp.get('price', 0)
            tp_prices += f"${price:.4f}  "
        print(tp_prices)

        # Print PnL
        pnl_color_u = "🟢" if unrealized >= 0 else "🔴"
        pnl_color_r = "🟢" if realized >= 0 else "🔴"
        print(f"PnL:    {pnl_color_u} Unrealized: ${unrealized:+.2f}  {pnl_color_r} Realized: ${realized:+.2f}")

        # Print current TP status
        if current_tp > 0:
            print(f"✅ TP{current_tp} HIT! - {current_tp} of 4 levels completed")
        else:
            print(f"⏳ Waiting for TP1 to be hit...")

    # Print summary
    print(f"\n{'─' * 56}")
    print("SUMMARY:")
    print(f"  Active Positions: {stats.get('active_positions', 0)}")
    print(f"  Combined PnL: ${stats.get('combined_pnl', 0):.2f}")
    print(f"  Daily PnL: ${stats.get('daily_pnl', 0):.2f}")
    print(f"  Total PnL: ${stats.get('total_pnl', 0):.2f}")
    print(f"  Win Rate: {stats.get('win_rate', 0)}%")

except Exception as e:
    print(f"Error: {e}", file=sys.stderr)
PYTHON

        echo ""
        echo "─────────────────────────────────────────────────────"
        echo "📋 RECENT TP EVENTS:"
        echo ""

        # Check logs for TP events
        if [ -f "$LOG_FILE" ]; then
            tail -50 "$LOG_FILE" | grep -E "TP level hit|placeNextTPOrder|Next take profit|Failed to place" | \
                grep -E "INFO|ERROR" | tail -5 | while read line; do
                if echo "$line" | grep -q "TP level hit"; then
                    echo "🟡 TP HIT: $line"
                elif echo "$line" | grep -q "Next take profit order placed"; then
                    echo "🟢 SUCCESS: $line"
                elif echo "$line" | grep -q "placeNextTPOrder called"; then
                    echo "🔵 FUNCTION: $line"
                elif echo "$line" | grep -q "Failed"; then
                    echo "🔴 ERROR: $line"
                fi
            done
        fi

    else
        echo "❌ Could not connect to server"
        echo "Make sure the server is running: ./binance-trading-bot.exe"
    fi

    echo ""
    echo "─────────────────────────────────────────────────────"
    printf "Next check in ${INTERVAL}s... (Ctrl+C to stop)\r"

    # Wait for interval but allow Ctrl+C
    sleep "$INTERVAL"

done
