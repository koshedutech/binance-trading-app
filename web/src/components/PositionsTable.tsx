import { useState, useEffect } from 'react';
import { useStore } from '../store';
import { X, Brain, ChevronDown, ChevronRight, TrendingUp, TrendingDown, Minus, Cog, User, Zap, AlertCircle } from 'lucide-react';
import { apiService } from '../services/api';
import { formatDistanceToNow } from 'date-fns';

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
  created_at: string;
}

interface Position {
  symbol: string;
  side: string;
  entry_price: number;
  current_price?: number;
  quantity: number;
  pnl?: number;
  pnl_percent?: number;
  entry_time: string;
  ai_decision_id?: number;
  ai_decision?: AIDecision;
  trailing_stop_enabled?: boolean;
  trailing_stop_percent?: number;
  stop_loss?: number;
  take_profit?: number;
  trade_source?: string; // 'manual' | 'strategy' | 'ai'
}

const TradeSourceBadge = ({ source }: { source?: string }) => {
  if (!source || source === 'manual') {
    return (
      <div className="flex items-center space-x-1" title="Manual trade">
        <User className="w-4 h-4 text-gray-400" />
        <span className="text-xs text-gray-400">Manual</span>
      </div>
    );
  }

  if (source === 'ai') {
    return (
      <div className="flex items-center space-x-1" title="AI autopilot trade">
        <Brain className="w-4 h-4 text-purple-500" />
        <span className="text-xs text-purple-400">AI</span>
      </div>
    );
  }

  if (source === 'strategy') {
    return (
      <div className="flex items-center space-x-1" title="Strategy trade">
        <Cog className="w-4 h-4 text-blue-500" />
        <span className="text-xs text-blue-400">Strategy</span>
      </div>
    );
  }

  return null;
};

const SignalBadge = ({ direction, confidence }: { direction?: string; confidence?: number }) => {
  if (!direction) return <span className="text-gray-500 text-xs">N/A</span>;

  const isLong = direction === 'long' || direction === 'up';
  const isShort = direction === 'short' || direction === 'down';

  return (
    <div className="flex items-center space-x-1">
      {isLong ? (
        <TrendingUp className="w-3 h-3 text-green-500" />
      ) : isShort ? (
        <TrendingDown className="w-3 h-3 text-red-500" />
      ) : (
        <Minus className="w-3 h-3 text-gray-500" />
      )}
      <span className={`text-xs font-medium ${isLong ? 'text-green-500' : isShort ? 'text-red-500' : 'text-gray-500'}`}>
        {direction}
      </span>
      {confidence !== undefined && (
        <span className="text-xs text-gray-400">
          ({(confidence * 100).toFixed(0)}%)
        </span>
      )}
    </div>
  );
};

// Trading Mode Badge Component
const TradingModeBadge = ({ mode }: { mode: 'live' | 'paper' | null }) => {
  if (!mode) return null;

  if (mode === 'live') {
    return (
      <div className="flex items-center gap-1 px-2 py-0.5 rounded text-xs font-bold bg-green-500/20 border border-green-500 text-green-400">
        <Zap className="w-3 h-3" />
        LIVE
      </div>
    );
  }

  return (
    <div className="flex items-center gap-1 px-2 py-0.5 rounded text-xs font-bold bg-yellow-500/20 border border-yellow-500 text-yellow-400">
      <AlertCircle className="w-3 h-3" />
      PAPER
    </div>
  );
};

