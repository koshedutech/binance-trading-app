import React, { useState, useEffect } from 'react';
import { futuresApi, formatUSD, formatPrice, formatQuantity, getPositionColor } from '../services/futuresApi';
import { formatDistanceToNow } from 'date-fns';
import {
  FileText,
  History,
  RefreshCw,
  X,
  Target,
  Shield,
  Clock,
  AlertTriangle,
  Brain,
  ChevronDown,
  ChevronRight,
  Zap,
  MessageSquare,
  BarChart3,
  Sparkles,
  Activity,
  GitBranch,
} from 'lucide-react';
import TradeLifecycleEvents from './TradeLifecycleEvents';

type TabType = 'orders' | 'history' | 'ai_trades' | 'lifecycle';

interface RegularOrder {
  orderId: number;
  symbol: string;
  side: string;
  positionSide: string;
  type: string;
  price: number;
  origQty: number;
  executedQty: number;
  status: string;
  time: number;
  stopPrice?: number;
}

interface AlgoOrder {
  algoId: number;
  symbol: string;
  side: string;
  positionSide: string;
  quantity: string;
  executedQty: string;
  price: string;
  triggerPrice: string;
  createTime: number;
  updateTime: number;
  orderType: string;
  algoType: string;
  algoStatus: string;
  closePosition: boolean;
  reduceOnly: boolean;
}

interface AccountTrade {
  symbol: string;
  id: number;
  orderId: number;
  side: string;
  positionSide: string;
  price: number;
  qty: number;
  realizedPnl: number;
  quoteQty: number;
  commission: number;
  commissionAsset: string;
  time: number;
}

