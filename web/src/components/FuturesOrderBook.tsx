import { useEffect, useMemo, useState } from 'react';
import { useFuturesStore, selectOrderBookSpread } from '../store/futuresStore';
import { formatPrice, formatQuantity } from '../services/futuresApi';
import { RefreshCw, ChevronDown } from 'lucide-react';

interface OrderBookRowProps {
  price: string;
  quantity: string;
  total: number;
  maxTotal: number;
  side: 'bid' | 'ask';
  onClick: (price: string) => void;
}

function OrderBookRow({ price, quantity, total, maxTotal, side, onClick }: OrderBookRowProps) {
  const percent = (total / maxTotal) * 100;
  const bgColor = side === 'bid' ? 'bg-green-500/20' : 'bg-red-500/20';
  const textColor = side === 'bid' ? 'text-green-500' : 'text-red-500';

  return (
    <div
      onClick={() => onClick(price)}
      className="relative flex items-center justify-between px-2 py-1 cursor-pointer hover:bg-gray-700/50 text-xs"
    >
      {/* Depth Bar */}
      <div
        className={`absolute ${side === 'bid' ? 'right-0' : 'left-0'} top-0 bottom-0 ${bgColor}`}
        style={{ width: `${Math.min(percent, 100)}%` }}
      />

      {/* Content */}
      <span className={`relative ${textColor} font-mono`}>{formatPrice(parseFloat(price))}</span>
      <span className="relative text-gray-300 font-mono">{formatQuantity(parseFloat(quantity))}</span>
      <span className="relative text-gray-400 font-mono">{formatQuantity(total, 2)}</span>
    </div>
  );
}

interface AggregationLevel {
  value: number;
  label: string;
}

