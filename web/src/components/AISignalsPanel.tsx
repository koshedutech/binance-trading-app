import { useState, useEffect } from 'react';
import { formatDistanceToNow } from 'date-fns';
import { Brain, TrendingUp, TrendingDown, Minus, Activity, BarChart3, MessageSquare, Zap, RefreshCw } from 'lucide-react';

interface AIDecision {
  id: number;
  symbol: string;
  current_price: number;
  action: string;
  confidence: number;
  reasoning: string;
  signals: Record<string, { direction: string; confidence: number; reason: string }>;
  ml_direction?: string;
  ml_confidence?: number;
  sentiment_direction?: string;
  sentiment_confidence?: number;
  llm_direction?: string;
  llm_confidence?: number;
  pattern_direction?: string;
  pattern_confidence?: number;
  bigcandle_direction?: string;
  bigcandle_confidence?: number;
  confluence_count: number;
  risk_level: string;
  executed: boolean;
  created_at: string;
}

interface AIStats {
  total: number;
  buy_decisions: number;
  sell_decisions: number;
  hold_decisions: number;
  executed: number;
  avg_confidence: number | null;
  avg_confluence: number | null;
}

const getDirectionIcon = (direction?: string) => {
  if (!direction) return <Minus className="w-3 h-3 text-gray-400" />;
  if (direction === 'long' || direction === 'up' || direction === 'bullish') {
    return <TrendingUp className="w-3 h-3 text-green-400" />;
  }
  if (direction === 'short' || direction === 'down' || direction === 'bearish') {
    return <TrendingDown className="w-3 h-3 text-red-400" />;
  }
  return <Minus className="w-3 h-3 text-gray-400" />;
};

const getDirectionColor = (direction?: string) => {
  if (!direction) return 'text-gray-400';
  if (direction === 'long' || direction === 'up' || direction === 'bullish') return 'text-green-400';
  if (direction === 'short' || direction === 'down' || direction === 'bearish') return 'text-red-400';
  return 'text-gray-400';
};

const getActionBadge = (action: string) => {
  switch (action) {
    case 'buy':
      return <span className="badge badge-success">BUY</span>;
    case 'sell':
      return <span className="badge badge-danger">SELL</span>;
    default:
      return <span className="badge bg-gray-600">HOLD</span>;
  }
};

const SignalSourceBadge = ({
  label,
  direction,
  confidence,
  icon: Icon
}: {
  label: string;
  direction?: string;
  confidence?: number;
  icon: React.ComponentType<{ className?: string }>;
}) => {
  if (!direction && !confidence) return null;

  return (
    <div className="flex items-center space-x-1 bg-dark-700 rounded px-2 py-1 text-xs">
      <Icon className="w-3 h-3 text-gray-400" />
      <span className="text-gray-400">{label}:</span>
      {getDirectionIcon(direction)}
      <span className={getDirectionColor(direction)}>
        {direction || 'N/A'}
      </span>
      {confidence !== undefined && confidence !== null && (
        <span className="text-gray-500">({(confidence * 100).toFixed(0)}%)</span>
      )}
    </div>
  );
};