// Type for AI Decision from API
interface AIDecisionFromAPI {
  id: number;
  symbol: string;
  current_price: number;
  action: string;
  confidence: number;
  reasoning: string;
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

export default function FuturesOrdersHistory() {
  const [activeTab, setActiveTab] = useState<TabType>('orders');
  const [loading, setLoading] = useState(false);
  const [regularOrders, setRegularOrders] = useState<RegularOrder[]>([]);
  const [algoOrders, setAlgoOrders] = useState<AlgoOrder[]>([]);
  const [trades, setTrades] = useState<AccountTrade[]>([]);
  const [aiDecisions, setAIDecisions] = useState<AIDecisionFromAPI[]>([]);
  const [limit, setLimit] = useState(20);

  const fetchOrders = async () => {
    setLoading(true);
    try {
      const response = await futuresApi.getAllOrders();
      // Sort by time descending (newest first)
      const sortedRegular = (response.regular_orders || []).sort((a, b) => b.time - a.time);
      const sortedAlgo = (response.algo_orders || []).sort((a, b) => b.createTime - a.createTime);
      setRegularOrders(sortedRegular);
      setAlgoOrders(sortedAlgo);
    } catch (err) {
      console.error('Failed to fetch orders:', err);
    } finally {
      setLoading(false);
    }
  };

  const fetchTrades = async () => {
    setLoading(true);
    try {
      const response = await futuresApi.getAccountTrades(undefined, limit);
      // Sort by time descending (newest first)
      const sortedTrades = (response.trades || []).sort((a, b) => b.time - a.time);
      setTrades(sortedTrades);
    } catch (err) {
      console.error('Failed to fetch trades:', err);
    } finally {
      setLoading(false);
    }
  };

  const fetchAIDecisions = async () => {
    setLoading(true);
    try {
      // Get AI decisions directly from database
      const response = await futuresApi.getAIDecisions(limit);
      setAIDecisions(response.data || []);
    } catch (err) {
      console.error('Failed to fetch AI decisions:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (activeTab === 'orders') {
      fetchOrders();
    } else if (activeTab === 'history') {
      fetchTrades();
    } else if (activeTab === 'ai_trades') {
      fetchAIDecisions();
    }
  }, [activeTab, limit]);

  // Auto-refresh every 30 seconds
  useEffect(() => {
    const interval = setInterval(() => {
      if (activeTab === 'orders') {
        fetchOrders();
      } else if (activeTab === 'history') {
        fetchTrades();
      } else if (activeTab === 'ai_trades') {
        fetchAIDecisions();
      }
    }, 30000);
    return () => clearInterval(interval);
  }, [activeTab]);

  const handleCancelOrder = async (symbol: string, orderId: number) => {
    if (window.confirm('Cancel this order?')) {
      try {
        await futuresApi.cancelOrder(symbol, orderId);
        fetchOrders();
      } catch (err) {
        console.error('Failed to cancel order:', err);
        alert('Failed to cancel order');
      }
    }
  };

  const handleCancelAlgoOrder = async (symbol: string, algoId: number) => {
    if (window.confirm('Cancel this conditional order?')) {
      try {
        await futuresApi.cancelAlgoOrder(symbol, algoId);
        fetchOrders();
      } catch (err) {
        console.error('Failed to cancel algo order:', err);
        alert('Failed to cancel order');
      }
    }
  };

  const tabs = [
    { id: 'orders' as TabType, label: 'Open Orders', icon: FileText, count: regularOrders.length + algoOrders.length },
    { id: 'history' as TabType, label: 'Trade History', icon: History },
    { id: 'ai_trades' as TabType, label: 'AI Decisions', icon: Brain, count: aiDecisions.length > 0 ? aiDecisions.length : undefined },
    { id: 'lifecycle' as TabType, label: 'Trade Events', icon: GitBranch },
  ];

  return (
    <div className="bg-gray-900 rounded-lg border border-gray-700 overflow-hidden">
      {/* Tab Headers */}
      <div className="flex items-center justify-between border-b border-gray-700">
        <div className="flex">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
                activeTab === tab.id
                  ? 'border-yellow-500 text-yellow-500'
                  : 'border-transparent text-gray-400 hover:text-white'
              }`}
            >
              <tab.icon className="w-4 h-4" />
              {tab.label}
              {tab.count !== undefined && tab.count > 0 && (
                <span className="px-1.5 py-0.5 bg-yellow-500/20 text-yellow-500 text-xs rounded">
                  {tab.count}
                </span>
              )}
            </button>
          ))}
        </div>

        <div className="flex items-center gap-2 px-4">
          <select
            value={limit}
            onChange={(e) => setLimit(Number(e.target.value))}
            className="bg-gray-800 border border-gray-700 rounded px-2 py-1 text-xs"
          >
            <option value={10}>10 rows</option>
            <option value={20}>20 rows</option>
            <option value={50}>50 rows</option>
            <option value={100}>100 rows</option>
          </select>

          <button
            onClick={() => {
              if (activeTab === 'orders') fetchOrders();
              else if (activeTab === 'history') fetchTrades();
              else if (activeTab === 'ai_trades') fetchAIDecisions();
            }}
            className="p-1.5 hover:bg-gray-700 rounded"
          >
            <RefreshCw className={`w-4 h-4 text-gray-400 ${loading ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {/* Tab Content */}
      <div className="overflow-x-auto max-h-96">
        {activeTab === 'orders' && (
          <OpenOrdersContent
            regularOrders={regularOrders}
            algoOrders={algoOrders}
            loading={loading}
            onCancelOrder={handleCancelOrder}
            onCancelAlgoOrder={handleCancelAlgoOrder}
          />
        )}
        {activeTab === 'history' && (
          <TradeHistoryContent trades={trades} loading={loading} />
        )}
        {activeTab === 'ai_trades' && (
          <AIDecisionsContent decisions={aiDecisions} loading={loading} />
        )}
        {activeTab === 'lifecycle' && (
          <div className="p-0">
            <TradeLifecycleEvents
              limit={limit}
              compact={true}
              showSummary={false}
              autoRefresh={true}
              refreshInterval={30000}
            />
          </div>
        )}
      </div>
    </div>
  );
}

// Open Orders Content
function OpenOrdersContent({
  regularOrders,
  algoOrders,
  loading,
  onCancelOrder,
  onCancelAlgoOrder,
}: {
  regularOrders: RegularOrder[];
  algoOrders: AlgoOrder[];
  loading: boolean;
  onCancelOrder: (symbol: string, orderId: number) => void;
  onCancelAlgoOrder: (symbol: string, algoId: number) => void;
}) {
  if (loading && regularOrders.length === 0 && algoOrders.length === 0) {
    return (
      <div className="text-center text-gray-400 py-8">
        <RefreshCw className="w-8 h-8 mx-auto mb-2 animate-spin opacity-30" />
        <p>Loading orders...</p>
      </div>
    );
  }

  if (regularOrders.length === 0 && algoOrders.length === 0) {
    return (
      <div className="text-center text-gray-400 py-8">
        <FileText className="w-8 h-8 mx-auto mb-2 opacity-30" />
        <p>No open orders</p>
      </div>
    );
  }

  return (
    <div>
      {/* Regular Orders Section */}
      {regularOrders.length > 0 && (
        <div>
          <div className="px-4 py-2 bg-gray-800 text-sm font-medium text-gray-300 flex items-center gap-2">
            <Clock className="w-4 h-4" />
            Regular Orders ({regularOrders.length})
          </div>
          <table className="w-full text-sm">
            <thead className="bg-gray-800/50">
              <tr className="text-gray-400">
                <th className="text-left py-2 px-4 font-medium">Time</th>
                <th className="text-left py-2 px-4 font-medium">Symbol</th>
                <th className="text-left py-2 px-4 font-medium">Type</th>
                <th className="text-left py-2 px-4 font-medium">Side</th>
                <th className="text-right py-2 px-4 font-medium">Price</th>
                <th className="text-right py-2 px-4 font-medium">Amount</th>
                <th className="text-right py-2 px-4 font-medium">Filled</th>
                <th className="text-center py-2 px-4 font-medium">Action</th>
              </tr>
            </thead>
            <tbody>
              {regularOrders.map((order) => (
                <tr key={order.orderId} className="border-b border-gray-800 hover:bg-gray-800/50">
                  <td className="py-2 px-4 text-gray-400 text-xs">
                    {formatDistanceToNow(new Date(order.time), { addSuffix: true })}
                  </td>
                  <td className="py-2 px-4 font-medium">{order.symbol}</td>
                  <td className="py-2 px-4 text-gray-400">{order.type}</td>
                  <td className={`py-2 px-4 ${order.side === 'BUY' ? 'text-green-500' : 'text-red-500'}`}>
                    {order.side}
                    {order.positionSide !== 'BOTH' && (
                      <span className="text-gray-500 text-xs ml-1">({order.positionSide})</span>
                    )}
                  </td>
                  <td className="py-2 px-4 text-right font-mono">
                    {order.stopPrice && order.stopPrice > 0 ? (
                      <div>
                        <div className="text-yellow-400">{formatPrice(order.stopPrice)}</div>
                        <div className="text-xs text-gray-500">→ {formatPrice(order.price)}</div>
                      </div>
                    ) : (
                      formatPrice(order.price)
                    )}
                  </td>
                  <td className="py-2 px-4 text-right font-mono">{formatQuantity(order.origQty)}</td>
                  <td className="py-2 px-4 text-right">
                    <span className={order.executedQty > 0 ? 'text-yellow-500' : ''}>
                      {formatQuantity(order.executedQty)} / {formatQuantity(order.origQty)}
                    </span>
                  </td>
                  <td className="py-2 px-4 text-center">
                    <button
                      onClick={() => onCancelOrder(order.symbol, order.orderId)}
                      className="p-1 hover:bg-red-500/20 rounded text-red-500"
                    >
                      <X className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Conditional/Algo Orders Section */}
      {algoOrders.length > 0 && (
        <div>
          <div className="px-4 py-2 bg-gray-800 text-sm font-medium text-gray-300 flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 text-yellow-500" />
            Conditional Orders - TP/SL ({algoOrders.length})
          </div>
          <table className="w-full text-sm">
            <thead className="bg-gray-800/50">
              <tr className="text-gray-400">
                <th className="text-left py-2 px-4 font-medium">Time</th>
                <th className="text-left py-2 px-4 font-medium">Symbol</th>
                <th className="text-left py-2 px-4 font-medium">Type</th>
                <th className="text-left py-2 px-4 font-medium">Side</th>
                <th className="text-right py-2 px-4 font-medium">Trigger</th>
                <th className="text-right py-2 px-4 font-medium">Qty</th>
                <th className="text-left py-2 px-4 font-medium">Status</th>
                <th className="text-center py-2 px-4 font-medium">Action</th>
              </tr>
            </thead>
            <tbody>
              {algoOrders.map((order) => {
                const isTP = order.orderType === 'TAKE_PROFIT_MARKET' || order.orderType === 'TAKE_PROFIT';
                const isSL = order.orderType === 'STOP_MARKET' || order.orderType === 'STOP';
                const triggerPrice = parseFloat(order.triggerPrice) || 0;
                const qty = parseFloat(order.quantity) || 0;
                return (
                  <tr key={order.algoId} className="border-b border-gray-800 hover:bg-gray-800/50">
                    <td className="py-2 px-4 text-gray-400 text-xs">
                      {formatDistanceToNow(new Date(order.createTime), { addSuffix: true })}
                    </td>
                    <td className="py-2 px-4 font-medium">{order.symbol}</td>
                    <td className="py-2 px-4">
                      <div className="flex items-center gap-1">
                        {isTP ? (
                          <><Target className="w-3 h-3 text-green-500" /><span className="text-green-400">TP</span></>
                        ) : isSL ? (
                          <><Shield className="w-3 h-3 text-red-500" /><span className="text-red-400">SL</span></>
                        ) : (
                          <span className="text-gray-400">{order.orderType}</span>
                        )}
                      </div>
                    </td>
                    <td className={`py-2 px-4 ${order.side === 'BUY' ? 'text-green-500' : 'text-red-500'}`}>
                      {order.side}
                      {order.positionSide !== 'BOTH' && (
                        <span className="text-gray-500 text-xs ml-1">({order.positionSide})</span>
                      )}
                    </td>
                    <td className="py-2 px-4 text-right font-mono text-yellow-400">
                      {formatPrice(triggerPrice)}
                    </td>
                    <td className="py-2 px-4 text-right font-mono">
                      {order.closePosition ? <span className="text-purple-400">Close All</span> : formatQuantity(qty)}
                    </td>
                    <td className="py-2 px-4">
                      <span className={`px-2 py-0.5 rounded text-xs ${
                        order.algoStatus === 'EXECUTING' ? 'bg-green-500/20 text-green-400' :
                        order.algoStatus === 'NEW' ? 'bg-blue-500/20 text-blue-400' :
                        order.algoStatus === 'PENDING' ? 'bg-yellow-500/20 text-yellow-400' :
                        'bg-gray-500/20 text-gray-400'
                      }`}>
                        {order.algoStatus}
                      </span>
                    </td>
                    <td className="py-2 px-4 text-center">
                      <button
                        onClick={() => onCancelAlgoOrder(order.symbol, order.algoId)}
                        className="p-1 hover:bg-red-500/20 rounded text-red-500"
                      >
                        <X className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

// Trade History Content
function TradeHistoryContent({
  trades,
  loading,
}: {
  trades: AccountTrade[];
  loading: boolean;
}) {
  if (loading && trades.length === 0) {
    return (
      <div className="text-center text-gray-400 py-8">
        <RefreshCw className="w-8 h-8 mx-auto mb-2 animate-spin opacity-30" />
        <p>Loading trade history...</p>
      </div>
    );
  }

  if (trades.length === 0) {
    return (
      <div className="text-center text-gray-400 py-8">
        <History className="w-8 h-8 mx-auto mb-2 opacity-30" />
        <p>No trade history</p>
      </div>
    );
  }

  // Calculate totals
  const totalPnl = trades.reduce((sum, t) => sum + (t.realizedPnl || 0), 0);
  const totalFees = trades.reduce((sum, t) => sum + (t.commission || 0), 0);

  return (
    <div>
      {/* Summary */}
      <div className="px-4 py-2 bg-gray-800 flex items-center justify-between text-sm">
        <span className="text-gray-400">Recent Trades: {trades.length}</span>
        <div className="flex items-center gap-4">
          <span className={getPositionColor(totalPnl)}>
            PnL: {formatUSD(totalPnl)}
          </span>
          <span className="text-gray-400">
            Fees: {formatUSD(totalFees)}
          </span>
        </div>
      </div>

      <table className="w-full text-sm">
        <thead className="bg-gray-800/50">
          <tr className="text-gray-400">
            <th className="text-left py-2 px-4 font-medium">Time</th>
            <th className="text-left py-2 px-4 font-medium">Symbol</th>
            <th className="text-left py-2 px-4 font-medium">Side</th>
            <th className="text-right py-2 px-4 font-medium">Price</th>
            <th className="text-right py-2 px-4 font-medium">Qty</th>
            <th className="text-right py-2 px-4 font-medium">Value</th>
            <th className="text-right py-2 px-4 font-medium">Realized PnL</th>
            <th className="text-right py-2 px-4 font-medium">Fee</th>
          </tr>
        </thead>
        <tbody>
          {trades.map((trade) => (
            <tr key={trade.id} className="border-b border-gray-800 hover:bg-gray-800/50">
              <td className="py-2 px-4 text-gray-400 text-xs">
                {formatDistanceToNow(new Date(trade.time), { addSuffix: true })}
              </td>
              <td className="py-2 px-4 font-medium">{trade.symbol}</td>
              <td className="py-2 px-4">
                <span className={trade.side === 'BUY' ? 'text-green-500' : 'text-red-500'}>
                  {trade.side}
                </span>
                {trade.positionSide !== 'BOTH' && (
                  <span className="text-gray-500 text-xs ml-1">({trade.positionSide})</span>
                )}
              </td>
              <td className="py-2 px-4 text-right font-mono">{formatPrice(trade.price)}</td>
              <td className="py-2 px-4 text-right font-mono">{formatQuantity(trade.qty)}</td>
              <td className="py-2 px-4 text-right font-mono">{formatUSD(trade.quoteQty)}</td>
              <td className={`py-2 px-4 text-right font-mono ${getPositionColor(trade.realizedPnl)}`}>
                {trade.realizedPnl !== 0 ? formatUSD(trade.realizedPnl) : '-'}
              </td>
              <td className="py-2 px-4 text-right font-mono text-gray-400 text-xs">
                {Number(trade.commission || 0).toFixed(4)} {trade.commissionAsset}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// Signal confidence bar component for AI decisions
function SignalBar({ label, icon: Icon, direction, confidence }: {
  label: string;
  icon: React.ElementType;
  direction?: string;
  confidence?: number;
}) {
  if (!direction || confidence === undefined) return null;

  const isLong = direction.toLowerCase() === 'long' || direction.toLowerCase() === 'buy';
  const isShort = direction.toLowerCase() === 'short' || direction.toLowerCase() === 'sell';
  const confidencePercent = Math.round(confidence * 100);

  return (
    <div className="flex items-center gap-2 text-xs">
      <Icon className="w-3.5 h-3.5 text-gray-400" />
      <span className="w-20 text-gray-400">{label}</span>
      <span className={`w-12 font-medium ${isLong ? 'text-green-400' : isShort ? 'text-red-400' : 'text-gray-400'}`}>
        {direction}
      </span>
      <div className="flex-1 h-2 bg-gray-700 rounded-full overflow-hidden max-w-[100px]">
        <div
          className={`h-full rounded-full ${isLong ? 'bg-green-500' : isShort ? 'bg-red-500' : 'bg-gray-500'}`}
          style={{ width: `${confidencePercent}%` }}
        />
      </div>
      <span className="w-10 text-right text-gray-300">{confidencePercent}%</span>
    </div>
  );
}

// AI Decisions Content - Shows AI decisions directly from database
function AIDecisionsContent({
  decisions,
  loading,
}: {
  decisions: AIDecisionFromAPI[];
  loading: boolean;
}) {
  const [expandedRows, setExpandedRows] = useState<Set<number>>(new Set());
  const [filter, setFilter] = useState<'all' | 'executed'>('all');

  const toggleRow = (id: number) => {
    setExpandedRows(prev => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  if (loading && decisions.length === 0) {
    return (
      <div className="text-center text-gray-400 py-8">
        <RefreshCw className="w-8 h-8 mx-auto mb-2 animate-spin opacity-30" />
        <p>Loading AI decisions...</p>
      </div>
    );
  }

  if (decisions.length === 0) {
    return (
      <div className="text-center text-gray-400 py-8">
        <Brain className="w-8 h-8 mx-auto mb-2 opacity-30" />
        <p>No AI decisions yet</p>
        <p className="text-xs mt-1">Autopilot decisions will appear here with AI reasoning</p>
      </div>
    );
  }

  const filteredDecisions = filter === 'executed'
    ? decisions.filter(d => d.executed)
    : decisions;

  // Calculate stats
  const executedCount = decisions.filter(d => d.executed).length;
  const longCount = filteredDecisions.filter(d => d.action === 'open_long').length;
  const shortCount = filteredDecisions.filter(d => d.action === 'open_short').length;
  const avgConfidence = filteredDecisions.length > 0
    ? filteredDecisions.reduce((sum, d) => sum + d.confidence, 0) / filteredDecisions.length
    : 0;

  return (
    <div>
      {/* Summary */}
      <div className="px-4 py-2 bg-gray-800 flex items-center justify-between text-sm">
        <div className="flex items-center gap-4">
          <span className="text-gray-400">
            {filteredDecisions.length} decisions |
            <span className="text-green-500 ml-1">{longCount} Long</span> /
            <span className="text-red-500 ml-1">{shortCount} Short</span>
          </span>
          <span className="text-purple-400">
            <Zap className="w-3 h-3 inline mr-1" />
            {executedCount} executed
          </span>
        </div>
        <div className="flex items-center gap-4">
          <span className="text-blue-400">
            Avg: {(Number(avgConfidence || 0) * 100).toFixed(0)}%
          </span>
          <select
            value={filter}
            onChange={(e) => setFilter(e.target.value as 'all' | 'executed')}
            className="bg-gray-700 border border-gray-600 rounded px-2 py-0.5 text-xs"
          >
            <option value="all">All Decisions</option>
            <option value="executed">Executed Only</option>
          </select>
        </div>
      </div>

      <table className="w-full text-sm">
        <thead className="bg-gray-800/50">
          <tr className="text-gray-400">
            <th className="w-8 py-2 px-2"></th>
            <th className="text-left py-2 px-3 font-medium">Time</th>
            <th className="text-left py-2 px-3 font-medium">Symbol</th>
            <th className="text-left py-2 px-3 font-medium">Action</th>
            <th className="text-right py-2 px-3 font-medium">Price</th>
            <th className="text-center py-2 px-3 font-medium">Confidence</th>
            <th className="text-center py-2 px-3 font-medium">Confluence</th>
            <th className="text-left py-2 px-3 font-medium">Risk</th>
            <th className="text-center py-2 px-3 font-medium">Executed</th>
          </tr>
        </thead>
        <tbody>
          {filteredDecisions.map((decision) => {
            const isExpanded = expandedRows.has(decision.id);
            const isLong = decision.action === 'open_long';

            return (
              <React.Fragment key={decision.id}>
                <tr
                  className="border-b border-gray-800 hover:bg-gray-800/50 cursor-pointer"
                  onClick={() => toggleRow(decision.id)}
                >
                  <td className="py-2 px-2 text-center">
                    {isExpanded ? (
                      <ChevronDown className="w-4 h-4 text-purple-400" />
                    ) : (
                      <ChevronRight className="w-4 h-4 text-gray-500" />
                    )}
                  </td>
                  <td className="py-2 px-3 text-gray-400 text-xs">
                    {formatDistanceToNow(new Date(decision.created_at), { addSuffix: true })}
                  </td>
                  <td className="py-2 px-3 font-medium">{decision.symbol}</td>
                  <td className="py-2 px-3">
                    <span className={isLong ? 'text-green-500' : 'text-red-500'}>
                      {isLong ? 'LONG' : 'SHORT'}
                    </span>
                  </td>
                  <td className="py-2 px-3 text-right font-mono">
                    {formatPrice(decision.current_price)}
                  </td>
                  <td className="py-2 px-3 text-center">
                    <div className="flex items-center justify-center gap-2">
                      <div className="w-16 h-2 bg-gray-700 rounded-full overflow-hidden">
                        <div
                          className={`h-full ${decision.confidence >= 0.7 ? 'bg-green-500' : decision.confidence >= 0.5 ? 'bg-yellow-500' : 'bg-red-500'}`}
                          style={{ width: `${decision.confidence * 100}%` }}
                        />
                      </div>
                      <span className="text-xs">{(Number(decision.confidence || 0) * 100).toFixed(0)}%</span>
                    </div>
                  </td>
                  <td className="py-2 px-3 text-center">
                    <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                      decision.confluence_count >= 4
                        ? 'bg-green-500/20 text-green-400'
                        : decision.confluence_count >= 3
                        ? 'bg-yellow-500/20 text-yellow-400'
                        : 'bg-gray-500/20 text-gray-400'
                    }`}>
                      {decision.confluence_count} signals
                    </span>
                  </td>
                  <td className="py-2 px-3">
                    <span className={`text-xs ${
                      decision.risk_level === 'conservative'
                        ? 'text-green-400'
                        : decision.risk_level === 'moderate'
                        ? 'text-yellow-400'
                        : 'text-red-400'
                    }`}>
                      {decision.risk_level}
                    </span>
                  </td>
                  <td className="py-2 px-3 text-center">
                    {decision.executed ? (
                      <span className="px-2 py-0.5 rounded text-xs font-medium bg-green-500/20 text-green-400">
                        Yes
                      </span>
                    ) : (
                      <span className="px-2 py-0.5 rounded text-xs font-medium bg-gray-500/20 text-gray-400">
                        No
                      </span>
                    )}
                  </td>
                </tr>
                {/* Expanded Decision Details Row */}
                {isExpanded && (
                  <tr className="bg-gray-800/30">
                    <td colSpan={9} className="py-3 px-4">
                      <AIDecisionDetailFromAPI decision={decision} />
                    </td>
                  </tr>
                )}
              </React.Fragment>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

// AI Decision Detail component for API decisions
function AIDecisionDetailFromAPI({ decision }: { decision: AIDecisionFromAPI }) {
  const signals = [
    { name: 'ML Model', direction: decision.ml_direction, confidence: decision.ml_confidence, icon: Activity },
    { name: 'LLM Analysis', direction: decision.llm_direction, confidence: decision.llm_confidence, icon: Brain },
    { name: 'Sentiment', direction: decision.sentiment_direction, confidence: decision.sentiment_confidence, icon: MessageSquare },
    { name: 'Pattern', direction: decision.pattern_direction, confidence: decision.pattern_confidence, icon: BarChart3 },
    { name: 'Big Candle', direction: decision.bigcandle_direction, confidence: decision.bigcandle_confidence, icon: Sparkles },
  ].filter(s => s.direction && s.confidence);

  return (
    <div className="space-y-3">
      {/* Reasoning */}
      <div className="bg-gray-900/50 rounded p-3">
        <h4 className="text-xs font-medium text-gray-400 mb-1 flex items-center gap-1">
          <MessageSquare className="w-3 h-3" /> AI Reasoning
        </h4>
        <p className="text-sm text-gray-300">{decision.reasoning}</p>
      </div>

      {/* Signal Breakdown */}
      <div>
        <h4 className="text-xs font-medium text-gray-400 mb-2 flex items-center gap-1">
          <Zap className="w-3 h-3" /> Signal Breakdown
        </h4>
        <div className="grid grid-cols-2 md:grid-cols-5 gap-2">
          {signals.map((signal) => (
            <SignalBar
              key={signal.name}
              label={signal.name}
              icon={signal.icon}
              direction={signal.direction}
              confidence={signal.confidence}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
