#!/usr/bin/env python3
"""
SQDUSDT Position Monitoring Script
Continuously monitors SQDUSDT until position closes via early profit booking
"""

import subprocess
import json
import time
from datetime import datetime, timedelta

THRESHOLDS = {'swing': 8.0}
FEE_RATE = 0.0004
CHECK_INTERVAL = 10  # seconds
MAX_MONITORING_TIME = 3600  # 1 hour

def calculate_roi(entry, current, qty, side, leverage):
    """Calculate ROI with leverage"""
    if side == "LONG":
        gross_pnl = (current - entry) * qty
    else:
        gross_pnl = (entry - current) * qty

    notional = qty * entry
    fees = (notional * FEE_RATE) + (current * qty * FEE_RATE)
    net_pnl = gross_pnl - fees

    if notional <= 0:
        return 0

    roi = (net_pnl * leverage / notional) * 100
    return roi

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

def find_sqdusdt_position(data):
    """Find SQDUSDT in positions"""
    if not data:
        return None

    positions = data.get('positions', [])
    for pos in positions:
        if pos['symbol'] == 'SQDUSDT':
            return pos
    return None

def check_server_log():
    """Check if SQDUSDT closed in logs"""
    try:
        result = subprocess.run(
            ['grep', '-i', 'sqdusdt.*booking profit', 'D:/Apps/binance-trading-bot/server.log'],
            capture_output=True,
            text=True,
            timeout=5
        )
        return result.stdout if result.stdout else None
    except:
        return None

def main():
    print("=" * 110)
    print("SQDUSDT POSITION MONITORING")
    print(f"Start Time: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 110)
    print("")
    print("Monitoring SQDUSDT (LONG, SWING mode)")
    print("Target ROI: 8.0%")
    print("Check Interval: 10 seconds")
    print("")

    start_time = datetime.now()
    check_count = 0
    last_roi = None
    roi_progression = []
    position_closed = False

    while True:
        elapsed_time = datetime.now() - start_time

        if elapsed_time.total_seconds() > MAX_MONITORING_TIME:
            print(f"\n[TIME]  Monitoring time limit reached ({MAX_MONITORING_TIME}s)")
            break

        check_count += 1
        timestamp = datetime.now().strftime('%H:%M:%S')

        # Fetch positions
        data = get_positions()
        sqdusdt_pos = find_sqdusdt_position(data)

        if not sqdusdt_pos:
            print(f"[{timestamp}] [WARNING] SQDUSDT position not found - may have been closed!")
            position_closed = True
            break

        # Calculate ROI
        roi = calculate_roi(
            sqdusdt_pos['entry_price'],
            sqdusdt_pos['highest_price'],
            sqdusdt_pos['remaining_qty'],
            sqdusdt_pos['side'],
            sqdusdt_pos['leverage']
        )

        threshold = THRESHOLDS.get(sqdusdt_pos['mode'], 8.0)
        gap = threshold - roi

        # Store ROI progression
        roi_progression.append({'time': timestamp, 'roi': roi})

        # Check if threshold hit
        if roi >= threshold:
            print(f"\n{'='*110}")
            print(f"[SUCCESS] ROI THRESHOLD HIT! ({roi:.2f}% >= {threshold}%)")
            print(f"{'='*110}")
            print(f"Time: {timestamp}")
            print(f"SQDUSDT should close automatically within 5 seconds...")
            time.sleep(5)
            position_closed = True
            break

        # Display status
        roi_change = ""
        if last_roi is not None:
            change = roi - last_roi
            if change > 0:
                roi_change = f" (↑ {change:+.2f}%)"
            elif change < 0:
                roi_change = f" (↓ {change:+.2f}%)"
            else:
                roi_change = " (→)"

        status_bar = ""
        if roi < 6.0:
            status_bar = "░░░░░░░░░░"
        elif roi < 7.0:
            status_bar = "██░░░░░░░░"
        elif roi < 7.5:
            status_bar = "████░░░░░░"
        elif roi < 8.0:
            status_bar = "██████░░░░"
        else:
            status_bar = "██████████"

        print(f"[{timestamp}] ROI: {roi:6.2f}% {roi_change} | Gap: {gap:5.2f}% | {status_bar} | Elapsed: {int(elapsed_time.total_seconds())}s")

        last_roi = roi
        time.sleep(CHECK_INTERVAL)

    # Final report
    print(f"\n{'='*110}")
    print("MONITORING SESSION COMPLETE")
    print(f"{'='*110}")

    if position_closed:
        print("\n[OK] POSITION CLOSED!")
        print(f"   Final ROI achieved: {last_roi:.2f}%")
        print(f"   Total monitoring time: {int((datetime.now() - start_time).total_seconds())}s")
        print(f"   Checks performed: {check_count}")

        # Check server log for confirmation
        log_entry = check_server_log()
        if log_entry:
            print("\n[OK] Confirmed in server.log:")
            print(f"   {log_entry[:200]}")
        else:
            print("\n[WARN]  Check server.log for 'Booking profit early' message")
    else:
        print("\n[TIME]  Monitoring time limit reached without threshold hit")
        if last_roi:
            print(f"   Last ROI: {last_roi:.2f}%")
            print(f"   Still need: {threshold - last_roi:.2f}% more")

    print(f"\n{'='*110}")

    # ROI Progression Summary
    if roi_progression:
        print("\nROI Progression:")
        for entry in roi_progression:
            print(f"  {entry['time']}: {entry['roi']:.2f}%")

    print(f"\n{'='*110}\n")

if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\n\n[STOP]  Monitoring stopped by user")
    except Exception as e:
        print(f"\n\n[ERROR] Error: {e}")
        import traceback
        traceback.print_exc()