export default function AISignalsPanel() {
  const [decisions, setDecisions] = useState<AIDecision[]>([]);
  const [stats, setStats] = useState<AIStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState<'all' | 'buy' | 'sell' | 'hold'>('all');

  const fetchData = async () => {
    try {
      setLoading(true);
      const [decisionsRes, statsRes] = await Promise.all([
        fetch('/api/ai-decisions?limit=100'),
        fetch('/api/ai-decisions/stats?hours=24')
      ]);

      if (!decisionsRes.ok || !statsRes.ok) {
        throw new Error('Failed to fetch AI decisions');
      }

      const decisionsData = await decisionsRes.json();
      const statsData = await statsRes.json();

      setDecisions(decisionsData.data || []);
      setStats(statsData.data || null);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load AI decisions');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 20000); // Reduced from 10s to 20s
    return () => clearInterval(interval);
  }, []);

  const filteredDecisions = decisions.filter(d => {
    if (filter === 'all') return true;
    return d.action === filter;
  });

  if (loading && decisions.length === 0) {
    return (
      <div className="p-8 text-center text-gray-400">
        <RefreshCw className="w-6 h-6 animate-spin mx-auto mb-2" />
        Loading AI decisions...
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-8 text-center text-red-400">
        {error}
        <button onClick={fetchData} className="btn btn-sm btn-primary mt-2">
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Stats Header */}
      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 p-4 bg-dark-800 rounded-lg">
          <div className="text-center">
            <div className="text-2xl font-bold text-blue-400">{stats.total}</div>
            <div className="text-xs text-gray-400">Total Decisions</div>
          </div>
          <div className="text-center">
            <div className="text-2xl font-bold text-green-400">{stats.buy_decisions}</div>
            <div className="text-xs text-gray-400">Buy Signals</div>
          </div>
          <div className="text-center">
            <div className="text-2xl font-bold text-red-400">{stats.sell_decisions}</div>
            <div className="text-xs text-gray-400">Sell Signals</div>
          </div>
          <div className="text-center">
            <div className="text-2xl font-bold text-yellow-400">
              {stats.avg_confidence ? (stats.avg_confidence * 100).toFixed(0) : 0}%
            </div>
            <div className="text-xs text-gray-400">Avg Confidence</div>
          </div>
        </div>
      )}

      {/* Filter Tabs */}
      <div className="flex space-x-2 px-4">
        {(['all', 'buy', 'sell', 'hold'] as const).map((f) => (
          <button
            key={f}
            onClick={() => setFilter(f)}
            className={`px-3 py-1 rounded text-sm capitalize ${
              filter === f
                ? 'bg-primary-600 text-white'
                : 'bg-dark-700 text-gray-400 hover:bg-dark-600'
            }`}
          >
            {f}
          </button>
        ))}
        <button
          onClick={fetchData}
          className="ml-auto px-3 py-1 rounded text-sm bg-dark-700 text-gray-400 hover:bg-dark-600"
        >
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {/* Decisions List */}
      <div className="divide-y divide-dark-700 max-h-[600px] overflow-y-auto scrollbar-thin">
        {filteredDecisions.length === 0 ? (
          <div className="p-8 text-center text-gray-400">
            <Brain className="w-12 h-12 mx-auto mb-2 opacity-50" />
            No AI decisions yet. Autopilot is analyzing markets...
          </div>
        ) : (
          filteredDecisions.map((decision) => (
            <div key={decision.id} className="p-4 hover:bg-dark-750 transition-colors">
              {/* Header Row */}
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center space-x-3">
                  <Brain className="w-5 h-5 text-purple-400" />
                  <span className="font-semibold text-lg">{decision.symbol}</span>
                  {getActionBadge(decision.action)}
                  {decision.executed && (
                    <span className="badge badge-success text-xs">Executed</span>
                  )}
                </div>
                <div className="text-right">
                  <div className="text-sm font-mono">${decision.current_price.toFixed(2)}</div>
                  <div className="text-xs text-gray-500">
                    {formatDistanceToNow(new Date(decision.created_at), { addSuffix: true })}
                  </div>
                </div>
              </div>

              {/* Reasoning */}
              <div className="text-sm text-gray-300 mb-3 bg-dark-700 rounded p-2">
                {decision.reasoning}
              </div>

              {/* Signal Sources */}
              <div className="flex flex-wrap gap-2 mb-2">
                <SignalSourceBadge
                  label="ML"
                  direction={decision.ml_direction}
                  confidence={decision.ml_confidence}
                  icon={BarChart3}
                />
                <SignalSourceBadge
                  label="Sentiment"
                  direction={decision.sentiment_direction}
                  confidence={decision.sentiment_confidence}
                  icon={Activity}
                />
                <SignalSourceBadge
                  label="LLM"
                  direction={decision.llm_direction}
                  confidence={decision.llm_confidence}
                  icon={MessageSquare}
                />
                <SignalSourceBadge
                  label="BigCandle"
                  direction={decision.bigcandle_direction}
                  confidence={decision.bigcandle_confidence}
                  icon={Zap}
                />
                <SignalSourceBadge
                  label="Pattern"
                  direction={decision.pattern_direction}
                  confidence={decision.pattern_confidence}
                  icon={TrendingUp}
                />
              </div>

              {/* Meta Info */}
              <div className="flex items-center space-x-4 text-xs text-gray-500">
                <span>Confluence: {decision.confluence_count}</span>
                <span>Confidence: {(decision.confidence * 100).toFixed(0)}%</span>
                <span className="capitalize">Risk: {decision.risk_level}</span>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
