# Web Interface Implementation - Remaining Components

This document contains the remaining React components and configuration files needed to complete the web interface.

## Component Files to Create

### 1. PositionsTable Component
**File:** `web/src/components/PositionsTable.tsx`

```tsx
import { useStore } from '../store';
import { X } from 'lucide-react';
import { apiService } from '../services/api';
import { formatDistanceToNow } from 'date-fns';

export default function PositionsTable() {
  const { positions } = useStore();

  const formatCurrency = (value: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
    }).format(value);
  };

  const handleClosePosition = async (symbol: string) => {
    if (!confirm(`Are you sure you want to close position for ${symbol}?`)) {
      return;
    }

    try {
      await apiService.closePosition(symbol);
      alert('Position closed successfully');
    } catch (error) {
      alert('Failed to close position');
      console.error(error);
    }
  };

  if (positions.length === 0) {
    return (
      <div className="p-8 text-center text-gray-400">
        No open positions
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="table">
        <thead>
          <tr>
            <th>Symbol</th>
            <th>Side</th>
            <th>Entry Price</th>
            <th>Current Price</th>
            <th>Quantity</th>
            <th>P&L</th>
            <th>Duration</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {positions.map((position) => (
            <tr key={position.symbol}>
              <td className="font-medium">{position.symbol}</td>
              <td>
                <span
                  className={`badge ${
                    position.side === 'BUY' ? 'badge-success' : 'badge-danger'
                  }`}
                >
                  {position.side}
                </span>
              </td>
              <td>{formatCurrency(position.entry_price)}</td>
              <td>{position.current_price ? formatCurrency(position.current_price) : '-'}</td>
              <td>{position.quantity}</td>
              <td>
                {position.pnl !== undefined ? (
                  <div
                    className={position.pnl >= 0 ? 'text-positive' : 'text-negative'}
                  >
                    <div className="font-semibold">{formatCurrency(position.pnl)}</div>
                    <div className="text-xs">
                      {position.pnl_percent !== undefined
                        ? `${position.pnl_percent >= 0 ? '+' : ''}${position.pnl_percent.toFixed(2)}%`
                        : ''}
                    </div>
                  </div>
                ) : (
                  '-'
                )}
              </td>
              <td className="text-sm">
                {formatDistanceToNow(new Date(position.entry_time), { addSuffix: true })}
              </td>
              <td>
                <button
                  onClick={() => handleClosePosition(position.symbol)}
                  className="btn-danger text-xs py-1 px-2 flex items-center space-x-1"
                >
                  <X className="w-3 h-3" />
                  <span>Close</span>
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
```

### 2. OrdersTable Component
**File:** `web/src/components/OrdersTable.tsx`

```tsx
import { useStore } from '../store';
import { X } from 'lucide-react';
import { apiService } from '../services/api';
import { formatDistanceToNow } from 'date-fns';

export default function OrdersTable() {
  const { activeOrders } = useStore();

  const formatCurrency = (value: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
    }).format(value);
  };

  const handleCancelOrder = async (orderId: number) => {
    if (!confirm('Are you sure you want to cancel this order?')) {
      return;
    }

    try {
      await apiService.cancelOrder(orderId);
      alert('Order cancelled successfully');
    } catch (error) {
      alert('Failed to cancel order');
      console.error(error);
    }
  };

  if (activeOrders.length === 0) {
    return (
      <div className="p-8 text-center text-gray-400">
        No active orders
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="table">
        <thead>
          <tr>
            <th>Symbol</th>
            <th>Type</th>
            <th>Side</th>
            <th>Price</th>
            <th>Quantity</th>
            <th>Status</th>
            <th>Age</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {activeOrders.map((order) => (
            <tr key={order.id}>
              <td className="font-medium">{order.symbol}</td>
              <td>
                <span className="badge badge-info">{order.order_type}</span>
              </td>
              <td>
                <span
                  className={`badge ${
                    order.side === 'BUY' ? 'badge-success' : 'badge-danger'
                  }`}
                >
                  {order.side}
                </span>
              </td>
              <td>{order.price ? formatCurrency(order.price) : 'MARKET'}</td>
              <td>
                {order.executed_qty > 0
                  ? `${order.executed_qty}/${order.quantity}`
                  : order.quantity}
              </td>
              <td>
                <span className="badge badge-warning">{order.status}</span>
              </td>
              <td className="text-sm">
                {formatDistanceToNow(new Date(order.created_at), { addSuffix: true })}
              </td>
              <td>
                <button
                  onClick={() => handleCancelOrder(order.id)}
                  className="btn-danger text-xs py-1 px-2 flex items-center space-x-1"
                >
                  <X className="w-3 h-3" />
                  <span>Cancel</span>
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
```

