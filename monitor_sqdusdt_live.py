#!/usr/bin/env python3
"""
SQDUSDT Live Monitoring Script
Monitors position closure with real-time ROI tracking and log analysis
"""

import subprocess
import json
import time
from datetime import datetime, timedelta
import sys

THRESHOLD = 8.0
FEE_RATE = 0.0004
CHECK_INTERVAL = 5  # seconds
API_URL = "http://localhost:8094/api/futures/ginie/autopilot/positions"
LOG_FILE = "D:\\Apps\\binance-trading-bot\\server.log"

def get_positions():
    """Fetch positions from API"""
    try:
        result = subprocess.run(
            ['curl', '-s', '--max-time', '10', API_URL],
            capture_output=True,
            text=True,
            timeout=15
        )
        return json.loads(result.stdout)
    except Exception as e:
        print(f"[ERROR] Failed to fetch positions: {e}")
        return None

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

    roi = (net_pnl * float(leverage) / notional) * 100
    return roi

def check_logs_for_success():
    """Check server logs for close success message"""
    try:
        with open(LOG_FILE, 'r', errors='ignore') as f:
            lines = f.readlines()

        # Look for the most recent full close success for SQDUSDT
        for line in reversed(lines[-500:]):
            if 'full close order placed' in line.lower() and 'SQDUSDT' in line:
                try:
                    data = json.loads(line)
                    if data.get('fields', {}).get('symbol') == 'SQDUSDT':
                        return True, data
                except:
                    pass

            # Also check for successful close in Ginie closing message
            if 'Ginie closing position' in line and 'SQDUSDT' in line:
                try:
                    data = json.loads(line)
                    if data.get('fields', {}).get('symbol') == 'SQDUSDT':
                        return 'closing', data
                except:
                    pass

        return False, None
    except Exception as e:
        print(f"[ERROR] Failed to check logs: {e}")
        return False, None

def print_status_bar(roi):
    """Print visual ROI progress bar"""
    if roi < 2:
        return "░░░░░░░░░░"
    elif roi < 4:
        return "██░░░░░░░░"
    elif roi < 6:
        return "████░░░░░░"
    elif roi < 8:
        return "██████░░░░"
    else:
        return "██████████"

def main():
    print("\n" + "="*100)
    print("SQDUSDT EARLY PROFIT BOOKING MONITOR")
    print("="*100)
    print(f"Start Time: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print(f"Target Threshold: {THRESHOLD}% ROI")
    print(f"Check Interval: {CHECK_INTERVAL} seconds")
    print("="*100 + "\n")

    start_time = datetime.now()
    check_count = 0
    last_roi = None
    roi_history = []

    while True:
        try:
            elapsed = datetime.now() - start_time
            check_count += 1
            timestamp = datetime.now().strftime('%H:%M:%S')

            # Fetch position data
            data = get_positions()
            if not data:
                print(f"[{timestamp}] [ERROR] Failed to fetch position data")
                time.sleep(CHECK_INTERVAL)
                continue

            positions = data.get('positions', [])
            sqdusdt_pos = None

            for pos in positions:
                if pos['symbol'] == 'SQDUSDT':
                    sqdusdt_pos = pos
                    break

            if not sqdusdt_pos:
                print(f"[{timestamp}] [CLOSED] SQDUSDT position not found - position may have been closed!")

                # Check logs for confirmation
                success, log_data = check_logs_for_success()
                if success:
                    try:
                        fields = log_data.get('fields', {})
                        print(f"\n[SUCCESS] Position closed successfully!")
                        print(f"  Exit Price: {fields.get('current_price', 'N/A')}")
                        print(f"  Realized PnL: {fields.get('net_pnl', 'N/A')}")
                        if last_roi:
                            print(f"  Final ROI: {last_roi:.2f}%")
                    except:
                        pass

                print("\n" + "="*100)
                print(f"Monitoring completed at {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
                print("="*100 + "\n")
                break

            # Calculate ROI
            entry = sqdusdt_pos['entry_price']
            current = sqdusdt_pos['highest_price']
            qty = sqdusdt_pos['remaining_qty']
            leverage = sqdusdt_pos['leverage']

            roi = calculate_roi(entry, current, qty, sqdusdt_pos['side'], leverage)
            gap = THRESHOLD - roi
            roi_history.append(roi)

            # Check for close in progress
            close_status, _ = check_logs_for_success()
            close_indicator = ""
            if close_status == 'closing':
                close_indicator = " [CLOSING...]"
            elif close_status:
                close_indicator = " [CLOSED!]"

            # Display status
            status_bar = print_status_bar(roi)

            if roi >= THRESHOLD:
                print(f"[{timestamp}] [HIT]     ROI: {roi:6.2f}% {status_bar} | {close_indicator}")
            elif gap <= 0.5:
                print(f"[{timestamp}] [CRITICAL] ROI: {roi:6.2f}% {status_bar} | Gap: {gap:5.2f}% (VERY CLOSE!)")
            elif gap <= 1.0:
                print(f"[{timestamp}] [CLOSE] ROI: {roi:6.2f}% {status_bar} | Gap: {gap:5.2f}%")
            else:
                progress = (roi / THRESHOLD) * 100
                print(f"[{timestamp}] ROI: {roi:6.2f}% {status_bar} | Gap: {gap:5.2f}% | Progress: {progress:3.0f}%")

            last_roi = roi
            time.sleep(CHECK_INTERVAL)

        except KeyboardInterrupt:
            print(f"\n\n[STOPPED] Monitoring stopped by user")
            if roi_history:
                print(f"[INFO] ROI progression: {[f'{r:.2f}%' for r in roi_history[:10]]}")
            break
        except Exception as e:
            print(f"[{timestamp}] [ERROR] {e}")
            time.sleep(CHECK_INTERVAL)

if __name__ == "__main__":
    main()