export default function FuturesOrderBook() {
  const {
    orderBook,
    markPrice,
    fundingRate,
    fetchOrderBook,
    updateOrderForm,
    selectedSymbol,
  } = useFuturesStore();

  const spreadInfo = useFuturesStore(selectOrderBookSpread);
  const [aggregation, setAggregation] = useState<number>(0.01);
  const [showRows] = useState<number>(15);
  const [isRefreshing, setIsRefreshing] = useState(false);

  // Aggregation levels
  const aggregationLevels: AggregationLevel[] = [
    { value: 0.01, label: '0.01' },
    { value: 0.1, label: '0.1' },
    { value: 1, label: '1' },
    { value: 10, label: '10' },
    { value: 100, label: '100' },
  ];

  useEffect(() => {
    fetchOrderBook(selectedSymbol, 50);
    // Order book is cached for 30s, so 60s polling is fine
    const interval = setInterval(() => {
      fetchOrderBook(selectedSymbol, 50);
    }, 60000);
    return () => clearInterval(interval);
  }, [selectedSymbol, fetchOrderBook]);

  // Aggregate order book data
  const aggregatedData = useMemo(() => {
    if (!orderBook) return { bids: [], asks: [] };

    const aggregateOrders = (orders: [string, string][], isAsk: boolean) => {
      const aggregated: Map<number, number> = new Map();

      orders.forEach(([price, qty]) => {
        const priceNum = parseFloat(price);
        const qtyNum = parseFloat(qty);
        const bucket = isAsk
          ? Math.ceil(priceNum / aggregation) * aggregation
          : Math.floor(priceNum / aggregation) * aggregation;

        aggregated.set(bucket, (aggregated.get(bucket) || 0) + qtyNum);
      });

      const result = Array.from(aggregated.entries())
        .map(([price, quantity]) => ({
          price: price.toString(),
          quantity: quantity.toString(),
        }))
        .sort((a, b) => {
          const diff = parseFloat(b.price) - parseFloat(a.price);
          return isAsk ? -diff : diff;
        });

      return result;
    };

    return {
      bids: aggregateOrders(orderBook.bids || [], false).slice(0, showRows),
      asks: aggregateOrders(orderBook.asks || [], true).slice(0, showRows).reverse(),
    };
  }, [orderBook, aggregation, showRows]);

  // Calculate cumulative totals
  const { bidsWithTotal, asksWithTotal, maxTotal } = useMemo(() => {
    let bidTotal = 0;
    let askTotal = 0;
    let maxTotal = 0;

    const bidsWithTotal = aggregatedData.bids.map((bid) => {
      bidTotal += parseFloat(bid.quantity);
      maxTotal = Math.max(maxTotal, bidTotal);
      return { ...bid, total: bidTotal };
    });

    const asksReversed = [...aggregatedData.asks].reverse();
    const asksWithTotal = asksReversed.map((ask) => {
      askTotal += parseFloat(ask.quantity);
      maxTotal = Math.max(maxTotal, askTotal);
      return { ...ask, total: askTotal };
    }).reverse();

    return { bidsWithTotal, asksWithTotal, maxTotal };
  }, [aggregatedData]);

  const handlePriceClick = (price: string) => {
    updateOrderForm({ price });
  };

  const handleRefresh = async () => {
    setIsRefreshing(true);
    await fetchOrderBook(selectedSymbol, 50);
    setIsRefreshing(false);
  };

  const currentPrice = markPrice?.markPrice || 0;
  const indexPrice = markPrice?.indexPrice || 0;
  const currentFundingRate = fundingRate?.fundingRate || 0;

  return (
    <div className="bg-gray-900 rounded-lg border border-gray-700 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-gray-700">
        <span className="text-sm font-semibold">Order Book</span>
        <div className="flex items-center gap-2">
          {/* Aggregation Selector */}
          <div className="relative">
            <select
              value={aggregation}
              onChange={(e) => setAggregation(parseFloat(e.target.value))}
              className="appearance-none bg-gray-800 text-xs px-2 py-1 pr-6 rounded border border-gray-700 focus:outline-none focus:border-yellow-500"
            >
              {aggregationLevels.map((level) => (
                <option key={level.value} value={level.value}>
                  {level.label}
                </option>
              ))}
            </select>
            <ChevronDown className="absolute right-1 top-1/2 -translate-y-1/2 w-3 h-3 text-gray-400 pointer-events-none" />
          </div>

          {/* Refresh Button */}
          <button
            onClick={handleRefresh}
            disabled={isRefreshing}
            className="p-1 hover:bg-gray-700 rounded"
          >
            <RefreshCw className={`w-4 h-4 text-gray-400 ${isRefreshing ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {/* Column Headers */}
      <div className="flex items-center justify-between px-2 py-1 text-xs text-gray-500 border-b border-gray-700">
        <span>Price (USDT)</span>
        <span>Amount</span>
        <span>Total</span>
      </div>

      {/* Asks (Sells) */}
      <div className="overflow-hidden">
        {asksWithTotal.map((ask, index) => (
          <OrderBookRow
            key={`ask-${index}`}
            price={ask.price}
            quantity={ask.quantity}
            total={ask.total}
            maxTotal={maxTotal}
            side="ask"
            onClick={handlePriceClick}
          />
        ))}
      </div>

      {/* Current Price / Spread */}
      <div className="px-2 py-2 bg-gray-800 border-y border-gray-700">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="text-lg font-bold text-white">
              {formatPrice(currentPrice)}
            </span>
            {spreadInfo.spread > 0 && (
              <span className="text-xs text-gray-400">
                Spread: {formatPrice(spreadInfo.spread)} ({spreadInfo.spreadPercent.toFixed(3)}%)
              </span>
            )}
          </div>
          <div className="text-right">
            <div className="text-xs text-gray-400">Index: {formatPrice(indexPrice)}</div>
            <div className={`text-xs ${currentFundingRate >= 0 ? 'text-green-500' : 'text-red-500'}`}>
              Funding: {(currentFundingRate * 100).toFixed(4)}%
            </div>
          </div>
        </div>
      </div>

      {/* Bids (Buys) */}
      <div className="overflow-hidden">
        {bidsWithTotal.map((bid, index) => (
          <OrderBookRow
            key={`bid-${index}`}
            price={bid.price}
            quantity={bid.quantity}
            total={bid.total}
            maxTotal={maxTotal}
            side="bid"
            onClick={handlePriceClick}
          />
        ))}
      </div>

      {/* Footer - Quick Stats */}
      <div className="px-3 py-2 border-t border-gray-700 text-xs text-gray-500">
        <div className="flex justify-between">
          <span>
            Buy Total: <span className="text-green-500">
              {formatQuantity(bidsWithTotal[bidsWithTotal.length - 1]?.total || 0, 2)}
            </span>
          </span>
          <span>
            Sell Total: <span className="text-red-500">
              {formatQuantity(asksWithTotal[0]?.total || 0, 2)}
            </span>
          </span>
        </div>
      </div>
    </div>
  );
}
