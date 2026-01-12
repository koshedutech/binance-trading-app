import React, { useState, useEffect } from 'react';
import { XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Area, AreaChart } from 'recharts';
import { TrendingUp, TrendingDown, DollarSign, Target, AlertTriangle } from 'lucide-react';
import { useFuturesStore } from '../store/futuresStore';

interface PnLData {
  timestamp: string;
  equity: number;
  dailyPnL: number;
  totalPnL: number;
}

interface Position {
  symbol: string;
  side: string;
  entry_price: number;
  current_price: number;
  quantity: number;
  pnl: number;
  pnl_percent: number;
}

interface RiskMetrics {
  account_balance: number;
  daily_pnl: number;
  daily_drawdown_percent: number;
  open_positions: number;
  max_positions: number;
  max_daily_drawdown: number;
  can_trade: boolean;
}

interface PnLDashboardProps {
  wsConnected?: boolean;
  initialBalance?: number;
}

const PnLDashboard: React.FC<PnLDashboardProps> = ({ wsConnected = false, initialBalance }) => {
  const [equityCurve, setEquityCurve] = useState<PnLData[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);
  const [riskMetrics, setRiskMetrics] = useState<RiskMetrics | null>(null);
  const [totalPnL, setTotalPnL] = useState(0);
  const [dailyPnL, setDailyPnL] = useState(0);
  const [winRate, setWinRate] = useState(0);
  const [maxDrawdown, setMaxDrawdown] = useState(0);

  // CRITICAL: Subscribe to trading mode changes to refresh mode-specific data (paper vs live)
  const tradingMode = useFuturesStore((state) => state.tradingMode);

  // Fetch initial data
  useEffect(() => {
    fetchDashboardData();
    const interval = setInterval(fetchDashboardData, 30000); // Reduced from 5s to 30s to avoid rate limits
    return () => clearInterval(interval);
  }, []);

  // CRITICAL: Refresh PnL data when trading mode changes (paper <-> live)
  useEffect(() => {
    console.log('PnLDashboard: Trading mode changed to', tradingMode.mode, '- refreshing');
    fetchDashboardData();
  }, [tradingMode.dryRun]);

  const fetchDashboardData = async () => {
    try {
      // Fetch positions
      const positionsRes = await fetch('/api/positions');
      if (positionsRes.ok) {
        const posData = await positionsRes.json();
        setPositions(posData || []);

        // Calculate total unrealized P&L
        const unrealizedPnL = (posData || []).reduce((sum: number, p: Position) => sum + p.pnl, 0);
        setTotalPnL(unrealizedPnL);
      }

      // Fetch risk metrics
      const riskRes = await fetch('/api/risk/metrics');
      if (riskRes.ok) {
        const riskData = await riskRes.json();
        setRiskMetrics(riskData);
        setDailyPnL(riskData.daily_pnl || 0);
      }

      // Fetch trade history for equity curve
      const historyRes = await fetch('/api/trades/history?limit=100');
      if (historyRes.ok) {
        const historyData = await historyRes.json();
        if (historyData && historyData.length > 0) {
          const curve = buildEquityCurve(historyData);
          setEquityCurve(curve);
          calculateMetrics(historyData);
        }
      }
    } catch (error) {
      console.error('Failed to fetch dashboard data:', error);
    }
  };

  const buildEquityCurve = (trades: any[]): PnLData[] => {
    // Use initialBalance prop, or first trade's balance, or 0
    const startingBalance = initialBalance ?? trades[0]?.cumulativeBalance ?? 0;
    let equity = startingBalance;
    return trades.map((trade) => {
      const pnl = trade.pnl || 0;
      equity += pnl;
      return {
        timestamp: new Date(trade.exit_time || trade.entry_time).toLocaleTimeString(),
        equity,
        dailyPnL: pnl,
        totalPnL: equity - startingBalance,
      };
    });
  };

  const calculateMetrics = (trades: any[]) => {
    if (trades.length === 0) return;

    const wins = trades.filter(t => (t.pnl || 0) > 0).length;
    setWinRate((wins / trades.length) * 100);

    // Calculate max drawdown
    const startingBalance = initialBalance ?? trades[0]?.cumulativeBalance ?? 0;
    let peak = startingBalance;
    let maxDD = 0;
    let equity = startingBalance;

    trades.forEach(trade => {
      equity += trade.pnl || 0;
      if (equity > peak) peak = equity;
      const dd = peak > 0 ? ((peak - equity) / peak) * 100 : 0;
      if (dd > maxDD) maxDD = dd;
    });

    setMaxDrawdown(maxDD);
  };

  const formatCurrency = (value: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
    }).format(value);
  };

  const formatPercent = (value: number | string | null | undefined) => {
    const num = typeof value === 'string' ? parseFloat(value) : value;
    if (num === null || num === undefined || isNaN(num)) return '0.00%';
    const sign = num >= 0 ? '+' : '';
    return `${sign}${Number(num).toFixed(2)}%`;
  };

  return (
    <div className="space-y-6">
      {/* Header Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {/* Total P&L */}
        <div className="bg-gray-800 rounded-lg p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-gray-400 text-sm">Total P&L</p>
              <p className={`text-2xl font-bold ${totalPnL >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                {formatCurrency(totalPnL)}
              </p>
            </div>
            {totalPnL >= 0 ? (
              <TrendingUp className="w-8 h-8 text-green-400" />
            ) : (
              <TrendingDown className="w-8 h-8 text-red-400" />
            )}
          </div>
        </div>

        {/* Daily P&L */}
        <div className="bg-gray-800 rounded-lg p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-gray-400 text-sm">Daily P&L</p>
              <p className={`text-2xl font-bold ${dailyPnL >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                {formatCurrency(dailyPnL)}
              </p>
            </div>
            <DollarSign className="w-8 h-8 text-blue-400" />
          </div>
        </div>

        {/* Win Rate */}
        <div className="bg-gray-800 rounded-lg p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-gray-400 text-sm">Win Rate</p>
              <p className="text-2xl font-bold text-white">{Number(winRate || 0).toFixed(1)}%</p>
            </div>
            <Target className="w-8 h-8 text-purple-400" />
          </div>
        </div>

        {/* Max Drawdown */}
        <div className="bg-gray-800 rounded-lg p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-gray-400 text-sm">Max Drawdown</p>
              <p className="text-2xl font-bold text-orange-400">{Number(maxDrawdown || 0).toFixed(2)}%</p>
            </div>
            <AlertTriangle className="w-8 h-8 text-orange-400" />
          </div>
        </div>
      </div>

      {/* Equity Curve Chart */}
      <div className="bg-gray-800 rounded-lg p-4">
        <h3 className="text-lg font-semibold text-white mb-4">Equity Curve</h3>
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={equityCurve}>
              <defs>
                <linearGradient id="equityGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#10B981" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#10B981" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
              <XAxis dataKey="timestamp" stroke="#9CA3AF" fontSize={12} />
              <YAxis stroke="#9CA3AF" fontSize={12} tickFormatter={(v) => `$${v}`} />
              <Tooltip
                contentStyle={{ backgroundColor: '#1F2937', border: 'none', borderRadius: '8px' }}
                labelStyle={{ color: '#9CA3AF' }}
                formatter={(value: number) => [formatCurrency(value), 'Equity']}
              />
              <Area
                type="monotone"
                dataKey="equity"
                stroke="#10B981"
                fill="url(#equityGradient)"
                strokeWidth={2}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Risk Metrics */}
      {riskMetrics && (
        <div className="bg-gray-800 rounded-lg p-4">
          <h3 className="text-lg font-semibold text-white mb-4">Risk Status</h3>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div>
              <p className="text-gray-400 text-sm">Account Balance</p>
              <p className="text-white font-semibold">{formatCurrency(riskMetrics.account_balance)}</p>
            </div>
            <div>
              <p className="text-gray-400 text-sm">Open Positions</p>
              <p className="text-white font-semibold">
                {riskMetrics.open_positions} / {riskMetrics.max_positions}
              </p>
            </div>
            <div>
              <p className="text-gray-400 text-sm">Daily Drawdown</p>
              <p className={`font-semibold ${riskMetrics.daily_drawdown_percent <= -3 ? 'text-red-400' : 'text-white'}`}>
                {formatPercent(riskMetrics.daily_drawdown_percent)}
              </p>
            </div>
            <div>
              <p className="text-gray-400 text-sm">Trading Status</p>
              <p className={`font-semibold ${riskMetrics.can_trade ? 'text-green-400' : 'text-red-400'}`}>
                {riskMetrics.can_trade ? 'Active' : 'Stopped'}
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Open Positions */}
      <div className="bg-gray-800 rounded-lg p-4">
        <h3 className="text-lg font-semibold text-white mb-4">
          Open Positions ({positions.length})
        </h3>
        {positions.length === 0 ? (
          <p className="text-gray-400 text-center py-4">No open positions</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="text-gray-400 text-sm border-b border-gray-700">
                  <th className="text-left py-2">Symbol</th>
                  <th className="text-left py-2">Side</th>
                  <th className="text-right py-2">Entry</th>
                  <th className="text-right py-2">Current</th>
                  <th className="text-right py-2">Quantity</th>
                  <th className="text-right py-2">P&L</th>
                  <th className="text-right py-2">P&L %</th>
                </tr>
              </thead>
              <tbody>
                {positions.map((pos, index) => (
                  <tr key={index} className="border-b border-gray-700">
                    <td className="py-3 font-semibold text-white">{pos.symbol}</td>
                    <td className={`py-3 ${pos.side === 'BUY' ? 'text-green-400' : 'text-red-400'}`}>
                      {pos.side}
                    </td>
                    <td className="py-3 text-right text-gray-300">{Number(pos.entry_price || 0).toFixed(4)}</td>
                    <td className="py-3 text-right text-gray-300">{Number(pos.current_price || 0).toFixed(4)}</td>
                    <td className="py-3 text-right text-gray-300">{Number(pos.quantity || 0).toFixed(6)}</td>
                    <td className={`py-3 text-right font-semibold ${pos.pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                      {formatCurrency(pos.pnl)}
                    </td>
                    <td className={`py-3 text-right font-semibold ${pos.pnl_percent >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                      {formatPercent(pos.pnl_percent)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Connection Status */}
      <div className="flex items-center justify-end space-x-2 text-sm">
        <div className={`w-2 h-2 rounded-full ${wsConnected ? 'bg-green-400' : 'bg-red-400'}`} />
        <span className="text-gray-400">{wsConnected ? 'Live' : 'Polling'}</span>
      </div>
    </div>
  );
};

export default PnLDashboard;
