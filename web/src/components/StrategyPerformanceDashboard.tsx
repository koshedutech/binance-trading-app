import React, { useState, useEffect } from 'react';
import {
  TrendingUp,
  TrendingDown,
  Activity,
  DollarSign,
  Target,
  AlertTriangle,
  BarChart2,
  Clock,
  Award,
  ChevronDown,
  ChevronUp,
  RefreshCw,
  Filter,
} from 'lucide-react';
import { apiService } from '../services/api';

interface StrategyPerformance {
  strategyName: string;
  symbol: string;
  totalTrades: number;
  winningTrades: number;
  losingTrades: number;
  winRate: number;
  totalPnL: number;
  avgPnL: number;
  avgWin: number;
  avgLoss: number;
  largestWin: number;
  largestLoss: number;
  profitFactor: number;
  maxDrawdown: number;
  sharpeRatio?: number;
  avgHoldTime?: number;
  lastTradeTime?: string;
  status: 'active' | 'paused' | 'stopped';
  trend: 'up' | 'down' | 'neutral';
  recentPnL: number[];
}

interface OverallMetrics {
  totalStrategies: number;
  activeStrategies: number;
  totalTrades: number;
  totalPnL: number;
  overallWinRate: number;
  todayPnL: number;
  weekPnL: number;
  monthPnL: number;
  bestStrategy: string;
  worstStrategy: string;
}

