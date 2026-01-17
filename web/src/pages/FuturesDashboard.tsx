import { useEffect, useState, useCallback } from 'react';
import { useFuturesStore, selectTotalUnrealizedPnl } from '../store/futuresStore';
import { wsService } from '../services/websocket';
import type { WSEvent } from '../types';
import FuturesTradingPanel from '../components/FuturesTradingPanel';
import FuturesOrderBook from '../components/FuturesOrderBook';
import FuturesChart from '../components/FuturesChart';
import FuturesPositionsTable from '../components/FuturesPositionsTable';
import FuturesOrdersHistory from '../components/FuturesOrdersHistory';
import GiniePanel from '../components/GiniePanel';
import InstanceControlPanel from '../components/InstanceControlPanel';
import CollapsibleCard from '../components/CollapsibleCard';
import PanicButton from '../components/PanicButton';
import NewsDashboard from '../components/NewsDashboard';
import TradeSourceStatsPanel from '../components/TradeSourceStatsPanel';
import TradingModeToggle from '../components/TradingModeToggle';
import ModeAllocationPanel from '../components/ModeAllocationPanel';
import ModeSafetyPanel from '../components/ModeSafetyPanel';
import { TradeLifecycleTab } from '../components/TradeLifecycle';
import AccountStatsCard from '../components/AccountStatsCard';
import { formatUSD, getPositionColor } from '../services/futuresApi';
import {
  Activity,
  ChevronDown,
  RefreshCw,
  AlertCircle,
  BarChart3,
  BookOpen,
  Zap,
  Brain,
  Sparkles,
  Layers,
  Shield,
  ShoppingCart,
  Wallet,
  TrendingUp,
  Wifi,
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

  const totalUnrealizedPnl = useFuturesStore(selectTotalUnrealizedPnl);
  const [wsConnected, setWsConnected] = useState(() => wsService.isConnected());
  const [showSymbolDropdown, setShowSymbolDropdown] = useState(false);
  const [centerView, setCenterView] = useState<'orderbook' | 'chart'>('orderbook');
  const [tradingMode, setTradingMode] = useState<{ mode: 'paper' | 'live'; mode_label: string } | null>(null);

  // Handle trading mode changes from TradingModeToggle component
  const handleTradingModeChange = (mode: { mode: 'paper' | 'live'; mode_label: string }) => {
    setTradingMode(mode);
    // CRITICAL: Immediately refresh all data when mode changes
    console.log('Trading mode changed, refreshing all data...');
    fetchAccountInfo();
    fetchPositions();
    fetchMetrics();
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

  // NOTE: Account/position/metrics updates are now handled by:
  // - FuturesPositionsTable: subscribes to POSITION_UPDATE, BALANCE_UPDATE
  // - GiniePanel: subscribes to GINIE_STATUS_UPDATE, POSITION_UPDATE, BALANCE_UPDATE
  // - AccountStatsCard: subscribes to BALANCE_UPDATE, POSITION_UPDATE
  // - FallbackPollingManager (60s) handles disconnection scenarios
  // No separate polling needed here - child components handle their own data.

  // WebSocket subscription for real-time balance updates in header
  useEffect(() => {
    const handleBalanceUpdate = (event: WSEvent) => {
      // Trigger refetch for balance updates
      fetchAccountInfo();
    };

    const handlePositionUpdate = (event: WSEvent) => {
      // Position updates affect unrealized PnL which affects margin balance
      fetchPositions();
    };

    wsService.subscribe('BALANCE_UPDATE', handleBalanceUpdate);
    wsService.subscribe('POSITION_UPDATE', handlePositionUpdate);

    // Track connection status
    const handleConnect = () => setWsConnected(true);
    const handleDisconnect = () => setWsConnected(false);
    wsService.onConnect(handleConnect);
    wsService.onDisconnect(handleDisconnect);

    return () => {
      wsService.unsubscribe('BALANCE_UPDATE', handleBalanceUpdate);
      wsService.unsubscribe('POSITION_UPDATE', handlePositionUpdate);
    };
  }, [fetchAccountInfo, fetchPositions]);

  // Safely parse values that may come as strings from API
  const safeNum = (val: number | string | null | undefined): number => {
    if (val === null || val === undefined) return 0;
    const num = typeof val === 'string' ? parseFloat(val) : val;
    return isNaN(num) ? 0 : num;
  };

  const walletBalance = safeNum(accountInfo?.total_wallet_balance);
  const marginBalance = walletBalance + totalUnrealizedPnl;
  const currentPrice = safeNum(markPrice?.markPrice);
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

      {/* Symbol & Price Bar */}
      <div className="mb-4 bg-gray-800 rounded-lg p-3">
        <div className="flex items-center justify-between">
          {/* Left side: Symbol, Price, Funding */}
          <div className="flex items-center gap-4">
            {/* Symbol Selector */}
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
            <div className="text-2xl font-bold">
              {currentPrice > 0 ? formatUSD(currentPrice) : '-'}
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

          {/* Right side: Wallet & Margin Balance */}
          <div className="flex items-center gap-4">
            {/* Wallet Balance */}
            <div className="flex items-center gap-2 px-3 py-1.5 bg-gray-700/50 rounded-lg border border-gray-600">
              <Wallet className="w-4 h-4 text-gray-400" />
              <div className="flex flex-col">
                <span className="text-[10px] text-gray-500 leading-none">Wallet</span>
                <span className="text-sm font-bold text-white">{formatUSD(walletBalance)}</span>
              </div>
              {wsConnected && <Wifi className="w-3 h-3 text-green-500" title="Real-time" />}
            </div>

            {/* Margin Balance */}
            <div className="flex items-center gap-2 px-3 py-1.5 bg-gray-700/50 rounded-lg border border-gray-600">
              <TrendingUp className="w-4 h-4 text-gray-400" />
              <div className="flex flex-col">
                <span className="text-[10px] text-gray-500 leading-none">Margin</span>
                <span className={`text-sm font-bold ${
                  marginBalance > walletBalance ? 'text-green-500' : marginBalance < walletBalance ? 'text-red-500' : 'text-white'
                }`}>
                  {formatUSD(marginBalance)}
                </span>
              </div>
              {wsConnected && <Wifi className="w-3 h-3 text-green-500" title="Real-time" />}
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
        {/* Instance Control Panel - Active/Standby control for multi-instance setup */}
        <InstanceControlPanel />
        <GiniePanel />
      </div>

      {/* Account Summary & P&L - Collapsible Stats Card */}
      <div className="mb-4">
        <AccountStatsCard />
      </div>

      {/* Positions Table */}
      <div className="mb-4">
        <FuturesPositionsTable
          onSymbolClick={(symbol) => {
            setSelectedSymbol(symbol);
            setCenterView('chart');
          }}
        />
      </div>

      {/* Order Chains / Trade Lifecycle - Between Positions and Orders */}
      <div className="mb-4">
        <CollapsibleCard
          title="Order Chains"
          icon={<Layers className="w-4 h-4" />}
          defaultExpanded={false}
          badge="Lifecycle"
          badgeColor="purple"
        >
          <TradeLifecycleTab />
        </CollapsibleCard>
      </div>

      {/* Open Orders & Trade History */}
      <div className="mb-4">
        <FuturesOrdersHistory />
      </div>

      {/* Trade Source Performance Stats - Below Orders */}
      <div className="mb-4">
        <CollapsibleCard
          title="Trade Source Performance"
          icon={<BarChart3 className="w-4 h-4" />}
          defaultExpanded={false}
          badge="Stats"
          badgeColor="cyan"
        >
          <TradeSourceStatsPanel />
        </CollapsibleCard>
      </div>

      {/* Mode Capital Allocation */}
      <div className="mb-4">
        <CollapsibleCard
          title="Mode Capital Allocation"
          icon={<Layers className="w-4 h-4" />}
          defaultExpanded={false}
          badge="Capital"
          badgeColor="blue"
        >
          <ModeAllocationPanel />
        </CollapsibleCard>
      </div>

      {/* Mode Safety Settings */}
      <div className="mb-4">
        <CollapsibleCard
          title="Mode Safety Settings"
          icon={<Shield className="w-4 h-4" />}
          defaultExpanded={false}
          badge="Safety"
          badgeColor="yellow"
        >
          <ModeSafetyPanel />
        </CollapsibleCard>
      </div>

      {/* Manual Trading Section - 3 Horizontal Cards */}
      <div className="mb-4">
        <CollapsibleCard
          title="Manual Trading"
          icon={<ShoppingCart className="w-4 h-4" />}
          defaultExpanded={false}
          badge="Trade"
          badgeColor="green"
        >
          {/* 3 Cards in a Row: Live Trading | Order Book/Chart | Stats */}
          <div className="grid grid-cols-12 gap-3">
            {/* Card 1: Live Trading (3 cols - smaller width) */}
            <div className="col-span-3 bg-gray-800 border border-gray-700 rounded-lg p-3">
              <div className="flex items-center gap-2 mb-2 pb-2 border-b border-gray-700">
                <Zap className="w-4 h-4 text-yellow-400" />
                <h4 className="text-sm font-semibold text-white">Live Trading</h4>
              </div>
              <FuturesTradingPanel />
            </div>

            {/* Card 2: Order Book / Chart (3 cols - same width as Card 1) - No Scroll */}
            <div className="col-span-3 bg-gray-800 border border-gray-700 rounded-lg p-3">
              <div className="flex items-center justify-between mb-2 pb-2 border-b border-gray-700">
                <div className="flex items-center gap-2">
                  <BookOpen className="w-4 h-4 text-blue-400" />
                  <h4 className="text-sm font-semibold text-white">Market Data</h4>
                </div>
                <div className="flex bg-gray-900 rounded-lg p-0.5">
                  <button
                    onClick={() => setCenterView('orderbook')}
                    className={`flex items-center gap-1 px-1.5 py-1 rounded text-[10px] font-medium transition-colors ${
                      centerView === 'orderbook'
                        ? 'bg-gray-700 text-white'
                        : 'text-gray-400 hover:text-white'
                    }`}
                  >
                    Book
                  </button>
                  <button
                    onClick={() => setCenterView('chart')}
                    className={`flex items-center gap-1 px-1.5 py-1 rounded text-[10px] font-medium transition-colors ${
                      centerView === 'chart'
                        ? 'bg-gray-700 text-white'
                        : 'text-gray-400 hover:text-white'
                    }`}
                  >
                    Chart
                  </button>
                </div>
              </div>
              {/* Fixed height - no scroll */}
              <div>
                {centerView === 'orderbook' ? <FuturesOrderBook /> : <FuturesChart />}
              </div>
            </div>

            {/* Card 3: Trading Stats (6 cols - remaining space) */}
            <div className="col-span-6 bg-gray-800 border border-gray-700 rounded-lg p-3">
              <div className="flex items-center gap-2 mb-2 pb-2 border-b border-gray-700">
                <Activity className="w-4 h-4 text-cyan-400" />
                <h4 className="text-sm font-semibold text-white">Trading Stats</h4>
              </div>
              {/* Stats Grid - 2 rows */}
              <div className="space-y-2">
                {/* Row 1: 6 stats */}
                <div className="grid grid-cols-6 gap-2">
                  <div className="bg-gray-900 rounded p-2 text-center">
                    <div className="text-[9px] text-gray-500">Total Trades</div>
                    <div className="text-sm font-bold text-white">{metrics?.totalTrades || 0}</div>
                  </div>
                  <div className="bg-gray-900 rounded p-2 text-center">
                    <div className="text-[9px] text-gray-500">Win Rate</div>
                    <div className={`text-sm font-bold ${metrics?.winRate && metrics.winRate >= 50 ? 'text-green-500' : 'text-red-500'}`}>
                      {(metrics?.winRate || 0).toFixed(1)}%
                    </div>
                  </div>
                  <div className="bg-gray-900 rounded p-2 text-center">
                    <div className="text-[9px] text-gray-500">Realized (7d)</div>
                    <div className={`text-sm font-bold ${getPositionColor(metrics?.totalRealizedPnl || 0)}`}>
                      {formatUSD(metrics?.totalRealizedPnl || 0)}
                    </div>
                  </div>
                  <div className="bg-gray-900 rounded p-2 text-center">
                    <div className="text-[9px] text-gray-500">Today's PnL</div>
                    <div className={`text-sm font-bold ${getPositionColor(metrics?.dailyRealizedPnl || 0)}`}>
                      {formatUSD(metrics?.dailyRealizedPnl || 0)}
                    </div>
                  </div>
                  <div className="bg-gray-900 rounded p-2 text-center">
                    <div className="text-[9px] text-gray-500">ROI</div>
                    <div className={`text-sm font-bold ${getPositionColor(metrics?.totalRealizedPnl || 0)}`}>
                      {walletBalance > 0 ? `${(((metrics?.totalRealizedPnl || 0) / walletBalance) * 100).toFixed(1)}%` : '0%'}
                    </div>
                  </div>
                  <div className="bg-gray-900 rounded p-2 text-center">
                    <div className="text-[9px] text-gray-500">Funding Fees</div>
                    <div className={`text-sm font-bold ${getPositionColor(metrics?.totalFundingFees || 0)}`}>
                      {formatUSD(metrics?.totalFundingFees || 0)}
                    </div>
                  </div>
                </div>
                {/* Row 2: 4 daily stats */}
                <div className="grid grid-cols-4 gap-2">
                  <div className="bg-gray-900 rounded p-2 text-center">
                    <div className="text-[9px] text-gray-500">Today's Trades</div>
                    <div className="text-sm font-bold text-white">{metrics?.dailyTrades || 0}</div>
                  </div>
                  <div className="bg-gray-900 rounded p-2 text-center">
                    <div className="text-[9px] text-gray-500">Today's Win %</div>
                    <div className={`text-sm font-bold ${metrics?.dailyWinRate && metrics.dailyWinRate >= 50 ? 'text-green-500' : 'text-red-500'}`}>
                      {(metrics?.dailyWinRate || 0).toFixed(1)}%
                    </div>
                  </div>
                  <div className="bg-gray-900 rounded p-2 text-center">
                    <div className="text-[9px] text-gray-500">Today's Wins</div>
                    <div className="text-sm font-bold text-green-500">{metrics?.dailyWins || 0}</div>
                  </div>
                  <div className="bg-gray-900 rounded p-2 text-center">
                    <div className="text-[9px] text-gray-500">Today's Losses</div>
                    <div className="text-sm font-bold text-red-500">{metrics?.dailyLosses || 0}</div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </CollapsibleCard>
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
  tooltip,
}: {
  title: string;
  value: string | number;
  icon: any;
  valueColor?: string;
  tooltip?: string;
}) {
  return (
    <div className="bg-gray-900 rounded-lg border border-gray-700 p-4" title={tooltip}>
      <div className="flex items-center justify-between mb-2">
        <span className="text-xs text-gray-500">{title}</span>
        <Icon className="w-4 h-4 text-gray-600" />
      </div>
      <div className={`text-lg font-semibold ${valueColor}`}>{value}</div>
    </div>
  );
}
