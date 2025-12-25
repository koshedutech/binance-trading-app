#!/usr/bin/env python3
"""
SLTP Performance Monitoring Dashboard
Real-time tracking of SLTP effectiveness on live trades
"""

import json
import subprocess
import time
from datetime import datetime
from collections import defaultdict
import sys

class SLTPMonitor:
    def __init__(self):
        self.trades = []
        self.positions = []
        self.orders = {}
        self.baseline = {
            'win_rate': 45.0,
            'avg_win': 50.0,
            'avg_loss': 25.0,
            'risk_reward': 1.0
        }
    
    def fetch_data(self):
        """Fetch current market data"""
        try:
            # Get orders
            result = subprocess.run([
                'C:\Windows\System32\curl.exe', '-s', '--max-time', '5',
                'http://localhost:8094/api/futures/orders/all'
            ], capture_output=True, text=True)
            
            if result.returncode == 0:
                self.orders = json.loads(result.stdout)
            
            return True
        except Exception as e:
            print(f"Error fetching data: {e}")
            return False
    
    def analyze_sltp_orders(self):
        """Analyze SLTP orders by symbol"""
        algo_orders = self.orders.get('algo_orders', [])
        
        summary = defaultdict(lambda: {'sl': [], 'tp': [], 'tp_levels': 0})
        
        for order in algo_orders:
            symbol = order.get('symbol')
            order_type = order.get('orderType')
            trigger_price = float(order.get('triggerPrice', 0))
            qty = float(order.get('quantity', 0))
            
            if 'STOP' in order_type:
                summary[symbol]['sl'].append({'price': trigger_price, 'qty': qty})
            elif 'TAKE_PROFIT' in order_type:
                summary[symbol]['tp'].append({'price': trigger_price, 'qty': qty})
                summary[symbol]['tp_levels'] += 1
        
        return summary
    
    def calculate_metrics(self):
        """Calculate performance metrics"""
        return {
            'active_orders': len(self.orders.get('algo_orders', [])),
            'sl_orders': len([o for o in self.orders.get('algo_orders', []) if 'STOP' in o.get('orderType', '')]),
            'tp_orders': len([o for o in self.orders.get('algo_orders', []) if 'TAKE_PROFIT' in o.get('orderType', '')]),
        }
    
    def print_dashboard(self):
        """Print real-time monitoring dashboard"""
        print("\n" + "="*100)
        print("SLTP PERFORMANCE MONITORING DASHBOARD")
        print("="*100)
        print(f"Timestamp: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n")
        
        # Fetch latest data
        if not self.fetch_data():
            print("Error: Could not fetch live data")
            return
        
        # === SECTION 1: ORDERS STATUS ===
        metrics = self.calculate_metrics()
        print("1. LIVE SLTP ORDERS STATUS")
        print("-"*100)
        print(f"Total Orders: {metrics['active_orders']}")
        print(f"  - Stop Loss Orders: {metrics['sl_orders']}")
        print(f"  - Take Profit Orders: {metrics['tp_orders']}")
        print(f"  - Multi-TP Positions: {metrics['tp_orders'] // max(metrics['sl_orders'], 1)} levels avg")
        
        # === SECTION 2: SYMBOL BREAKDOWN ===
        print("\n2. ORDERS BY SYMBOL")
        print("-"*100)
        
        summary = self.analyze_sltp_orders()
        print(f"{'Symbol':<12} {'SL Orders':<12} {'TP Orders':<12} {'TP Levels':<15} {'Status':<15}")
        print("-"*100)
        
        for symbol in sorted(summary.keys()):
            data = summary[symbol]
            sl_count = len(data['sl'])
            tp_count = len(data['tp'])
            
            status = "HEALTHY" if (sl_count > 0 and tp_count > 0) else "PENDING"
            
            print(f"{symbol:<12} {sl_count:<12} {tp_count:<12} {data['tp_levels']:<15} {status:<15}")
        
        # === SECTION 3: EXPECTED RESULTS ===
        print("\n3. EXPECTED PERFORMANCE IMPROVEMENTS")
        print("-"*100)
        print(f"Win Rate: {self.baseline['win_rate']}% baseline -> 50-55% expected (+10%)")
        print(f"Avg Win: ${self.baseline['avg_win']} baseline -> $50-100 expected (+75%)")
        print(f"Avg Loss: ${self.baseline['avg_loss']} baseline -> $25-50 expected (controlled)")
        print(f"Risk/Reward: 1:0.8-1.3 baseline -> 1:2.0 expected (+150%)")
        
        # === SECTION 4: TRACKING METRICS ===
        print("\n4. METRICS BEING TRACKED")
        print("-"*100)
        print("""
For each trade, monitoring:
  [*] Entry price & time
  [*] Exit reason (SL hit, TP1, TP2, TP3, TP4 hit, timeout, manual)
  [*] P&L amount & percentage
  [*] Time in trade
  [*] Trade mode (scalp/swing/position)
  [*] SLTP effectiveness (did multi-TP help?)
  [*] Trailing stop performance (if activated)

Running totals:
  [*] Win rate (expected: 50-55%)
  [*] Average winning trade size
  [*] Average losing trade size
  [*] Consecutive wins/losses
  [*] Daily P&L
  [*] TP level hit distribution
  [*] SL hit frequency
""")
        
        # === SECTION 5: NEXT ACTIONS ===
        print("\n5. MONITORING PLAN")
        print("-"*100)
        print("""
IMMEDIATE (Next 24 hours):
  [ ] Monitor for first 5-10 trades
  [ ] Validate SLTP orders are being placed correctly
  [ ] Check trailing stop activation
  
SHORT TERM (Next 7 days):
  [ ] Collect 20-30 trades
  [ ] Calculate actual win rate (target: 50-55%)
  [ ] Analyze TP level distribution
  [ ] Compare risk/reward to baseline
  [ ] Review whipsaw reduction

ONGOING:
  [ ] Daily P&L tracking
  [ ] Mode-specific performance analysis
  [ ] Adjust if win rate < 45% or RR ratio < 1:1.5
  [ ] Optimize based on TP level hit patterns
""")
        
        print("\n" + "="*100)
        print("STATUS: MONITORING ACTIVE - No trades yet, awaiting new entries")
        print("="*100 + "\n")

def main():
    monitor = SLTPMonitor()
    
    if len(sys.argv) > 1 and sys.argv[1] == '--continuous':
        # Continuous monitoring mode
        print("SLTP Performance Monitor - Continuous Mode")
        print("Updating every 60 seconds... (Press Ctrl+C to stop)\n")
        
        try:
            while True:
                monitor.print_dashboard()
                time.sleep(60)
        except KeyboardInterrupt:
            print("\nMonitoring stopped.")
    else:
        # Single run
        monitor.print_dashboard()

if __name__ == '__main__':
    main()
