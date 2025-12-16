import React, { useState } from 'react';
import { ChartModal } from './ChartModal';
import { SymbolSearchPicker } from './SymbolSearchPicker';
import { BarChart3 } from 'lucide-react';

interface ChartViewerProps {
  defaultSymbol?: string;
  defaultInterval?: string;
  backtestTrades?: any[];
}

export const ChartViewer: React.FC<ChartViewerProps> = ({
  defaultSymbol = 'BTCUSDT',
  defaultInterval = '5m',
  backtestTrades,
}) => {
  const [showChart, setShowChart] = useState(false);
  const [symbol, setSymbol] = useState(defaultSymbol);
  const [interval, setInterval] = useState(defaultInterval);

  return (
    <div className="bg-gray-800 rounded-lg p-4">
      <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
        <BarChart3 className="w-5 h-5" />
        Chart Viewer
      </h3>

      <div className="grid grid-cols-3 gap-4 mb-4">
        <div>
          <label className="block text-sm text-gray-400 mb-2">Symbol</label>
          <SymbolSearchPicker value={symbol} onChange={setSymbol} />
        </div>
        <div>
          <label className="block text-sm text-gray-400 mb-2">Interval</label>
          <select
            value={interval}
            onChange={(e) => setInterval(e.target.value)}
            className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white focus:outline-none focus:border-blue-500"
          >
            <option value="1m">1 Minute</option>
            <option value="5m">5 Minutes</option>
            <option value="15m">15 Minutes</option>
            <option value="30m">30 Minutes</option>
            <option value="1h">1 Hour</option>
            <option value="4h">4 Hours</option>
            <option value="1d">1 Day</option>
          </select>
        </div>
        <div className="flex items-end">
          <button
            onClick={() => setShowChart(true)}
            className="w-full px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded text-white font-medium flex items-center justify-center gap-2 transition-colors"
          >
            <BarChart3 className="w-5 h-5" />
            Load Chart
          </button>
        </div>
      </div>

      {backtestTrades && backtestTrades.length > 0 && (
        <div className="text-xs text-gray-400 mb-2">
          Chart will show {backtestTrades.length} backtest trades
        </div>
      )}

      <ChartModal
        isOpen={showChart}
        onClose={() => setShowChart(false)}
        symbol={symbol}
        interval={interval}
        backtestTrades={backtestTrades}
      />
    </div>
  );
};
