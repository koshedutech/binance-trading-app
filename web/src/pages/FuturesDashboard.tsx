import { useEffect, useState } from 'react';
import { useFuturesStore, selectAvailableBalance, selectTotalMarginUsed, selectTotalUnrealizedPnl } from '../store/futuresStore';
import FuturesTradingPanel from '../components/FuturesTradingPanel';
import FuturesOrderBook from '../components/FuturesOrderBook';
import FuturesChart from '../components/FuturesChart';
import FuturesPositionsTable from '../components/FuturesPositionsTable';
import FuturesHistoryTabs from '../components/FuturesHistoryTabs';
import FuturesAutopilotPanel from '../components/FuturesAutopilotPanel';
import FuturesAISignals from '../components/FuturesAISignals';
import PanicButton from '../components/PanicButton';
import { formatUSD, formatPercent, getPositionColor } from '../services/futuresApi';
import { apiService } from '../services/api';
import {
  Wallet,
  TrendingUp,
  TrendingDown,
  Activity,
  ChevronDown,
  RefreshCw,
  AlertCircle,
  BarChart3,
  BookOpen,
  Zap,
} from 'lucide-react';

export default function FuturesDashboard() {
  const {
    selectedSymbol,
    setSelectedSymbol,
    symbols,
    fetchSymbols,
    accountInfo,
    fetchAccountInfo,
    markPrice,
    fundingRate,
    fetchMarkPrice,
    fetchFundingRate,
    fetchPositions,
    fetchPositionMode,
    metrics,
    fetchMetrics,
    error,
    setError,
    isLoading,
  } = useFuturesStore();

  const availableBalance = useFuturesStore(selectAvailableBalance);
  const marginUsed = useFuturesStore(selectTotalMarginUsed);
  const totalUnrealizedPnl = useFuturesStore(selectTotalUnrealizedPnl);

  const [showSymbolDropdown, setShowSymbolDropdown] = useState(false);
  const [centerView, setCenterView] = useState<'orderbook' | 'chart'>('orderbook');
  const [tradingMode, setTradingMode] = useState<{ mode: 'paper' | 'live'; mode_label: string } | null>(null);

  // Fetch trading mode
  useEffect(() => {
    const fetchTradingMode = async () => {
      try {
        const modeData = await apiService.getTradingMode();
        setTradingMode(modeData);
      } catch (error) {
        console.error('Error fetching trading mode:', error);
      }
    };
    fetchTradingMode();
    const interval = setInterval(fetchTradingMode, 10000);
    return () => clearInterval(interval);
  }, []);

  // Initial data fetch
  useEffect(() => {
    fetchSymbols();
    fetchAccountInfo();
    fetchPositionMode();
    fetchMetrics();
  }, [fetchSymbols, fetchAccountInfo, fetchPositionMode, fetchMetrics]);

  // Fetch data for selected symbol
  useEffect(() => {
    fetchMarkPrice(selectedSymbol);
    fetchFundingRate(selectedSymbol);

    const interval = setInterval(() => {
      fetchMarkPrice(selectedSymbol);
      fetchFundingRate(selectedSymbol);
    }, 1000);

    return () => clearInterval(interval);
  }, [selectedSymbol, fetchMarkPrice, fetchFundingRate]);

  // Periodic account/position refresh
  useEffect(() => {
    const interval = setInterval(() => {
      fetchAccountInfo();
      fetchPositions();
    }, 5000);

    return () => clearInterval(interval);
  }, [fetchAccountInfo, fetchPositions]);

  const walletBalance = accountInfo?.totalWalletBalance || 0;
  const currentPrice = markPrice?.markPrice || 0;
  const priceChange24h = 0; // Would come from ticker data
  const currentFundingRate = fundingRate?.fundingRate || 0;
  const nextFundingTime = fundingRate?.nextFundingTime || 0;

  const filteredSymbols = symbols.filter(s => s.includes('USDT'));

  return (
    <div className="min-h-screen bg-gray-950 text-white p-4">
      {/* Error Banner */}
      {error && (
        <div className="mb-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg flex items-center justify-between">
          <div className="flex items-center gap-2 text-red-500">
            <AlertCircle className="w-5 h-5" />
            <span>{error}</span>
          </div>
          <button onClick={() => setError(null)} className="text-red-500 hover:text-red-400">
            ×
          </button>
        </div>
      )}

      {/* Header - Symbol Selector & Account Summary */}
      <div className="mb-4 flex flex-wrap items-center justify-between gap-4">
        {/* Left: Symbol Selector */}
        <div className="flex items-center gap-4">
          {/* Symbol Dropdown */}
          <div className="relative">
            <button
              onClick={() => setShowSymbolDropdown(!showSymbolDropdown)}
              className="flex items-center gap-2 px-4 py-2 bg-gray-800 hover:bg-gray-700 rounded-lg border border-gray-700"
            >
              <span className="text-lg font-bold">{selectedSymbol}</span>
              <ChevronDown className={`w-4 h-4 transition-transform ${showSymbolDropdown ? 'rotate-180' : ''}`} />
            </button>

            {showSymbolDropdown && (
              <div className="absolute z-50 mt-1 w-64 max-h-96 overflow-y-auto bg-gray-800 border border-gray-700 rounded-lg shadow-lg">
                <input
                  type="text"
                  placeholder="Search symbol..."
                  className="w-full px-3 py-2 bg-gray-900 border-b border-gray-700 text-sm focus:outline-none"
                  onChange={() => {
                    // Filter symbols (simple client-side filter)
                  }}
                />
                {filteredSymbols.slice(0, 50).map((symbol) => (
                  <button
                    key={symbol}
                    onClick={() => {
                      setSelectedSymbol(symbol);
                      setShowSymbolDropdown(false);
                    }}
                    className={`w-full px-3 py-2 text-left text-sm hover:bg-gray-700 ${
                      symbol === selectedSymbol ? 'bg-gray-700 text-yellow-500' : ''
                    }`}
                  >
                    {symbol}
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* Price Info */}
          <div className="flex items-center gap-3">
            <div className="text-2xl font-bold">
              {currentPrice > 0 ? formatUSD(currentPrice) : '-'}
            </div>
            {priceChange24h !== 0 && (
              <div className={`flex items-center gap-1 ${priceChange24h >= 0 ? 'text-green-500' : 'text-red-500'}`}>
                {priceChange24h >= 0 ? <TrendingUp className="w-4 h-4" /> : <TrendingDown className="w-4 h-4" />}
                {formatPercent(priceChange24h)}
              </div>
            )}
          </div>

          {/* Funding Rate */}
          <div className="text-sm">
            <span className="text-gray-500">Funding: </span>
            <span className={currentFundingRate >= 0 ? 'text-green-500' : 'text-red-500'}>
              {(currentFundingRate * 100).toFixed(4)}%
            </span>
            {nextFundingTime > 0 && (
              <span className="text-gray-500 ml-2">
                in {Math.floor((nextFundingTime - Date.now()) / 60000)}m
              </span>
            )}
          </div>
        </div>

        {/* Right: Account Summary, Live/Paper Indicator & Panic Button */}
        <div className="flex items-center gap-4">
          {/* Live/Paper Indicator */}
          {tradingMode && (
            <div className={`
              flex items-center gap-2 px-3 py-2 rounded-lg font-bold text-sm
              ${tradingMode.mode === 'live'
                ? 'bg-green-500/20 border-2 border-green-500 text-green-400'
                : 'bg-yellow-500/20 border-2 border-yellow-500 text-yellow-400'
              }
            `}>
              <span className={`w-2 h-2 rounded-full animate-pulse ${tradingMode.mode === 'live' ? 'bg-green-500' : 'bg-yellow-500'}`} />
              {tradingMode.mode === 'live' ? (
                <>
                  <Zap className="w-4 h-4" />
                  LIVE
                </>
              ) : (
                <>
                  <AlertCircle className="w-4 h-4" />
                  PAPER
                </>
              )}
            </div>
          )}

          {/* Wallet Balance */}
          <div className="flex items-center gap-2">
            <Wallet className="w-4 h-4 text-gray-400" />
            <div>
              <div className="text-xs text-gray-500">Wallet Balance</div>
              <div className="font-semibold">{formatUSD(walletBalance)}</div>
            </div>
          </div>

          {/* Available Balance */}
          <div>
            <div className="text-xs text-gray-500">Available</div>
            <div className="font-semibold text-green-500">{formatUSD(availableBalance)}</div>
          </div>

          {/* Margin Used */}
          <div>
            <div className="text-xs text-gray-500">Margin Used</div>
            <div className="font-semibold text-yellow-500">{formatUSD(marginUsed)}</div>
          </div>

          {/* Unrealized PnL */}
          <div>
            <div className="text-xs text-gray-500">Unrealized PnL</div>
            <div className={`font-semibold ${getPositionColor(totalUnrealizedPnl)}`}>
              {formatUSD(totalUnrealizedPnl)}
            </div>
          </div>

          {/* Refresh Button */}
          <button
            onClick={() => {
              fetchAccountInfo();
              fetchPositions();
            }}
            className="p-2 hover:bg-gray-700 rounded"
            title="Refresh"
          >
            <RefreshCw className={`w-4 h-4 text-gray-400 ${isLoading ? 'animate-spin' : ''}`} />
          </button>

          {/* Panic Button */}
          <PanicButton type="futures" onComplete={() => fetchPositions()} />
        </div>
      </div>

      {/* Main Grid Layout */}
      <div className="grid grid-cols-12 gap-4">
        {/* Left Column - Trading Panel & Autopilot */}
        <div className="col-span-12 lg:col-span-3 space-y-4">
          <FuturesTradingPanel />
          <FuturesAutopilotPanel />
        </div>

        {/* Center Column - Order Book / Chart Toggle */}
        <div className="col-span-12 lg:col-span-3 space-y-4">
          {/* View Toggle */}
          <div className="flex bg-gray-800 rounded-lg p-1">
            <button
              onClick={() => setCenterView('orderbook')}
              className={`flex-1 flex items-center justify-center gap-2 py-2 rounded-md text-sm font-medium transition-colors ${
                centerView === 'orderbook'
                  ? 'bg-gray-700 text-white'
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              <BookOpen className="w-4 h-4" />
              Order Book
            </button>
            <button
              onClick={() => setCenterView('chart')}
              className={`flex-1 flex items-center justify-center gap-2 py-2 rounded-md text-sm font-medium transition-colors ${
                centerView === 'chart'
                  ? 'bg-gray-700 text-white'
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              <BarChart3 className="w-4 h-4" />
              Chart
            </button>
          </div>

          {/* Content */}
          <div className="h-[500px]">
            {centerView === 'orderbook' ? <FuturesOrderBook /> : <FuturesChart />}
          </div>

          {/* AI Signals Panel */}
          <FuturesAISignals />
        </div>

        {/* Right Column - Stats/Info */}
        <div className="col-span-12 lg:col-span-6 space-y-4">
          {/* Quick Stats */}
          <div className="grid grid-cols-4 gap-4">
            <StatCard
              title="Total Trades"
              value={metrics?.totalTrades || 0}
              icon={Activity}
            />
            <StatCard
              title="Win Rate"
              value={`${(metrics?.winRate || 0).toFixed(1)}%`}
              icon={TrendingUp}
              valueColor={metrics?.winRate && metrics.winRate >= 50 ? 'text-green-500' : 'text-red-500'}
            />
            <StatCard
              title="Realized PnL"
              value={formatUSD(metrics?.totalRealizedPnl || 0)}
              icon={Wallet}
              valueColor={getPositionColor(metrics?.totalRealizedPnl || 0)}
            />
            <StatCard
              title="Funding Fees"
              value={formatUSD(metrics?.totalFundingFees || 0)}
              icon={Activity}
              valueColor={getPositionColor(metrics?.totalFundingFees || 0)}
            />
          </div>

          {/* Positions Table */}
          <FuturesPositionsTable />
        </div>
      </div>

      {/* Bottom Section - History Tabs */}
      <div className="mt-4">
        <FuturesHistoryTabs />
      </div>
    </div>
  );
}

// Stat Card Component
function StatCard({
  title,
  value,
  icon: Icon,
  valueColor = 'text-white',
}: {
  title: string;
  value: string | number;
  icon: any;
  valueColor?: string;
}) {
  return (
    <div className="bg-gray-900 rounded-lg border border-gray-700 p-4">
      <div className="flex items-center justify-between mb-2">
        <span className="text-xs text-gray-500">{title}</span>
        <Icon className="w-4 h-4 text-gray-600" />
      </div>
      <div className={`text-lg font-semibold ${valueColor}`}>{value}</div>
    </div>
  );
}
