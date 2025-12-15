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