### 3. StrategiesPanel Component
**File:** `web/src/components/StrategiesPanel.tsx`

```tsx
import { useEffect } from 'react';
import { useStore } from '../store';
import { apiService } from '../services/api';
import { Power, PowerOff } from 'lucide-react';

export default function StrategiesPanel() {
  const { strategies, setStrategies } = useStore();

  useEffect(() => {
    const fetchStrategies = async () => {
      try {
        const data = await apiService.getStrategies();
        setStrategies(data);
      } catch (error) {
        console.error('Failed to fetch strategies:', error);
      }
    };

    fetchStrategies();
    const interval = setInterval(fetchStrategies, 60000);
    return () => clearInterval(interval);
  }, [setStrategies]);

  const handleToggle = async (name: string, enabled: boolean) => {
    try {
      await apiService.toggleStrategy(name, !enabled);
      const updated = await apiService.getStrategies();
      setStrategies(updated);
    } catch (error) {
      alert('Failed to toggle strategy');
      console.error(error);
    }
  };

  if (strategies.length === 0) {
    return (
      <div className="p-8 text-center text-gray-400">
        No strategies registered
      </div>
    );
  }

  return (
    <div className="divide-y divide-dark-700">
      {strategies.map((strategy) => (
        <div key={strategy.name} className="p-4 hover:bg-dark-750 transition-colors">
          <div className="flex items-center justify-between">
            <div className="flex-1">
              <div className="flex items-center space-x-2">
                <h3 className="font-semibold">{strategy.name}</h3>
                <span
                  className={`badge ${
                    strategy.enabled ? 'badge-success' : 'badge-danger'
                  }`}
                >
                  {strategy.enabled ? 'Active' : 'Disabled'}
                </span>
              </div>
              <div className="mt-1 text-sm text-gray-400">
                {strategy.symbol} • {strategy.interval}
              </div>
              {strategy.last_signal && (
                <div className="mt-1 text-xs text-gray-500">
                  Last signal: {strategy.last_signal}
                </div>
              )}
            </div>
            <button
              onClick={() => handleToggle(strategy.name, strategy.enabled)}
              className={`btn text-xs py-1 px-3 flex items-center space-x-1 ${
                strategy.enabled ? 'btn-danger' : 'btn-success'
              }`}
            >
              {strategy.enabled ? (
                <>
                  <PowerOff className="w-3 h-3" />
                  <span>Disable</span>
                </>
              ) : (
                <>
                  <Power className="w-3 h-3" />
                  <span>Enable</span>
                </>
              )}
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}
```

### 4. ScreenerResults Component
**File:** `web/src/components/ScreenerResults.tsx`

