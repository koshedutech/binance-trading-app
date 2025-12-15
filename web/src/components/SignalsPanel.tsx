import { useStore } from '../store';
import { formatDistanceToNow } from 'date-fns';
import { TrendingUp, TrendingDown } from 'lucide-react';

export default function SignalsPanel() {
  const { recentSignals } = useStore();

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
