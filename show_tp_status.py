#!/usr/bin/env python3

import urllib.request
import json
from datetime import datetime
import sys

def get_positions():
    try:
        req = urllib.request.Request('http://localhost:8094/api/futures/ginie/autopilot/status')
        with urllib.request.urlopen(req, timeout=5) as response:
            data = json.loads(response.read().decode())
            return data
    except Exception as e:
        print(f"Error: {e}")
        return None

def show_positions():
    data = get_positions()

    if not data:
        print("❌ Could not connect to server")
        print("Make sure it's running: ./binance-trading-bot.exe")
        return

    positions = data.get('positions', [])
    stats = data.get('stats', {})

    print("╔════════════════════════════════════════════════════════════════════╗")
    print("║               LIVE TP MONITORING - CURRENT STATUS                 ║")
    print("║              " + datetime.now().strftime('%Y-%m-%d %H:%M:%S') + "                           ║")
    print("╚════════════════════════════════════════════════════════════════════╝")
    print()
    print("📊 SUMMARY:")
    print(f"  Active Positions: {stats.get('active_positions', 0)}/10")
    print(f"  Combined PnL: ${stats.get('combined_pnl', 0):.2f}")
    print(f"  Daily PnL: ${stats.get('daily_pnl', 0):.2f}")
    print(f"  Total PnL: ${stats.get('total_pnl', 0):.2f}")
    print(f"  Win Rate: {stats.get('win_rate', 0)}%")
    print(f"  Mode: {'PAPER (Dry Run)' if stats.get('dry_run') else 'LIVE'}")
    print()

    for pos in positions:
        symbol = pos.get('symbol', 'N/A')
        side = pos.get('side', 'N/A')
        entry = pos.get('entry_price', 0)
        current_tp = pos.get('current_tp_level', 0)
        unrealized = pos.get('unrealized_pnl', 0)
        realized = pos.get('realized_pnl', 0)
        tps = pos.get('take_profits', [])
        mode = pos.get('mode', 'N/A')

        print("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
        print(f"{symbol:15} | {side:6} | {mode.upper():8} | Entry: ${entry:.8f}")
        print()

        # TP Progression line
        tp_line = "  TP Progress: "
        for tp in tps:
            level = tp.get('level', 0)
            status = tp.get('status', 'pending')
            if status == 'hit':
                tp_line += f"[TP{level}✓] "
            elif current_tp + 1 == level:
                tp_line += f"[TP{level}⚠] "
            else:
                tp_line += f"[TP{level}○] "
        print(tp_line)

        # TP prices
        tp_prices = "  Prices:       "
        for tp in tps:
            price = tp.get('price', 0)
            tp_prices += f"${price:.4f}  "
        print(tp_prices)

        # TP percentages
        tp_pcts = "  Allocation:   "
        for tp in tps:
            pct = tp.get('percent', 0)
            tp_pcts += f"{pct}%      "
        print(tp_pcts)
        print()

        # Status
        if current_tp > 0:
            print(f"  ✅ TP{current_tp} HIT! - {current_tp} of 4 levels completed")
        else:
            print(f"  ⏳ Waiting for TP1 to be hit...")

        pnl_str = f"  Realized PnL: ${realized:+.2f} | Unrealized PnL: ${unrealized:+.2f}"
        print(pnl_str)
        print()

if __name__ == '__main__':
    try:
        while True:
            show_positions()
            print("Next update in 10 seconds... (Ctrl+C to stop)")
            print()

            import time
            time.sleep(10)
    except KeyboardInterrupt:
        print("\nMonitoring stopped.")
        sys.exit(0)
