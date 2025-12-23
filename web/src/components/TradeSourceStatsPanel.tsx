import { useState, useEffect } from 'react';
import { futuresApi, formatUSD, getPositionColor } from '../services/futuresApi';
import { TradeSourceStats } from '../types/futures';
import { Brain, Zap, User, Target, AlertTriangle, TrendingUp, TrendingDown, RefreshCw } from 'lucide-react';

interface TradeSourceStatsData {
  ai: TradeSourceStats;
  strategy: TradeSourceStats;
  manual: TradeSourceStats;
}

export default function TradeSourceStatsPanel() {
  const [stats, setStats] = useState<TradeSourceStatsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchStats = async () => {
    try {
      setLoading(true);
      const data = await futuresApi.getTradeSourceStats();
      setStats(data);
      setError(null);
    } catch (err) {
      console.error('Failed to fetch trade source stats:', err);
      setError('Failed to load stats');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStats();
    const interval = setInterval(fetchStats, 60000); // Refresh every minute
    return () => clearInterval(interval);
  }, []);

  if (loading && !stats) {
    return (
      <div className="bg-gray-900 rounded-lg border border-gray-700 p-4">
        <div className="flex items-center gap-2 mb-4">
          <Brain className="w-5 h-5 text-purple-400" />
          <h2 className="text-lg font-semibold">Trade Source Performance</h2>
        </div>
        <div className="text-center text-gray-400 py-4">Loading stats...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-gray-900 rounded-lg border border-gray-700 p-4">
        <div className="flex items-center gap-2 mb-4">
          <Brain className="w-5 h-5 text-purple-400" />
          <h2 className="text-lg font-semibold">Trade Source Performance</h2>
        </div>
        <div className="text-center text-red-400 py-4">{error}</div>
      </div>
    );
  }

  return (
    <div className="bg-gray-900 rounded-lg border border-gray-700 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-700">
        <div className="flex items-center gap-2">
          <Brain className="w-5 h-5 text-purple-400" />
          <h2 className="text-lg font-semibold">Trade Source Performance</h2>
        </div>
        <button
          onClick={fetchStats}
          className="p-1.5 hover:bg-gray-700 rounded transition-colors"
        >
          <RefreshCw className={`w-4 h-4 text-gray-400 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-3 divide-x divide-gray-700">
        {/* AI Autopilot Stats */}
        <SourceCard
          title="AI Autopilot"
          icon={Brain}
          iconColor="text-purple-400"
          bgColor="bg-purple-500/10"
          stats={stats?.ai}
        />

        {/* Strategy Stats */}
        <SourceCard
          title="Strategy"
          icon={Zap}
          iconColor="text-yellow-400"
          bgColor="bg-yellow-500/10"
          stats={stats?.strategy}
        />

        {/* Manual Stats */}
        <SourceCard
          title="Manual"
          icon={User}
          iconColor="text-blue-400"
          bgColor="bg-blue-500/10"
          stats={stats?.manual}
        />
      </div>
    </div>
  );
}

function SourceCard({
  title,
  icon: Icon,
  iconColor,
  bgColor,
  stats,
}: {
  title: string;
  icon: any;
  iconColor: string;
  bgColor: string;
  stats?: TradeSourceStats;
}) {
  const totalTrades = stats?.totalTrades || 0;
  const winRate = stats?.winRate || 0;
  const totalPnl = stats?.totalPnl || 0;
  const tpHits = stats?.tpHits || 0;
  const slHits = stats?.slHits || 0;

  return (
    <div className={`p-4 ${bgColor}`}>
      {/* Title */}
      <div className="flex items-center gap-2 mb-3">
        <Icon className={`w-5 h-5 ${iconColor}`} />
        <span className="font-semibold text-white">{title}</span>
      </div>

      {/* Main Stats */}
      <div className="space-y-2">
        {/* Total Trades */}
        <div className="flex justify-between text-sm">
          <span className="text-gray-400">Total Trades</span>
          <span className="font-medium text-white">{totalTrades}</span>
        </div>

        {/* Win Rate */}
        <div className="flex justify-between text-sm">
          <span className="text-gray-400">Win Rate</span>
          <span className={`font-medium ${winRate >= 50 ? 'text-green-500' : 'text-red-500'}`}>
            {Number(winRate || 0).toFixed(1)}%
          </span>
        </div>

        {/* Total PnL */}
        <div className="flex justify-between text-sm">
          <span className="text-gray-400">Total PnL</span>
          <span className={`font-medium ${getPositionColor(totalPnl)}`}>
            {formatUSD(totalPnl)}
          </span>
        </div>

        {/* TP/SL Hits */}
        <div className="flex justify-between text-sm border-t border-gray-700 pt-2 mt-2">
          <div className="flex items-center gap-1">
            <Target className="w-3 h-3 text-green-500" />
            <span className="text-gray-400">TP Hits</span>
          </div>
          <span className="font-medium text-green-500">{tpHits}</span>
        </div>

        <div className="flex justify-between text-sm">
          <div className="flex items-center gap-1">
            <AlertTriangle className="w-3 h-3 text-red-500" />
            <span className="text-gray-400">SL Hits</span>
          </div>
          <span className="font-medium text-red-500">{slHits}</span>
        </div>

        {/* Win/Loss Count */}
        <div className="flex justify-between text-sm border-t border-gray-700 pt-2 mt-2">
          <div className="flex items-center gap-1">
            <TrendingUp className="w-3 h-3 text-green-500" />
            <span className="text-gray-400">Wins</span>
          </div>
          <span className="font-medium text-green-500">{stats?.winningTrades || 0}</span>
        </div>

        <div className="flex justify-between text-sm">
          <div className="flex items-center gap-1">
            <TrendingDown className="w-3 h-3 text-red-500" />
            <span className="text-gray-400">Losses</span>
          </div>
          <span className="font-medium text-red-500">{stats?.losingTrades || 0}</span>
        </div>
      </div>
    </div>
  );
}
