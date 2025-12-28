import { useState, useEffect } from 'react';
import { formatDistanceToNow } from 'date-fns';
import { Brain, TrendingUp, TrendingDown, Minus, RefreshCw, Zap, AlertTriangle } from 'lucide-react';

interface FuturesAIDecision {
  symbol: string;
  action: string;
  confidence: number;
  reasoning: string;
  signals: Record<string, { direction: string; confidence: number; reason: string }>;
  created_at: string;
  executed?: boolean;
}

const getActionColor = (action: string) => {
  switch (action) {
    case 'open_long':
      return 'text-green-500 bg-green-500/20';
    case 'open_short':
      return 'text-red-500 bg-red-500/20';
    case 'close':
      return 'text-yellow-500 bg-yellow-500/20';
    default:
      return 'text-gray-500 bg-gray-500/20';
  }
};

const getActionLabel = (action: string) => {
  switch (action) {
    case 'open_long':
      return 'LONG';
    case 'open_short':
      return 'SHORT';
    case 'close':
      return 'CLOSE';
    default:
      return 'HOLD';
  }
};

const getActionIcon = (action: string) => {
  switch (action) {
    case 'open_long':
      return <TrendingUp className="w-3 h-3" />;
    case 'open_short':
      return <TrendingDown className="w-3 h-3" />;
    case 'close':
      return <AlertTriangle className="w-3 h-3" />;
    default:
      return <Minus className="w-3 h-3" />;
  }
};

export default function FuturesAISignals() {
  const [decisions, setDecisions] = useState<FuturesAIDecision[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchDecisions = async () => {
    try {
      const token = localStorage.getItem('access_token');
      const headers: HeadersInit = token ? { Authorization: `Bearer ${token}` } : {};

      // Fetch from the existing AI decisions endpoint, which includes futures decisions
      const res = await fetch('/api/ai-decisions?limit=50', { headers });

      // Silently handle 401 errors
      if (res.status === 401) {
        console.warn('FuturesAISignals: Not authenticated');
        setDecisions([]);
        setError(null);
        return;
      }

      if (!res.ok) throw new Error('Failed to fetch');
      const data = await res.json();

      // Filter for futures-related decisions (actions contain open_long, open_short, close)
      const futuresDecisions = (data.data || []).filter((d: any) =>
        ['open_long', 'open_short', 'close', 'hold'].includes(d.action)
      );

      setDecisions(futuresDecisions.slice(0, 20));
      setError(null);
    } catch (err) {
      console.error('Failed to fetch AI decisions:', err);
      // Don't show error for background fetches
      if (decisions.length === 0) {
        setError('No AI signals available');
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDecisions();
    const interval = setInterval(fetchDecisions, 20000); // Reduced from 10s to 20s
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="bg-gray-900 rounded-lg border border-gray-700 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-gray-700">
        <div className="flex items-center gap-2">
          <Brain className="w-4 h-4 text-purple-400" />
          <span className="text-sm font-semibold">AI Signals</span>
        </div>
        <button
          onClick={fetchDecisions}
          disabled={loading}
          className="p-1 hover:bg-gray-700 rounded"
          title="Refresh"
        >
          <RefreshCw className={`w-4 h-4 text-gray-400 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {/* Content */}
      <div className="max-h-[400px] overflow-y-auto">
        {loading && decisions.length === 0 ? (
          <div className="p-4 text-center text-gray-500 text-sm">
            <RefreshCw className="w-5 h-5 animate-spin mx-auto mb-2" />
            Loading AI signals...
          </div>
        ) : error && decisions.length === 0 ? (
          <div className="p-4 text-center text-gray-500 text-sm">
            <Brain className="w-8 h-8 mx-auto mb-2 opacity-40" />
            {error}
          </div>
        ) : decisions.length === 0 ? (
          <div className="p-4 text-center text-gray-500 text-sm">
            <Brain className="w-8 h-8 mx-auto mb-2 opacity-40" />
            No AI signals yet
          </div>
        ) : (
          <div className="divide-y divide-gray-800">
            {decisions.map((decision, idx) => (
              <div key={idx} className="p-3 hover:bg-gray-800/50">
                {/* Header row */}
                <div className="flex items-center justify-between mb-1">
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-sm">{decision.symbol}</span>
                    <span className={`flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${getActionColor(decision.action)}`}>
                      {getActionIcon(decision.action)}
                      {getActionLabel(decision.action)}
                    </span>
                    {decision.executed && (
                      <span title="Executed">
                        <Zap className="w-3 h-3 text-yellow-500" />
                      </span>
                    )}
                  </div>
                  <span className="text-xs text-gray-500">
                    {(decision.confidence * 100).toFixed(0)}%
                  </span>
                </div>

                {/* Reasoning */}
                <p className="text-xs text-gray-400 line-clamp-2 mb-1">
                  {decision.reasoning}
                </p>

                {/* Time */}
                <div className="text-xs text-gray-500">
                  {formatDistanceToNow(new Date(decision.created_at), { addSuffix: true })}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
