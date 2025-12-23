import { useState, useEffect } from 'react';
import { useFuturesStore } from '../store/futuresStore';
import {
  formatUSD,
  formatPrice,
  formatQuantity,
  formatFundingRate,
  getPositionColor,
  getOrderTypeLabel,
  getSideColor,
  futuresApi,
} from '../services/futuresApi';
import { formatDistanceToNow } from 'date-fns';
import {
  FileText,
  History,
  DollarSign,
  BarChart3,
  RefreshCw,
  X,
} from 'lucide-react';

type TabType = 'orders' | 'history' | 'transactions' | 'funding';

interface AccountTrade {
  symbol: string;
  id: number;
  orderId: number;
  side: string;
  positionSide: string;
  price: number;
  qty: number;
  realizedPnl: number;
  marginAsset: string;
  quoteQty: number;
  commission: number;
  commissionAsset: string;
  time: number;
  buyer: boolean;
  maker: boolean;
}

export default function FuturesHistoryTabs() {
  const [activeTab, setActiveTab] = useState<TabType>('orders');
  const [limit, setLimit] = useState(20);
  const [accountTrades, setAccountTrades] = useState<AccountTrade[]>([]);
  const [tradesLoading, setTradesLoading] = useState(false);

  const {
    openOrders,
    fundingFees,
    transactions,
    fetchOpenOrders,
    fetchFundingFees,
    fetchTransactions,
    cancelOrder,
    cancelAllOrders,
    selectedSymbol,
    isLoading,
  } = useFuturesStore();

  // Fetch account trades from Binance
  const fetchAccountTrades = async () => {
    setTradesLoading(true);
    try {
      const response = await futuresApi.getAccountTrades(undefined, limit);
      setAccountTrades(response.trades || []);
    } catch (err) {
      console.error('Failed to fetch account trades:', err);
    } finally {
      setTradesLoading(false);
    }
  };

  useEffect(() => {
    switch (activeTab) {
      case 'orders':
        fetchOpenOrders();
        break;
      case 'history':
        fetchAccountTrades();
        break;
      case 'transactions':
        fetchTransactions(undefined, undefined, limit);
        break;
      case 'funding':
        fetchFundingFees(undefined, limit);
        break;
    }
  }, [activeTab, limit, fetchOpenOrders, fetchFundingFees, fetchTransactions]);

  const handleCancelOrder = async (symbol: string, orderId: number) => {
    if (window.confirm('Are you sure you want to cancel this order?')) {
      await cancelOrder(symbol, orderId);
    }
  };

  const handleCancelAll = async () => {
    if (window.confirm(`Cancel all open orders for ${selectedSymbol}?`)) {
      await cancelAllOrders(selectedSymbol);
    }
  };

  const tabs = [
    { id: 'orders' as TabType, label: 'Open Orders', icon: FileText, count: openOrders.length },
    { id: 'history' as TabType, label: 'Trade History', icon: History },
    { id: 'transactions' as TabType, label: 'Transactions', icon: BarChart3 },
    { id: 'funding' as TabType, label: 'Funding Fees', icon: DollarSign },
  ];

  const refreshData = () => {
    switch (activeTab) {
      case 'orders':
        fetchOpenOrders();
        break;
      case 'history':
        fetchAccountTrades();
        break;
      case 'transactions':
        fetchTransactions(undefined, undefined, limit);
        break;
      case 'funding':
        fetchFundingFees(undefined, limit);
        break;
    }
  };

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
          {activeTab === 'orders' && openOrders.length > 0 && (
            <button
              onClick={handleCancelAll}
              className="text-xs text-red-500 hover:text-red-400"
            >
              Cancel All
            </button>
          )}

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
            onClick={refreshData}
            className="p-1.5 hover:bg-gray-700 rounded"
          >
            <RefreshCw className={`w-4 h-4 text-gray-400 ${isLoading ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {/* Tab Content */}
      <div className="overflow-x-auto max-h-96">
        {activeTab === 'orders' && <OpenOrdersTable orders={openOrders} onCancel={handleCancelOrder} />}
        {activeTab === 'history' && <AccountTradesTable trades={accountTrades} loading={tradesLoading} />}
        {activeTab === 'transactions' && <TransactionsTable transactions={transactions} />}
        {activeTab === 'funding' && <FundingFeesTable fees={fundingFees} />}
      </div>
    </div>
  );
}

// Open Orders Table Component
function OpenOrdersTable({ orders, onCancel }: { orders: any[]; onCancel: (symbol: string, orderId: number) => void }) {
  if (orders.length === 0) {
    return (
      <div className="text-center text-gray-400 py-8">
        <FileText className="w-8 h-8 mx-auto mb-2 opacity-30" />
        <p>No open orders</p>
      </div>
    );
  }

  return (
    <table className="w-full text-sm">
      <thead className="bg-gray-800">
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
        {orders.map((order) => (
          <tr key={order.orderId} className="border-b border-gray-800 hover:bg-gray-800/50">
            <td className="py-2 px-4 text-gray-400 text-xs">
              {formatDistanceToNow(new Date(order.time), { addSuffix: true })}
            </td>
            <td className="py-2 px-4 font-medium">{order.symbol}</td>
            <td className="py-2 px-4 text-gray-400">{getOrderTypeLabel(order.type)}</td>
            <td className={`py-2 px-4 ${getSideColor(order.side)}`}>
              {order.side}
              {order.positionSide !== 'BOTH' && (
                <span className="text-gray-500 text-xs ml-1">({order.positionSide})</span>
              )}
            </td>
            <td className="py-2 px-4 text-right font-mono">
              {order.stopPrice > 0 ? (
                <div>
                  <div>{formatPrice(order.stopPrice)}</div>
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
                onClick={() => onCancel(order.symbol, order.orderId)}
                className="p-1 hover:bg-red-500/20 rounded text-red-500"
              >
                <X className="w-4 h-4" />
              </button>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

// Account Trades Table Component (from Binance API)
function AccountTradesTable({ trades, loading }: { trades: AccountTrade[]; loading: boolean }) {
  if (loading) {
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

  return (
    <table className="w-full text-sm">
      <thead className="bg-gray-800">
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
              <span className={`${trade.side === 'BUY' ? 'text-green-500' : 'text-red-500'}`}>
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
  );
}

// Transactions Table Component
function TransactionsTable({ transactions }: { transactions: any[] }) {
  if (transactions.length === 0) {
    return (
      <div className="text-center text-gray-400 py-8">
        <BarChart3 className="w-8 h-8 mx-auto mb-2 opacity-30" />
        <p>No transactions</p>
      </div>
    );
  }

  return (
    <table className="w-full text-sm">
      <thead className="bg-gray-800">
        <tr className="text-gray-400">
          <th className="text-left py-2 px-4 font-medium">Time</th>
          <th className="text-left py-2 px-4 font-medium">Symbol</th>
          <th className="text-left py-2 px-4 font-medium">Type</th>
          <th className="text-right py-2 px-4 font-medium">Amount</th>
          <th className="text-left py-2 px-4 font-medium">Info</th>
        </tr>
      </thead>
      <tbody>
        {transactions.map((tx) => (
          <tr key={tx.id} className="border-b border-gray-800 hover:bg-gray-800/50">
            <td className="py-2 px-4 text-gray-400 text-xs">
              {formatDistanceToNow(new Date(tx.timestamp), { addSuffix: true })}
            </td>
            <td className="py-2 px-4 font-medium">{tx.symbol || '-'}</td>
            <td className="py-2 px-4">
              <span className="px-2 py-0.5 bg-gray-700 rounded text-xs">
                {tx.incomeType.replace(/_/g, ' ')}
              </span>
            </td>
            <td className={`py-2 px-4 text-right font-mono ${getPositionColor(tx.income)}`}>
              {tx.income >= 0 ? '+' : ''}{formatUSD(tx.income)}
            </td>
            <td className="py-2 px-4 text-gray-400 text-xs truncate max-w-xs">
              {tx.info || '-'}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

// Funding Fees Table Component
function FundingFeesTable({ fees }: { fees: any[] }) {
  if (fees.length === 0) {
    return (
      <div className="text-center text-gray-400 py-8">
        <DollarSign className="w-8 h-8 mx-auto mb-2 opacity-30" />
        <p>No funding fee history</p>
      </div>
    );
  }

  const totalFees = fees.reduce((sum, fee) => sum + fee.fundingFee, 0);

  return (
    <>
      <div className="px-4 py-2 bg-gray-800 border-b border-gray-700 flex justify-between text-sm">
        <span className="text-gray-400">Total Funding Fees:</span>
        <span className={getPositionColor(totalFees)}>{formatUSD(totalFees)}</span>
      </div>
      <table className="w-full text-sm">
        <thead className="bg-gray-800">
          <tr className="text-gray-400">
            <th className="text-left py-2 px-4 font-medium">Time</th>
            <th className="text-left py-2 px-4 font-medium">Symbol</th>
            <th className="text-right py-2 px-4 font-medium">Funding Rate</th>
            <th className="text-right py-2 px-4 font-medium">Position Size</th>
            <th className="text-right py-2 px-4 font-medium">Funding Fee</th>
          </tr>
        </thead>
        <tbody>
          {fees.map((fee) => (
            <tr key={fee.id} className="border-b border-gray-800 hover:bg-gray-800/50">
              <td className="py-2 px-4 text-gray-400 text-xs">
                {formatDistanceToNow(new Date(fee.timestamp), { addSuffix: true })}
              </td>
              <td className="py-2 px-4 font-medium">{fee.symbol}</td>
              <td className={`py-2 px-4 text-right ${fee.fundingRate >= 0 ? 'text-green-500' : 'text-red-500'}`}>
                {formatFundingRate(fee.fundingRate)}
              </td>
              <td className="py-2 px-4 text-right font-mono">
                {formatQuantity(fee.positionAmt)}
              </td>
              <td className={`py-2 px-4 text-right font-mono ${getPositionColor(fee.fundingFee)}`}>
                {fee.fundingFee >= 0 ? '+' : ''}{formatUSD(fee.fundingFee)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}