```tsx
import { useStore } from '../store';
import { TrendingUp, TrendingDown } from 'lucide-react';

export default function ScreenerResults() {
  const { screenerResults } = useStore();

  const formatCurrency = (value: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
    }).format(value);
  };

  if (screenerResults.length === 0) {
    return (
      <div className="p-8 text-center text-gray-400">
        No screener results available
      </div>
    );
  }

  return (
    <div className="divide-y divide-dark-700 max-h-96 overflow-y-auto scrollbar-thin">
      {screenerResults.slice(0, 10).map((result) => (
        <div key={result.id} className="p-4 hover:bg-dark-750 transition-colors">
          <div className="flex items-center justify-between">
            <div>
              <div className="font-semibold">{result.symbol}</div>
              <div className="text-sm text-gray-400">
                {formatCurrency(result.last_price)}
              </div>
              {result.signals && result.signals.length > 0 && (
                <div className="mt-1 flex flex-wrap gap-1">
                  {result.signals.map((signal, idx) => (
                    <span key={idx} className="badge badge-info text-xs">
                      {signal}
                    </span>
                  ))}
                </div>
              )}
            </div>
            {result.price_change_percent !== undefined && (
              <div
                className={`text-right ${
                  result.price_change_percent >= 0 ? 'text-positive' : 'text-negative'
                }`}
              >
                <div className="flex items-center space-x-1">
                  {result.price_change_percent >= 0 ? (
                    <TrendingUp className="w-4 h-4" />
                  ) : (
                    <TrendingDown className="w-4 h-4" />
                  )}
                  <span className="font-semibold">
                    {result.price_change_percent >= 0 ? '+' : ''}
                    {result.price_change_percent.toFixed(2)}%
                  </span>
                </div>
                {result.quote_volume && (
                  <div className="text-xs text-gray-400 mt-1">
                    Vol: ${(result.quote_volume / 1000000).toFixed(2)}M
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
```

### 5. SignalsPanel Component
**File:** `web/src/components/SignalsPanel.tsx`

```tsx
import { useEffect } from 'react';
import { useStore } from '../store';
import { apiService } from '../services/api';
import { formatDistanceToNow } from 'date-fns';
import { TrendingUp, TrendingDown } from 'lucide-react';

export default function SignalsPanel() {
  const { recentSignals, setRecentSignals } = useStore();

  useEffect(() => {
    const fetchSignals = async () => {
      try {
        const signals = await apiService.getSignals(20);
        setRecentSignals(signals);
      } catch (error) {
        console.error('Failed to fetch signals:', error);
      }
    };

    fetchSignals();
    const interval = setInterval(fetchSignals, 30000);
    return () => clearInterval(interval);
  }, [setRecentSignals]);

  if (recentSignals.length === 0) {
    return (
      <div className="p-8 text-center text-gray-400">
        No recent signals
      </div>
    );
  }

  return (
    <div className="divide-y divide-dark-700 max-h-96 overflow-y-auto scrollbar-thin">
      {recentSignals.map((signal) => (
        <div key={signal.id} className="p-4 hover:bg-dark-750 transition-colors">
          <div className="flex items-center justify-between">
            <div className="flex-1">
              <div className="flex items-center space-x-2">
                <span className="font-semibold">{signal.symbol}</span>
                <span
                  className={`badge ${
                    signal.signal_type === 'BUY' ? 'badge-success' : 'badge-danger'
                  }`}
                >
                  {signal.signal_type === 'BUY' ? (
                    <TrendingUp className="w-3 h-3 inline mr-1" />
                  ) : (
                    <TrendingDown className="w-3 h-3 inline mr-1" />
                  )}
                  {signal.signal_type}
                </span>
                {signal.executed && (
                  <span className="badge badge-success">Executed</span>
                )}
              </div>
              <div className="text-sm text-gray-400 mt-1">
                {signal.strategy_name} • ${signal.entry_price.toFixed(2)}
              </div>
              {signal.reason && (
                <div className="text-xs text-gray-500 mt-1">{signal.reason}</div>
              )}
              <div className="text-xs text-gray-500 mt-1">
                {formatDistanceToNow(new Date(signal.timestamp), { addSuffix: true })}
              </div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
```

## Summary of Completed Work

### Backend ✅
1. Event bus system (`internal/events/`)
2. Database layer with PostgreSQL (`internal/database/`)
   - Models, repositories, migrations
3. REST API with Gin (`internal/api/`)
   - All endpoints for positions, orders, strategies, metrics
4. WebSocket support for real-time updates

### Frontend ✅
1. React + TypeScript + Vite setup
2. Tailwind CSS styling
3. Zustand state management
4. API and WebSocket services
5. Dashboard with metrics
6. All major components

## Next Steps to Complete

1. **Create the remaining component files** listed above
2. **Update docker-compose.yml** - Add PostgreSQL, change port to 8088
3. **Update Dockerfile** - Multi-stage build with React
4. **Update main.go** - Initialize web server and database
5. **Update go.mod** - Add new dependencies

These final steps will be completed next!