export default function PositionsTable() {
  const { positions } = useStore();
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set());
  const [aiDecisions, setAiDecisions] = useState<Record<string, AIDecision>>({});
  const [loadingAI, setLoadingAI] = useState<Set<string>>(new Set());
  const [tradingMode, setTradingMode] = useState<'live' | 'paper' | null>(null);

  // Fetch trading mode
  useEffect(() => {
    const fetchTradingMode = async () => {
      try {
        const modeData = await apiService.getTradingMode();
        setTradingMode(modeData.mode === 'live' ? 'live' : 'paper');
      } catch (error) {
        console.error('Error fetching trading mode:', error);
      }
    };
    fetchTradingMode();
  }, []);

  const formatCurrency = (value: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
    }).format(value);
  };

  const toggleRow = async (symbol: string, aiDecisionId?: number) => {
    const newExpanded = new Set(expandedRows);
    if (newExpanded.has(symbol)) {
      newExpanded.delete(symbol);
    } else {
      newExpanded.add(symbol);
      // Load AI decision if not already loaded
      if (aiDecisionId && !aiDecisions[symbol]) {
        setLoadingAI(prev => new Set(prev).add(symbol));
        try {
          const response = await apiService.get(`/api/ai/decisions/${aiDecisionId}`);
          setAiDecisions(prev => ({ ...prev, [symbol]: response.data }));
        } catch (error) {
          console.error('Failed to load AI decision:', error);
        } finally {
          setLoadingAI(prev => {
            const next = new Set(prev);
            next.delete(symbol);
            return next;
          });
        }
      }
    }
    setExpandedRows(newExpanded);
  };

  const handleClosePosition = async (symbol: string) => {
    if (!confirm(`Are you sure you want to close position for ${symbol}?`)) {
      return;
    }

    try {
      await apiService.closePosition(symbol);
      alert('Position closed successfully');
    } catch (error) {
      alert('Failed to close position');
      console.error(error);
    }
  };

  if (positions.length === 0) {
    return (
      <div className="p-8 text-center text-gray-400">
        No open positions
      </div>
    );
  }

  const renderAIDecisionDetails = (position: Position) => {
    const ai = position.ai_decision || aiDecisions[position.symbol];
    const isLoading = loadingAI.has(position.symbol);

    if (isLoading) {
      return (
        <tr>
          <td colSpan={11} className="bg-gray-800/50 p-4">
            <div className="flex items-center justify-center space-x-2 text-gray-400">
              <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-500"></div>
              <span>Loading AI decision details...</span>
            </div>
          </td>
        </tr>
      );
    }

    if (!ai) {
      return (
        <tr>
          <td colSpan={11} className="bg-gray-800/50 p-4">
            <div className="text-gray-400 text-center">
              No AI decision data available for this position
            </div>
          </td>
        </tr>
      );
    }

    return (
      <tr>
        <td colSpan={11} className="bg-gray-800/50 p-0">
          <div className="p-4 space-y-4">
            {/* AI Decision Header */}
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-2">
                <Brain className="w-5 h-5 text-purple-500" />
                <span className="font-semibold text-purple-400">AI Decision Analysis</span>
              </div>
              <div className="flex items-center space-x-4 text-sm">
                <span className="text-gray-400">
                  Confidence: <span className={ai.confidence >= 0.7 ? 'text-green-500' : ai.confidence >= 0.5 ? 'text-yellow-500' : 'text-red-500'}>
                    {(ai.confidence * 100).toFixed(0)}%
                  </span>
                </span>
                <span className="text-gray-400">
                  Confluence: <span className="text-blue-400">{ai.confluence_count} signals</span>
                </span>
                <span className="text-gray-400">
                  Risk: <span className={ai.risk_level === 'conservative' ? 'text-green-500' : ai.risk_level === 'aggressive' ? 'text-red-500' : 'text-yellow-500'}>
                    {ai.risk_level}
                  </span>
                </span>
              </div>
            </div>

            {/* Reasoning */}
            {ai.reasoning && (
              <div className="bg-gray-900/50 rounded-lg p-3">
                <div className="text-xs text-gray-500 mb-1">Decision Reasoning</div>
                <p className="text-sm text-gray-300">{ai.reasoning}</p>
              </div>
            )}

            {/* Signal Sources Grid */}
            <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
              {/* ML Signal */}
              <div className="bg-gray-900/50 rounded-lg p-3">
                <div className="text-xs text-gray-500 mb-2">ML Prediction</div>
                <SignalBadge
                  direction={ai.ml_direction}
                  confidence={ai.ml_confidence}
                />
                {ai.signals?.ml?.reason && (
                  <p className="text-xs text-gray-500 mt-1 truncate" title={ai.signals.ml.reason}>
                    {ai.signals.ml.reason}
                  </p>
                )}
              </div>

              {/* Sentiment Signal */}
              <div className="bg-gray-900/50 rounded-lg p-3">
                <div className="text-xs text-gray-500 mb-2">Sentiment</div>
                <SignalBadge
                  direction={ai.sentiment_direction}
                  confidence={ai.sentiment_confidence}
                />
                {ai.signals?.sentiment?.reason && (
                  <p className="text-xs text-gray-500 mt-1 truncate" title={ai.signals.sentiment.reason}>
                    {ai.signals.sentiment.reason}
                  </p>
                )}
              </div>

              {/* LLM Signal */}
              <div className="bg-gray-900/50 rounded-lg p-3">
                <div className="text-xs text-gray-500 mb-2">LLM Analysis</div>
                <SignalBadge
                  direction={ai.llm_direction}
                  confidence={ai.llm_confidence}
                />
                {ai.signals?.llm?.reason && (
                  <p className="text-xs text-gray-500 mt-1 truncate" title={ai.signals.llm.reason}>
                    {ai.signals.llm.reason}
                  </p>
                )}
              </div>

              {/* Pattern Signal */}
              <div className="bg-gray-900/50 rounded-lg p-3">
                <div className="text-xs text-gray-500 mb-2">Pattern Detection</div>
                <SignalBadge
                  direction={ai.pattern_direction}
                  confidence={ai.pattern_confidence}
                />
                {ai.signals?.pattern?.reason && (
                  <p className="text-xs text-gray-500 mt-1 truncate" title={ai.signals.pattern.reason}>
                    {ai.signals.pattern.reason}
                  </p>
                )}
              </div>

              {/* Big Candle Signal */}
              <div className="bg-gray-900/50 rounded-lg p-3">
                <div className="text-xs text-gray-500 mb-2">Big Candle</div>
                <SignalBadge
                  direction={ai.bigcandle_direction}
                  confidence={ai.bigcandle_confidence}
                />
                {ai.signals?.big_candle?.reason && (
                  <p className="text-xs text-gray-500 mt-1 truncate" title={ai.signals.big_candle.reason}>
                    {ai.signals.big_candle.reason}
                  </p>
                )}
              </div>
            </div>

            {/* Order Protection Info */}
            {(position.stop_loss || position.take_profit || position.trailing_stop_enabled) && (
              <div className="border-t border-gray-700 pt-3">
                <div className="text-xs text-gray-500 mb-2">Order Protection</div>
                <div className="flex flex-wrap gap-4 text-sm">
                  {position.stop_loss && (
                    <div className="flex items-center space-x-2">
                      <span className="text-gray-400">Stop Loss:</span>
                      <span className="text-red-400">{formatCurrency(position.stop_loss)}</span>
                    </div>
                  )}
                  {position.take_profit && (
                    <div className="flex items-center space-x-2">
                      <span className="text-gray-400">Take Profit:</span>
                      <span className="text-green-400">{formatCurrency(position.take_profit)}</span>
                    </div>
                  )}
                  {position.trailing_stop_enabled && (
                    <div className="flex items-center space-x-2">
                      <span className="text-gray-400">Trailing Stop:</span>
                      <span className="text-yellow-400">{position.trailing_stop_percent}%</span>
                    </div>
                  )}
                </div>
              </div>
            )}

            {/* Decision Time */}
            <div className="text-xs text-gray-500 text-right">
              Decision made {formatDistanceToNow(new Date(ai.created_at), { addSuffix: true })} at ${ai.current_price.toFixed(2)}
            </div>
          </div>
        </td>
      </tr>
    );
  };

  return (
    <div className="overflow-x-auto">
      <table className="table">
        <thead>
          <tr>
            <th className="w-8"></th>
            <th>Symbol</th>
            <th>Mode</th>
            <th>Source</th>
            <th>Side</th>
            <th>Entry Price</th>
            <th>Current Price</th>
            <th>Quantity</th>
            <th>P&L</th>
            <th>Duration</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {positions.map((position: Position) => (
            <>
              <tr
                key={position.symbol}
                className={`${position.ai_decision_id ? 'cursor-pointer hover:bg-gray-800/50' : ''}`}
                onClick={() => position.ai_decision_id && toggleRow(position.symbol, position.ai_decision_id)}
              >
                <td className="w-8">
                  {position.ai_decision_id ? (
                    <div className="flex items-center">
                      {expandedRows.has(position.symbol) ? (
                        <ChevronDown className="w-4 h-4 text-gray-400" />
                      ) : (
                        <ChevronRight className="w-4 h-4 text-gray-400" />
                      )}
                    </div>
                  ) : null}
                </td>
                <td className="font-medium">{position.symbol}</td>
                <td>
                  <TradingModeBadge mode={tradingMode} />
                </td>
                <td>
                  <TradeSourceBadge source={position.trade_source} />
                </td>
                <td>
                  <span
                    className={`badge ${
                      position.side === 'BUY' ? 'badge-success' : 'badge-danger'
                    }`}
                  >
                    {position.side}
                  </span>
                </td>
                <td>{formatCurrency(position.entry_price)}</td>
                <td>{position.current_price ? formatCurrency(position.current_price) : '-'}</td>
                <td>{position.quantity}</td>
                <td>
                  {position.pnl !== undefined ? (
                    <div
                      className={position.pnl >= 0 ? 'text-positive' : 'text-negative'}
                    >
                      <div className="font-semibold">{formatCurrency(position.pnl)}</div>
                      <div className="text-xs">
                        {position.pnl_percent !== undefined
                          ? `${position.pnl_percent >= 0 ? '+' : ''}${position.pnl_percent.toFixed(2)}%`
                          : ''}
                      </div>
                    </div>
                  ) : (
                    '-'
                  )}
                </td>
                <td className="text-sm">
                  {formatDistanceToNow(new Date(position.entry_time), { addSuffix: true })}
                </td>
                <td>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      handleClosePosition(position.symbol);
                    }}
                    className="btn-danger text-xs py-1 px-2 flex items-center space-x-1"
                  >
                    <X className="w-3 h-3" />
                    <span>Close</span>
                  </button>
                </td>
              </tr>
              {expandedRows.has(position.symbol) && renderAIDecisionDetails(position)}
            </>
          ))}
        </tbody>
      </table>
    </div>
  );
}
