#!/usr/bin/env python3
"""
ROI Monitoring Script for Ginie Autopilot Positions
Tracks positions and monitors for ROI-based early profit booking
"""

import subprocess
import json
import sys
from datetime import datetime

THRESHOLDS = {'ultra_fast': 3.0, 'scalp': 5.0, 'swing': 8.0, 'position': 10.0}
FEE_RATE = 0.0004

def calculate_roi(entry, current, qty, side, leverage):
    """Calculate ROI with leverage consideration"""
    if side == "LONG":
        gross_pnl = (current - entry) * qty
    else:
        gross_pnl = (entry - current) * qty

    notional = qty * entry
    fees = (notional * FEE_RATE) + (current * qty * FEE_RATE)
    net_pnl = gross_pnl - fees

    if notional <= 0:
        return 0, 0

    roi = (net_pnl * leverage / notional) * 100
    return roi, net_pnl

def get_positions():
    """Fetch positions from API"""
    try:
        result = subprocess.run(
            ['curl', '-s', 'http://localhost:8094/api/futures/ginie/autopilot/positions', '--max-time', '10'],
            capture_output=True,
            text=True,
            timeout=15
        )
        return json.loads(result.stdout)
    except Exception as e:
        print(f"Error fetching positions: {e}")
        return None

def main():
    print("=" * 140)
    print("GINIE AUTOPILOT - ROI MONITORING REPORT")
    print(f"Timestamp: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 140)
    print("")

    data = get_positions()

    if not data:
        print("Error: Could not fetch position data")
        sys.exit(1)

    positions = data.get('positions', [])
    count = data.get('count', 0)

    print(f"{'Symbol':<15} {'Side':<6} {'Mode':<10} {'Lev':<3} {'Entry Price':<18} {'Current':<18} {'ROI%':<10} {'Threshold%':<12} {'Status':<25}")
    print("-" * 140)

    threshold_hits = []
    total_roi = 0
    positive_count = 0

    for pos in positions:
        symbol = pos['symbol']
        entry = pos['entry_price']
        current = pos['highest_price']
        qty = pos['remaining_qty']
        side = pos['side']
        leverage = pos['leverage']
        mode = pos['mode']

        roi, net_pnl = calculate_roi(entry, current, qty, side, leverage)
        threshold = THRESHOLDS.get(mode, 8.0)

        total_roi += roi
        if roi > 0:
            positive_count += 1

        if roi >= threshold:
            status = f"✓ THRESHOLD HIT ({roi - threshold:.1f}% above)"
            threshold_hits.append({
                'symbol': symbol,
                'mode': mode,
                'roi': roi,
                'threshold': threshold,
                'side': side,
                'entry': entry,
                'current': current,
                'leverage': leverage
            })
            color = "\033[92m"  # Green
        else:
            remaining = threshold - roi
            status = f"Waiting ({remaining:.1f}% more needed)"
            color = "\033[0m"   # Default

        print(f"{color}{symbol:<15} {side:<6} {mode:<10} {leverage:<3} {entry:<18.10f} {current:<18.10f} {roi:<10.2f} {threshold:<12.1f} {status:<25}\033[0m")

    print("-" * 140)

    if positions:
        avg_roi = total_roi / len(positions)
        win_rate = (positive_count / len(positions)) * 100
        print(f"Summary: {len(positions)} positions | Avg ROI: {avg_roi:.2f}% | Profitable: {positive_count}/{len(positions)} ({win_rate:.1f}%)")

    print("")

    if threshold_hits:
        print("=" * 140)
        print("⚠️  ROI THRESHOLD HITS - EARLY PROFIT BOOKING TRIGGERED")
        print("=" * 140)
        for i, hit in enumerate(threshold_hits, 1):
            print(f"\n{i}. {hit['symbol']} ({hit['mode'].upper()}) - {hit['side']}")
            print(f"   Entry Price: {hit['entry']:.10f}")
            print(f"   Current Price: {hit['current']:.10f}")
            print(f"   Price Move: {((hit['current'] - hit['entry']) / hit['entry'] * 100):+.2f}%")
            print(f"   ROI Achieved: {hit['roi']:.2f}% (Threshold: {hit['threshold']}%)")
            print(f"   Leverage: {hit['leverage']}x")
            print(f"   ✓ Action: Position will be CLOSED by early profit booking system")

        print(f"\n{'=' * 140}")
        print(f"Total Positions Ready for Exit: {len(threshold_hits)}")
        print(f"{'=' * 140}\n")
    else:
        print("=" * 140)
        print("Status: Monitoring active - waiting for positions to hit ROI thresholds")
        print("=" * 140)
        print("\nNext actions when ROI threshold hit:")
        print("  1. Position ROI >= threshold value triggers early profit booking")
        print("  2. Position is automatically CLOSED with reason: 'early_profit_booking'")
        print("  3. Trade is recorded in position history with realized PnL")
        print("  4. Log message: 'Booking profit early based on ROI threshold'")
        print("")

if __name__ == "__main__":
    main()