const MetricCard: React.FC<{
  title: string;
  value: string | number;
  change?: number;
  icon: React.ElementType;
  color: string;
  subtitle?: string;
}> = ({ title, value, change, icon: Icon, color, subtitle }) => (
  <div className="bg-gray-700/50 rounded-lg p-4 border border-gray-600">
    <div className="flex items-center justify-between mb-2">
      <span className="text-sm text-gray-400">{title}</span>
      <Icon className={`w-5 h-5 ${color}`} />
    </div>
    <div className="flex items-end justify-between">
      <div>
        <div className={`text-2xl font-bold ${color}`}>{value}</div>
        {subtitle && <div className="text-xs text-gray-500">{subtitle}</div>}
      </div>
      {change !== undefined && (
        <div className={`flex items-center text-sm ${change >= 0 ? 'text-green-400' : 'text-red-400'}`}>
          {change >= 0 ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
          {Math.abs(change).toFixed(1)}%
        </div>
      )}
    </div>
  </div>
);

const MiniSparkline: React.FC<{ data: number[]; color: string }> = ({ data, color }) => {
  if (!data || data.length === 0) return null;

  const min = Math.min(...data);
  const max = Math.max(...data);
  const range = max - min || 1;
  const height = 30;
  const width = 80;
  const points = data.map((v, i) => {
    const x = (i / (data.length - 1)) * width;
    const y = height - ((v - min) / range) * height;
    return `${x},${y}`;
  }).join(' ');

  return (
    <svg width={width} height={height} className="inline-block">
      <polyline
        points={points}
        fill="none"
        stroke={color}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
};

const PerformanceBar: React.FC<{ winRate: number }> = ({ winRate }) => (
  <div className="flex items-center gap-2">
    <div className="flex-1 h-2 bg-gray-600 rounded-full overflow-hidden">
      <div
        className={`h-full rounded-full transition-all ${
          winRate >= 60 ? 'bg-green-500' : winRate >= 50 ? 'bg-yellow-500' : 'bg-red-500'
        }`}
        style={{ width: `${winRate}%` }}
      />
    </div>
    <span className={`text-sm font-medium ${
      winRate >= 60 ? 'text-green-400' : winRate >= 50 ? 'text-yellow-400' : 'text-red-400'
    }`}>
      {Number(winRate || 0).toFixed(1)}%
    </span>
  </div>
);

export const StrategyPerformanceDashboard: React.FC = () => {
  const [performances, setPerformances] = useState<StrategyPerformance[]>([]);
  const [overallMetrics, setOverallMetrics] = useState<OverallMetrics | null>(null);
  const [loading, setLoading] = useState(true);
  const [timeRange, setTimeRange] = useState<'today' | 'week' | 'month' | 'all'>('all');
  const [sortBy, setSortBy] = useState<'winRate' | 'pnl' | 'trades'>('pnl');
  const [expandedStrategy, setExpandedStrategy] = useState<string | null>(null);

  const fetchPerformanceData = async () => {
    setLoading(true);
    try {
      // Fetch metrics and trades to calculate strategy performance
      const [metrics, trades, strategies] = await Promise.all([
        apiService.getMetrics(),
        apiService.getPositionHistory(500),
        apiService.getStrategies(),
      ]);

      // Calculate per-strategy performance
      const strategyMap = new Map<string, StrategyPerformance>();

      trades.forEach((trade) => {
        const strategyName = trade.strategy_name || 'Manual';
        if (!strategyMap.has(strategyName)) {
          strategyMap.set(strategyName, {
            strategyName,
            symbol: trade.symbol,
            totalTrades: 0,
            winningTrades: 0,
            losingTrades: 0,
            winRate: 0,
            totalPnL: 0,
            avgPnL: 0,
            avgWin: 0,
            avgLoss: 0,
            largestWin: 0,
            largestLoss: 0,
            profitFactor: 0,
            maxDrawdown: 0,
            status: 'active',
            trend: 'neutral',
            recentPnL: [],
          });
        }

        const perf = strategyMap.get(strategyName)!;
        perf.totalTrades++;

        const pnl = trade.pnl || 0;
        perf.totalPnL += pnl;
        perf.recentPnL.push(pnl);

        if (pnl > 0) {
          perf.winningTrades++;
          perf.avgWin = (perf.avgWin * (perf.winningTrades - 1) + pnl) / perf.winningTrades;
          perf.largestWin = Math.max(perf.largestWin, pnl);
        } else if (pnl < 0) {
          perf.losingTrades++;
          perf.avgLoss = (perf.avgLoss * (perf.losingTrades - 1) + Math.abs(pnl)) / perf.losingTrades;
          perf.largestLoss = Math.min(perf.largestLoss, pnl);
        }

        perf.lastTradeTime = trade.exit_time || trade.entry_time;
      });

      // Calculate derived metrics
      strategyMap.forEach((perf) => {
        perf.winRate = perf.totalTrades > 0 ? (perf.winningTrades / perf.totalTrades) * 100 : 0;
        perf.avgPnL = perf.totalTrades > 0 ? perf.totalPnL / perf.totalTrades : 0;

        const totalWins = perf.avgWin * perf.winningTrades;
        const totalLosses = perf.avgLoss * perf.losingTrades;
        perf.profitFactor = totalLosses > 0 ? totalWins / totalLosses : totalWins > 0 ? Infinity : 0;

        // Calculate trend from recent P&L
        if (perf.recentPnL.length >= 5) {
          const recent = perf.recentPnL.slice(-5);
          const recentSum = recent.reduce((a, b) => a + b, 0);
          perf.trend = recentSum > 0 ? 'up' : recentSum < 0 ? 'down' : 'neutral';
        }

        // Keep only last 10 for sparkline
        perf.recentPnL = perf.recentPnL.slice(-10);

        // Calculate max drawdown
        let peak = 0;
        let maxDD = 0;
        let cumulative = 0;
        trades.filter(t => (t.strategy_name || 'Manual') === perf.strategyName).forEach(t => {
          cumulative += t.pnl || 0;
          peak = Math.max(peak, cumulative);
          maxDD = Math.min(maxDD, cumulative - peak);
        });
        perf.maxDrawdown = Math.abs(maxDD);
      });

      // Update strategy status from API
      strategies.forEach((s) => {
        const perf = strategyMap.get(s.name);
        if (perf) {
          perf.status = s.enabled ? 'active' : 'paused';
        }
      });

      const performanceList = Array.from(strategyMap.values());

      // Calculate overall metrics
      const overall: OverallMetrics = {
        totalStrategies: performanceList.length,
        activeStrategies: performanceList.filter(p => p.status === 'active').length,
        totalTrades: metrics.total_trades || 0,
        totalPnL: performanceList.reduce((sum, p) => sum + p.totalPnL, 0),
        overallWinRate: metrics.win_rate || 0,
        todayPnL: 0, // Would need date filtering
        weekPnL: 0,
        monthPnL: 0,
        bestStrategy: performanceList.sort((a, b) => b.totalPnL - a.totalPnL)[0]?.strategyName || 'N/A',
        worstStrategy: performanceList.sort((a, b) => a.totalPnL - b.totalPnL)[0]?.strategyName || 'N/A',
      };

      setPerformances(performanceList);
      setOverallMetrics(overall);
    } catch (error) {
      console.error('Failed to fetch performance data:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchPerformanceData();
    const interval = setInterval(fetchPerformanceData, 30000); // Refresh every 30s
    return () => clearInterval(interval);
  }, [timeRange]);

  const sortedPerformances = [...performances].sort((a, b) => {
    switch (sortBy) {
      case 'winRate': return b.winRate - a.winRate;
      case 'pnl': return b.totalPnL - a.totalPnL;
      case 'trades': return b.totalTrades - a.totalTrades;
      default: return 0;
    }
  });

  if (loading && !overallMetrics) {
    return (
      <div className="flex items-center justify-center h-64">
        <RefreshCw className="w-8 h-8 text-blue-400 animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold text-white flex items-center gap-2">
          <BarChart2 className="w-6 h-6 text-blue-400" />
          Strategy Performance Dashboard
        </h2>
        <div className="flex items-center gap-2">
          {/* Time Range Filter */}
          <div className="flex items-center gap-1 bg-gray-700 rounded-lg p-1">
            {(['today', 'week', 'month', 'all'] as const).map((range) => (
              <button
                key={range}
                onClick={() => setTimeRange(range)}
                className={`px-3 py-1 rounded text-sm transition-colors ${
                  timeRange === range
                    ? 'bg-blue-600 text-white'
                    : 'text-gray-400 hover:text-white'
                }`}
              >
                {range.charAt(0).toUpperCase() + range.slice(1)}
              </button>
            ))}
          </div>
          <button
            onClick={fetchPerformanceData}
            className="p-2 bg-gray-700 hover:bg-gray-600 rounded-lg text-gray-400 hover:text-white transition-colors"
            disabled={loading}
          >
            <RefreshCw className={`w-5 h-5 ${loading ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {/* Overall Metrics */}
      {overallMetrics && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <MetricCard
            title="Total P&L"
            value={`$${Number(overallMetrics.totalPnL || 0).toFixed(2)}`}
            icon={DollarSign}
            color={overallMetrics.totalPnL >= 0 ? 'text-green-400' : 'text-red-400'}
            subtitle={`${overallMetrics.totalTrades} trades`}
          />
          <MetricCard
            title="Win Rate"
            value={`${Number(overallMetrics.overallWinRate || 0).toFixed(1)}%`}
            icon={Target}
            color={overallMetrics.overallWinRate >= 50 ? 'text-green-400' : 'text-red-400'}
          />
          <MetricCard
            title="Active Strategies"
            value={`${overallMetrics.activeStrategies}/${overallMetrics.totalStrategies}`}
            icon={Activity}
            color="text-blue-400"
          />
          <MetricCard
            title="Best Performer"
            value={overallMetrics.bestStrategy}
            icon={Award}
            color="text-yellow-400"
          />
        </div>
      )}

      {/* Strategy Performance Table */}
      <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
        <div className="p-4 border-b border-gray-700 flex items-center justify-between">
          <h3 className="font-medium text-white">Strategy Performance</h3>
          <div className="flex items-center gap-2">
            <Filter className="w-4 h-4 text-gray-400" />
            <select
              value={sortBy}
              onChange={(e) => setSortBy(e.target.value as any)}
              className="bg-gray-700 border border-gray-600 rounded px-2 py-1 text-sm text-white"
            >
              <option value="pnl">Sort by P&L</option>
              <option value="winRate">Sort by Win Rate</option>
              <option value="trades">Sort by Trades</option>
            </select>
          </div>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-700/50">
              <tr>
                <th className="text-left p-3 text-sm text-gray-400 font-medium">Strategy</th>
                <th className="text-center p-3 text-sm text-gray-400 font-medium">Status</th>
                <th className="text-center p-3 text-sm text-gray-400 font-medium">Trades</th>
                <th className="text-center p-3 text-sm text-gray-400 font-medium">Win Rate</th>
                <th className="text-right p-3 text-sm text-gray-400 font-medium">Total P&L</th>
                <th className="text-right p-3 text-sm text-gray-400 font-medium">Avg Trade</th>
                <th className="text-center p-3 text-sm text-gray-400 font-medium">Trend</th>
                <th className="text-center p-3 text-sm text-gray-400 font-medium">P.Factor</th>
              </tr>
            </thead>
            <tbody>
              {sortedPerformances.length === 0 ? (
                <tr>
                  <td colSpan={8} className="p-8 text-center text-gray-400">
                    No strategy performance data available
                  </td>
                </tr>
              ) : (
                sortedPerformances.map((perf) => (
                  <React.Fragment key={perf.strategyName}>
                    <tr
                      className="border-t border-gray-700 hover:bg-gray-700/30 cursor-pointer transition-colors"
                      onClick={() => setExpandedStrategy(
                        expandedStrategy === perf.strategyName ? null : perf.strategyName
                      )}
                    >
                      <td className="p-3">
                        <div className="flex items-center gap-2">
                          <ChevronDown
                            className={`w-4 h-4 text-gray-400 transition-transform ${
                              expandedStrategy === perf.strategyName ? 'rotate-180' : ''
                            }`}
                          />
                          <span className="font-medium text-white">{perf.strategyName}</span>
                        </div>
                      </td>
                      <td className="p-3 text-center">
                        <span className={`px-2 py-1 rounded text-xs ${
                          perf.status === 'active'
                            ? 'bg-green-600/30 text-green-400'
                            : perf.status === 'paused'
                            ? 'bg-yellow-600/30 text-yellow-400'
                            : 'bg-gray-600/30 text-gray-400'
                        }`}>
                          {perf.status}
                        </span>
                      </td>
                      <td className="p-3 text-center">
                        <span className="text-white">{perf.totalTrades}</span>
                        <span className="text-xs text-gray-400 ml-1">
                          ({perf.winningTrades}W/{perf.losingTrades}L)
                        </span>
                      </td>
                      <td className="p-3">
                        <PerformanceBar winRate={perf.winRate} />
                      </td>
                      <td className={`p-3 text-right font-medium ${
                        perf.totalPnL >= 0 ? 'text-green-400' : 'text-red-400'
                      }`}>
                        {perf.totalPnL >= 0 ? '+' : ''}{perf.totalPnL.toFixed(2)}
                      </td>
                      <td className={`p-3 text-right ${
                        perf.avgPnL >= 0 ? 'text-green-400' : 'text-red-400'
                      }`}>
                        {perf.avgPnL >= 0 ? '+' : ''}{perf.avgPnL.toFixed(2)}
                      </td>
                      <td className="p-3 text-center">
                        <div className="flex items-center justify-center gap-2">
                          {perf.trend === 'up' && <TrendingUp className="w-4 h-4 text-green-400" />}
                          {perf.trend === 'down' && <TrendingDown className="w-4 h-4 text-red-400" />}
                          {perf.trend === 'neutral' && <Activity className="w-4 h-4 text-gray-400" />}
                          <MiniSparkline
                            data={perf.recentPnL}
                            color={perf.trend === 'up' ? '#4ade80' : perf.trend === 'down' ? '#f87171' : '#9ca3af'}
                          />
                        </div>
                      </td>
                      <td className={`p-3 text-center font-medium ${
                        perf.profitFactor >= 1.5 ? 'text-green-400' :
                        perf.profitFactor >= 1 ? 'text-yellow-400' : 'text-red-400'
                      }`}>
                        {perf.profitFactor === Infinity ? '∞' : perf.profitFactor.toFixed(2)}
                      </td>
                    </tr>

                    {/* Expanded Details */}
                    {expandedStrategy === perf.strategyName && (
                      <tr className="bg-gray-700/20">
                        <td colSpan={8} className="p-4">
                          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                            <div className="bg-gray-700/50 rounded-lg p-3">
                              <div className="text-xs text-gray-400 mb-1">Largest Win</div>
                              <div className="text-lg font-medium text-green-400">
                                +${perf.largestWin.toFixed(2)}
                              </div>
                            </div>
                            <div className="bg-gray-700/50 rounded-lg p-3">
                              <div className="text-xs text-gray-400 mb-1">Largest Loss</div>
                              <div className="text-lg font-medium text-red-400">
                                ${perf.largestLoss.toFixed(2)}
                              </div>
                            </div>
                            <div className="bg-gray-700/50 rounded-lg p-3">
                              <div className="text-xs text-gray-400 mb-1">Avg Win</div>
                              <div className="text-lg font-medium text-green-400">
                                +${perf.avgWin.toFixed(2)}
                              </div>
                            </div>
                            <div className="bg-gray-700/50 rounded-lg p-3">
                              <div className="text-xs text-gray-400 mb-1">Avg Loss</div>
                              <div className="text-lg font-medium text-red-400">
                                -${perf.avgLoss.toFixed(2)}
                              </div>
                            </div>
                            <div className="bg-gray-700/50 rounded-lg p-3">
                              <div className="text-xs text-gray-400 mb-1">Max Drawdown</div>
                              <div className="text-lg font-medium text-red-400">
                                ${perf.maxDrawdown.toFixed(2)}
                              </div>
                            </div>
                            <div className="bg-gray-700/50 rounded-lg p-3">
                              <div className="text-xs text-gray-400 mb-1">Win/Loss Ratio</div>
                              <div className="text-lg font-medium text-blue-400">
                                {perf.avgLoss > 0 ? (perf.avgWin / perf.avgLoss).toFixed(2) : 'N/A'}
                              </div>
                            </div>
                            <div className="bg-gray-700/50 rounded-lg p-3 col-span-2">
                              <div className="text-xs text-gray-400 mb-1">Last Trade</div>
                              <div className="text-sm text-white flex items-center gap-2">
                                <Clock className="w-4 h-4 text-gray-400" />
                                {perf.lastTradeTime
                                  ? new Date(perf.lastTradeTime).toLocaleString()
                                  : 'No trades yet'}
                              </div>
                            </div>
                          </div>

                          {/* Risk Assessment */}
                          <div className="mt-4 p-3 bg-gray-700/30 rounded-lg">
                            <h4 className="text-sm font-medium text-white mb-2 flex items-center gap-2">
                              <AlertTriangle className="w-4 h-4 text-yellow-400" />
                              Risk Assessment
                            </h4>
                            <div className="grid grid-cols-3 gap-4 text-sm">
                              <div>
                                <span className="text-gray-400">Expectancy: </span>
                                <span className={perf.avgPnL >= 0 ? 'text-green-400' : 'text-red-400'}>
                                  ${perf.avgPnL.toFixed(2)} per trade
                                </span>
                              </div>
                              <div>
                                <span className="text-gray-400">Risk/Reward: </span>
                                <span className={perf.avgWin / (perf.avgLoss || 1) >= 1.5 ? 'text-green-400' : 'text-yellow-400'}>
                                  {(perf.avgWin / (perf.avgLoss || 1)).toFixed(2)}:1
                                </span>
                              </div>
                              <div>
                                <span className="text-gray-400">Consistency: </span>
                                <span className={perf.winRate >= 50 ? 'text-green-400' : 'text-yellow-400'}>
                                  {perf.winRate >= 60 ? 'High' : perf.winRate >= 45 ? 'Medium' : 'Low'}
                                </span>
                              </div>
                            </div>
                          </div>
                        </td>
                      </tr>
                    )}
                  </React.Fragment>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Performance Tips */}
      <div className="bg-gray-800 rounded-lg border border-gray-700 p-4">
        <h3 className="font-medium text-white mb-3 flex items-center gap-2">
          <Award className="w-5 h-5 text-yellow-400" />
          Performance Insights
        </h3>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {overallMetrics && overallMetrics.overallWinRate < 50 && (
            <div className="p-3 bg-red-600/10 border border-red-600/30 rounded-lg">
              <div className="text-sm text-red-400 font-medium">Low Win Rate</div>
              <div className="text-xs text-gray-400 mt-1">
                Consider reviewing entry conditions or adding confirmation filters.
              </div>
            </div>
          )}
          {performances.some(p => p.profitFactor < 1) && (
            <div className="p-3 bg-yellow-600/10 border border-yellow-600/30 rounded-lg">
              <div className="text-sm text-yellow-400 font-medium">Underperforming Strategy</div>
              <div className="text-xs text-gray-400 mt-1">
                Some strategies have profit factor below 1. Consider pausing or optimizing them.
              </div>
            </div>
          )}
          {performances.some(p => p.maxDrawdown > p.totalPnL * 0.5) && (
            <div className="p-3 bg-orange-600/10 border border-orange-600/30 rounded-lg">
              <div className="text-sm text-orange-400 font-medium">High Drawdown</div>
              <div className="text-xs text-gray-400 mt-1">
                Some strategies have significant drawdown. Review position sizing and stop losses.
              </div>
            </div>
          )}
          {overallMetrics && overallMetrics.totalPnL > 0 && overallMetrics.overallWinRate >= 50 && (
            <div className="p-3 bg-green-600/10 border border-green-600/30 rounded-lg">
              <div className="text-sm text-green-400 font-medium">Positive Performance</div>
              <div className="text-xs text-gray-400 mt-1">
                Your strategies are performing well. Consider increasing position sizes gradually.
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
