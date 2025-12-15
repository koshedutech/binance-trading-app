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
