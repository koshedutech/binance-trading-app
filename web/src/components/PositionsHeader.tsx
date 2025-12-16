import React, { useState } from 'react';
import { X, RefreshCcw, Loader } from 'lucide-react';
import { useStore } from '../store';
import { apiService } from '../services/api';

export const PositionsHeader: React.FC = () => {
  const { positions } = useStore();
  const [showDetails, setShowDetails] = useState(false);
  const [reversingSymbol, setReversingSymbol] = useState<string | null>(null);
  const [closingSymbol, setClosingSymbol] = useState<string | null>(null);

  const totalPnL = positions.reduce((sum, pos) => sum + (pos.pnl || 0), 0);
  const totalPnLPercent = positions.length > 0
    ? positions.reduce((sum, pos) => sum + (pos.pnl_percent || 0), 0) / positions.length
    : 0;

  const handleReverse = async (symbol: string, currentSide: string, quantity: number) => {
    if (!confirm(`Reverse position for ${symbol}? This will close current position and open opposite.`)) {
      return;
    }

    setReversingSymbol(symbol);
    try {
      // Close current position
      await apiService.closePosition(symbol);

      // Open opposite position with double quantity
      const newSide = currentSide === 'LONG' ? 'SELL' : 'BUY';
      await apiService.placeOrder({
        symbol,
        side: newSide,
        order_type: 'MARKET',
        quantity: quantity * 2, // Double to reverse
        price: 0,
      });
    } catch (err: any) {
      console.error('Failed to reverse position:', err);
      alert(err.response?.data?.error || 'Failed to reverse position');
    } finally {
      setReversingSymbol(null);
    }
  };

  const handleClose = async (symbol: string) => {
    if (!confirm(`Close position for ${symbol}?`)) {
      return;
    }

    setClosingSymbol(symbol);
    try {
      await apiService.closePosition(symbol);
    } catch (err: any) {
      console.error('Failed to close position:', err);
      alert(err.response?.data?.error || 'Failed to close position');
    } finally {
      setClosingSymbol(null);
    }
  };

  if (positions.length === 0) {
    return (
      <div className="px-4 py-2 bg-gray-800 border-b border-gray-700">
        <div className="text-sm text-gray-400">No open positions</div>
      </div>
    );
  }

  return (
    <div className="bg-gray-800 border-b border-gray-700">
      <div
        className="px-4 py-2 cursor-pointer hover:bg-gray-750 transition-colors"
        onClick={() => setShowDetails(!showDetails)}
      >
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <div className="text-sm font-semibold text-gray-300">
              Positions: <span className="text-white">{positions.length}</span>
            </div>
            <div className={`text-sm font-bold ${totalPnL >= 0 ? 'text-green-400' : 'text-red-400'}`}>
              Total P&L: ${totalPnL.toFixed(2)} ({totalPnLPercent >= 0 ? '+' : ''}{totalPnLPercent.toFixed(2)}%)
            </div>
          </div>
          <div className="text-xs text-gray-400">
            {showDetails ? 'Click to hide' : 'Click to show details'}
          </div>
        </div>
      </div>

      {showDetails && (
        <div className="max-h-96 overflow-y-auto bg-gray-850">
          <div className="grid grid-cols-1 divide-y divide-gray-700">
            {positions.map((position) => {
              const pnl = position.pnl || 0;
              const pnlPercent = position.pnl_percent || 0;
              const isLong = position.side === 'LONG';

              return (
                <div key={position.symbol} className="p-4 hover:bg-gray-800 transition-colors">
                  <div className="flex items-start justify-between">
                    <div className="flex-1">
                      <div className="flex items-center gap-2 mb-2">
                        <h4 className="text-lg font-bold text-white">{position.symbol}</h4>
                        <span
                          className={`px-2 py-0.5 rounded text-xs font-semibold ${
                            isLong ? 'bg-green-600 text-white' : 'bg-red-600 text-white'
                          }`}
                        >
                          {position.side}
                        </span>
                        <span
                          className={`px-2 py-0.5 rounded text-xs font-semibold ${
                            pnl >= 0 ? 'bg-green-900/50 text-green-400' : 'bg-red-900/50 text-red-400'
                          }`}
                        >
                          {pnl >= 0 ? '+' : ''}${pnl.toFixed(2)} ({pnlPercent >= 0 ? '+' : ''}{pnlPercent.toFixed(2)}%)
                        </span>
                      </div>

                      <div className="grid grid-cols-2 md:grid-cols-5 gap-3 text-xs">
                        <div>
                          <div className="text-gray-400">Entry Price</div>
                          <div className="text-white font-semibold">${position.entry_price?.toFixed(4) || 'N/A'}</div>
                        </div>
                        <div>
                          <div className="text-gray-400">Mark Price</div>
                          <div className="text-white font-semibold">${position.current_price?.toFixed(4) || 'N/A'}</div>
                        </div>
                        <div>
                          <div className="text-gray-400">Liquidation</div>
                          <div className="text-yellow-400 font-semibold">
                            {(position as any).liquidation_price ? `$${(position as any).liquidation_price.toFixed(4)}` : 'N/A'}
                          </div>
                        </div>
                        <div>
                          <div className="text-gray-400">Quantity</div>
                          <div className="text-white font-semibold">{position.quantity?.toFixed(6) || 'N/A'}</div>
                        </div>
                        <div>
                          <div className="text-gray-400">USDT Used</div>
                          <div className="text-white font-semibold">
                            ${((position.entry_price || 0) * (position.quantity || 0)).toFixed(2)}
                          </div>
                        </div>
                      </div>
                    </div>

                    <div className="flex items-center gap-2 ml-4">
                      <button
                        onClick={() => handleReverse(position.symbol, position.side, position.quantity || 0)}
                        disabled={reversingSymbol === position.symbol}
                        className="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 text-white rounded text-xs font-semibold flex items-center gap-1 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                      >
                        {reversingSymbol === position.symbol ? (
                          <>
                            <Loader className="w-3 h-3 animate-spin" />
                            Reversing...
                          </>
                        ) : (
                          <>
                            <RefreshCcw className="w-3 h-3" />
                            Reverse
                          </>
                        )}
                      </button>
                      <button
                        onClick={() => handleClose(position.symbol)}
                        disabled={closingSymbol === position.symbol}
                        className="px-3 py-1.5 bg-red-600 hover:bg-red-700 text-white rounded text-xs font-semibold flex items-center gap-1 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                      >
                        {closingSymbol === position.symbol ? (
                          <>
                            <Loader className="w-3 h-3 animate-spin" />
                            Closing...
                          </>
                        ) : (
                          <>
                            <X className="w-3 h-3" />
                            Close
                          </>
                        )}
                      </button>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
};
