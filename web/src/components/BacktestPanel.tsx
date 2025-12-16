import React, { useState, useEffect } from 'react';
import { apiService } from '../services/api';
import { ChartModal } from './ChartModal';
import type { BacktestResult, BacktestTrade } from '../types';
import { PlayCircle, BarChart3 } from 'lucide-react';

interface BacktestPanelProps {
  strategyConfigId: number;
  symbol: string;
  interval: string;
}

export const BacktestPanel: React.FC<BacktestPanelProps> = ({
  strategyConfigId,
  symbol,
  interval,
}) => {
  const [running, setRunning] = useState(false);
  const [results, setResults] = useState<BacktestResult[]>([]);
  const [selectedResult, setSelectedResult] = useState<BacktestResult | null>(null);
  const [trades, setTrades] = useState<BacktestTrade[]>([]);
  const [showChart, setShowChart] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Default to last 30 days
  const [startDate, setStartDate] = useState(() => {
    const date = new Date();
    date.setDate(date.getDate() - 30);
    return date.toISOString().split('T')[0];
  });
  const [endDate, setEndDate] = useState(() => {
    return new Date().toISOString().split('T')[0];
  });

  useEffect(() => {
    fetchBacktestResults();
  }, [strategyConfigId]);

  const fetchBacktestResults = async () => {
    try {
      const data = await apiService.getBacktestResults(strategyConfigId, 10);
      setResults(data);
      if (data.length > 0) {
        setSelectedResult(data[0]);
        loadTrades(data[0].id);
      }
    } catch (err: any) {
      console.error('Failed to fetch backtest results:', err);
    }
  };

  const runBacktest = async () => {
    setRunning(true);
    setError(null);

    try {
      const response = await apiService.runBacktest(strategyConfigId, {
        symbol,
        interval,
        start_date: startDate,
        end_date: endDate,
      });

      // Response contains both result and trades
      const result = response.result;
      const newTrades = response.trades || [];

      setResults([result, ...results]);
      setSelectedResult(result);
      setTrades(newTrades);
    } catch (err: any) {
      console.error('Backtest failed:', err);
      setError(err.response?.data?.message || err.message || 'Backtest failed');
    } finally {
      setRunning(false);
    }
  };

  const loadTrades = async (resultId: number) => {
    try {
      const tradesData = await apiService.getBacktestTrades(resultId);
      setTrades(tradesData);
    } catch (err) {
      console.error('Failed to load trades:', err);
    }
  };

  return (
    <div className="space-y-6">
      {/* Backtest Configuration */}
      <div className="bg-gray-800 rounded-lg p-4">
        <h3 className="text-lg font-semibold text-white mb-4">Run Backtest</h3>
        <div className="grid grid-cols-2 gap-4 mb-4">
          <div>
            <label className="block text-sm text-gray-400 mb-2">Start Date</label>
            <input
              type="date"
              value={startDate}
              onChange={(e) => setStartDate(e.target.value)}
              className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white focus:outline-none focus:border-blue-500"
            />
          </div>
          <div>
            <label className="block text-sm text-gray-400 mb-2">End Date</label>
            <input
              type="date"
              value={endDate}
              onChange={(e) => setEndDate(e.target.value)}
              className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white focus:outline-none focus:border-blue-500"
            />
          </div>
        </div>

        {error && (
          <div className="mb-4 p-3 bg-red-900/20 border border-red-500/50 rounded text-red-400 text-sm">
            {error}
          </div>
        )}

        <button
          onClick={runBacktest}
          disabled={running}
          className="w-full px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed rounded text-white font-medium flex items-center justify-center gap-2 transition-colors"
        >
          <PlayCircle className="w-5 h-5" />
          {running ? 'Running Backtest...' : 'Run Backtest'}
        </button>
      </div>

      {/* Results */}
      {selectedResult && (
        <div className="bg-gray-800 rounded-lg p-4">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-lg font-semibold text-white">Backtest Results</h3>
            <button
              onClick={() => setShowChart(true)}
              className="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 rounded text-sm text-white font-medium flex items-center gap-2 transition-colors"
            >
              <BarChart3 className="w-4 h-4" />
              View Chart
            </button>
          </div>

          <div className="grid grid-cols-3 gap-4">
            <div className="bg-gray-700/50 rounded-lg p-3">
              <div className="text-sm text-gray-400 mb-1">Total P&L</div>
              <div
                className={`text-2xl font-bold ${
                  selectedResult.net_pnl >= 0 ? 'text-green-400' : 'text-red-400'
                }`}
              >
                ${selectedResult.net_pnl.toFixed(2)}
              </div>
              <div className="text-xs text-gray-500 mt-1">
                Fees: ${selectedResult.total_fees.toFixed(2)}
              </div>
            </div>

            <div className="bg-gray-700/50 rounded-lg p-3">
              <div className="text-sm text-gray-400 mb-1">Win Rate</div>
              <div className="text-2xl font-bold text-white">
                {selectedResult.win_rate.toFixed(1)}%
              </div>
              <div className="text-xs text-gray-500 mt-1">
                {selectedResult.winning_trades}W / {selectedResult.losing_trades}L
              </div>
            </div>

            <div className="bg-gray-700/50 rounded-lg p-3">
              <div className="text-sm text-gray-400 mb-1">Total Trades</div>
              <div className="text-2xl font-bold text-white">
                {selectedResult.total_trades}
              </div>
              <div className="text-xs text-gray-500 mt-1">
                Avg {selectedResult.avg_trade_duration_minutes}m
              </div>
            </div>

            <div className="bg-gray-700/50 rounded-lg p-3">
              <div className="text-sm text-gray-400 mb-1">Profit Factor</div>
              <div className="text-lg font-semibold text-white">
                {selectedResult.profit_factor.toFixed(2)}
              </div>
            </div>

            <div className="bg-gray-700/50 rounded-lg p-3">
              <div className="text-sm text-gray-400 mb-1">Max Drawdown</div>
              <div className="text-lg font-semibold text-red-400">
                -{selectedResult.max_drawdown_percent.toFixed(2)}%
              </div>
            </div>

            <div className="bg-gray-700/50 rounded-lg p-3">
              <div className="text-sm text-gray-400 mb-1">Avg Win/Loss</div>
              <div className="text-xs">
                <span className="text-green-400">${selectedResult.average_win.toFixed(2)}</span>
                {' / '}
                <span className="text-red-400">${selectedResult.average_loss.toFixed(2)}</span>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Trades List */}
      {trades.length > 0 && (
        <div className="bg-gray-800 rounded-lg">
          <div className="p-4 border-b border-gray-700">
            <h3 className="text-lg font-semibold text-white">Trades ({trades.length})</h3>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="border-b border-gray-700">
                <tr>
                  <th className="text-left p-3 text-gray-400 font-medium">Entry</th>
                  <th className="text-left p-3 text-gray-400 font-medium">Exit</th>
                  <th className="text-right p-3 text-gray-400 font-medium">P&L</th>
                  <th className="text-right p-3 text-gray-400 font-medium">P&L %</th>
                  <th className="text-center p-3 text-gray-400 font-medium">Duration</th>
                </tr>
              </thead>
              <tbody>
                {trades.slice(0, 10).map((trade, idx) => (
                  <tr key={idx} className="border-b border-gray-700/50 hover:bg-gray-700/30">
                    <td className="p-3">
                      <div className="text-white text-xs">
                        {new Date(trade.entry_time).toLocaleString()}
                      </div>
                      <div className="text-xs text-gray-400">
                        ${trade.entry_price.toFixed(2)}
                      </div>
                    </td>
                    <td className="p-3">
                      <div className="text-white text-xs">
                        {new Date(trade.exit_time).toLocaleString()}
                      </div>
                      <div className="text-xs text-gray-400">
                        ${trade.exit_price.toFixed(2)}
                      </div>
                    </td>
                    <td
                      className={`p-3 text-right font-medium ${
                        trade.pnl >= 0 ? 'text-green-400' : 'text-red-400'
                      }`}
                    >
                      ${trade.pnl.toFixed(2)}
                    </td>
                    <td
                      className={`p-3 text-right ${
                        trade.pnl >= 0 ? 'text-green-400' : 'text-red-400'
                      }`}
                    >
                      {trade.pnl_percent.toFixed(2)}%
                    </td>
                    <td className="p-3 text-center text-gray-300">
                      {trade.duration_minutes}m
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {trades.length > 10 && (
            <div className="p-3 text-center text-xs text-gray-500">
              Showing first 10 of {trades.length} trades
            </div>
          )}
        </div>
      )}

      {/* Chart Modal */}
      <ChartModal
        isOpen={showChart}
        onClose={() => setShowChart(false)}
        symbol={symbol}
        interval={interval}
        backtestTrades={trades}
      />
    </div>
  );
};
