import { useEffect, useState } from 'react';
import { useFuturesStore, selectAvailableBalance, selectTotalMarginUsed, selectTotalUnrealizedPnl } from '../store/futuresStore';
import FuturesTradingPanel from '../components/FuturesTradingPanel';
import FuturesOrderBook from '../components/FuturesOrderBook';
import FuturesChart from '../components/FuturesChart';
import FuturesPositionsTable from '../components/FuturesPositionsTable';
import FuturesOrdersHistory from '../components/FuturesOrdersHistory';
import GiniePanel from '../components/GiniePanel';
import PanicButton from '../components/PanicButton';
import NewsDashboard from '../components/NewsDashboard';
import TradeSourceStatsPanel from '../components/TradeSourceStatsPanel';
import TradingModeToggle from '../components/TradingModeToggle';
import ModeAllocationPanel from '../components/ModeAllocationPanel';
import ModeSafetyPanel from '../components/ModeSafetyPanel';
import { formatUSD, formatPercent, getPositionColor } from '../services/futuresApi';
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
  Brain,
  Percent,
  Sparkles,
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

  // Handle trading mode changes from TradingModeToggle component
  const handleTradingModeChange = (mode: { mode: 'paper' | 'live'; mode_label: string }) => {
    setTradingMode(mode);
  };

  // Initial data fetch
  useEffect(() => {
    fetchSymbols();
    fetchAccountInfo();
    fetchPositionMode();
    fetchMetrics();
  }, [fetchSymbols, fetchAccountInfo, fetchPositionMode, fetchMetrics]);

  // Fetch data for selected symbol
  // NOTE: Mark price and funding rate are cached from WebSocket, so longer intervals are fine
  useEffect(() => {
    fetchMarkPrice(selectedSymbol);
    fetchFundingRate(selectedSymbol);

    // Mark price: 60s (WebSocket updates cache in real-time)
    const markPriceInterval = setInterval(() => {
      fetchMarkPrice(selectedSymbol);
    }, 60000);

    // Funding rate: 5 minutes (only changes every 8 hours, cached for 5 min)
    const fundingInterval = setInterval(() => {
      fetchFundingRate(selectedSymbol);
    }, 300000);

    return () => {
      clearInterval(markPriceInterval);
      clearInterval(fundingInterval);
    };
  }, [selectedSymbol, fetchMarkPrice, fetchFundingRate]);

  // Periodic account/position refresh
  useEffect(() => {
    const interval = setInterval(() => {
      fetchAccountInfo();
      fetchPositions();
    }, 60000); // Reduced to 60s to avoid rate limits

    return () => clearInterval(interval);
  }, [fetchAccountInfo, fetchPositions]);

  // Safely parse values that may come as strings from API
  const safeNum = (val: number | string | null | undefined): number => {
    if (val === null || val === undefined) return 0;
    const num = typeof val === 'string' ? parseFloat(val) : val;
    return isNaN(num) ? 0 : num;
  };

  const walletBalance = safeNum(accountInfo?.totalWalletBalance);
  const currentPrice = safeNum(markPrice?.markPrice);
  const priceChange24h = 0; // Would come from ticker data
  const currentFundingRate = safeNum(fundingRate?.fundingRate);
  const nextFundingTime = safeNum(fundingRate?.nextFundingTime);

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

      {/* Header Row - Trading Mode & Actions */}
      <div className="mb-4 flex items-center justify-between gap-4 flex-wrap">
        {/* Left side - Trading Mode & Live/Paper Indicator */}
        <div className="flex items-center gap-4">
          <TradingModeToggle onModeChange={handleTradingModeChange} />

          {/* Live/Paper Indicator */}
          {tradingMode && (
            <div className={`
              flex items-center gap-2 px-4 py-2 rounded-lg font-bold text-sm
              ${tradingMode.mode === 'live'
                ? 'bg-green-500/20 border-2 border-green-500 text-green-400'
                : 'bg-yellow-500/20 border-2 border-yellow-500 text-yellow-400'
              }
            `}>
              <span className={`w-3 h-3 rounded-full animate-pulse ${tradingMode.mode === 'live' ? 'bg-green-500' : 'bg-yellow-500'}`} />
              {tradingMode.mode === 'live' ? (
                <>
                  <Zap className="w-4 h-4" />
                  LIVE TRADING
                </>
              ) : (
                <>
                  <AlertCircle className="w-4 h-4" />
                  PAPER TRADING
                </>
              )}
            </div>
          )}

          {/* Futures Trading Badge */}
          <div className="flex items-center gap-2 px-3 py-2 bg-purple-500/20 border border-purple-500/50 rounded-lg">
            <Sparkles className="w-4 h-4 text-purple-400" />
            <span className="text-purple-400 font-semibold text-sm">FUTURES TRADING</span>
          </div>
        </div>

        {/* Right side - Panic Button & Refresh */}
        <div className="flex gap-3 flex-wrap">
          <button
            onClick={() => {
              fetchAccountInfo();
              fetchPositions();
            }}
            className="bg-gray-700 hover:bg-gray-600 text-white px-4 py-2 rounded-lg font-medium transition-colors flex items-center"
            title="Refresh"
          >
            <RefreshCw className={`w-4 h-4 mr-2 ${isLoading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <PanicButton type="futures" onComplete={() => fetchPositions()} />
        </div>
      </div>

      {/* Account Summary Bar */}
      <div className="mb-4 bg-gray-800 rounded-lg p-4">
        <div className="flex flex-wrap items-center justify-between gap-4">
          {/* Symbol Selector */}
          <div className="flex items-center gap-4">
            <div className="relative">
              <button
                onClick={() => setShowSymbolDropdown(!showSymbolDropdown)}
                className="flex items-center gap-2 px-4 py-2 bg-gray-700 hover:bg-gray-600 rounded-lg border border-gray-600"
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
                    onChange={() => {}}
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

          {/* Account Stats */}
          <div className="flex items-center gap-6">
            <div className="flex items-center gap-2">
              <Wallet className="w-4 h-4 text-gray-400" />
              <div>
                <div className="text-xs text-gray-500">Wallet Balance</div>
                <div className="font-semibold">{formatUSD(walletBalance)}</div>
              </div>
            </div>
            <div>
              <div className="text-xs text-gray-500">Available</div>
              <div className="font-semibold text-green-500">{formatUSD(availableBalance)}</div>
            </div>
            <div>
              <div className="text-xs text-gray-500">Margin Used</div>
              <div className="font-semibold text-yellow-500">{formatUSD(marginUsed)}</div>
            </div>
            <div>
              <div className="text-xs text-gray-500">Unrealized PnL</div>
              <div className={`font-semibold ${getPositionColor(totalUnrealizedPnl)}`}>
                {formatUSD(totalUnrealizedPnl)}
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* News Dashboard */}
      <div className="mb-4">
        <NewsDashboard />
      </div>

      {/* Ginie AI Trader - Primary Futures Trading AI */}
      <div className="mb-4 bg-gradient-to-br from-purple-900/40 to-blue-900/40 rounded-xl border border-purple-500/30 p-6">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-purple-500/20 rounded-lg">
              <Brain className="w-6 h-6 text-purple-400" />
            </div>
            <div>
              <h2 className="text-xl font-bold text-white">Ginie AI Trader</h2>
              <p className="text-sm text-gray-400">Autonomous futures trading with AI signals</p>
            </div>
          </div>
        </div>
        <GiniePanel />
      </div>

      {/* Mode Capital Allocation & Safety Control */}
      <div className="mb-4 grid grid-cols-1 lg:grid-cols-2 gap-4">
        <ModeAllocationPanel />
        <ModeSafetyPanel />
      </div>

      {/* Main Grid Layout */}
      <div className="grid grid-cols-12 gap-4">
        {/* Left Column - Trading Panel */}
        <div className="col-span-12 lg:col-span-3 space-y-4">
          <FuturesTradingPanel />
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
        </div>

        {/* Right Column - Stats/Info */}
        <div className="col-span-12 lg:col-span-6 space-y-4">
          {/* Quick Stats - Row 1 */}
          <div className="grid grid-cols-6 gap-4">
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
              title="Total PnL"
              value={formatUSD(metrics?.totalRealizedPnl || 0)}
              icon={Wallet}
              valueColor={getPositionColor(metrics?.totalRealizedPnl || 0)}
            />
            <StatCard
              title="Today's PnL"
              value={formatUSD(metrics?.dailyRealizedPnl || 0)}
              icon={TrendingUp}
              valueColor={getPositionColor(metrics?.dailyRealizedPnl || 0)}
            />
            <StatCard
              title="ROI"
              value={walletBalance > 0 ? `${(((metrics?.totalRealizedPnl || 0) / walletBalance) * 100).toFixed(2)}%` : '0%'}
              icon={Percent}
              valueColor={getPositionColor(metrics?.totalRealizedPnl || 0)}
            />
            <StatCard
              title="Funding Fees"
              value={formatUSD(metrics?.totalFundingFees || 0)}
              icon={Activity}
              valueColor={getPositionColor(metrics?.totalFundingFees || 0)}
            />
          </div>

          {/* Daily Stats - Row 2 */}
          <div className="grid grid-cols-4 gap-4">
            <StatCard
              title="Today's Trades"
              value={metrics?.dailyTrades || 0}
              icon={Activity}
            />
            <StatCard
              title="Today's Win Rate"
              value={`${(metrics?.dailyWinRate || 0).toFixed(1)}%`}
              icon={TrendingUp}
              valueColor={metrics?.dailyWinRate && metrics.dailyWinRate >= 50 ? 'text-green-500' : 'text-red-500'}
            />
            <StatCard
              title="Today's Wins"
              value={metrics?.dailyWins || 0}
              icon={TrendingUp}
              valueColor="text-green-500"
            />
            <StatCard
              title="Today's Losses"
              value={metrics?.dailyLosses || 0}
              icon={TrendingDown}
              valueColor="text-red-500"
            />
          </div>

          {/* Trade Source Performance Stats */}
          <TradeSourceStatsPanel />

          {/* Positions Table */}
          <FuturesPositionsTable
            onSymbolClick={(symbol) => {
              setSelectedSymbol(symbol);
              setCenterView('chart');
            }}
          />

          {/* Open Orders & Trade History */}
          <FuturesOrdersHistory />

        </div>
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
