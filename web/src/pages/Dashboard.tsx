import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useStore } from '../store';
import { TrendingUp, TrendingDown, Activity, DollarSign, Bell, Settings, Sparkles, ArrowRight, Brain, History, Zap, AlertCircle } from 'lucide-react';
import PositionsTable from '../components/PositionsTable';
import OrdersTable from '../components/OrdersTable';
import StrategiesPanel from '../components/StrategiesPanel';
import ScreenerResults from '../components/ScreenerResults';
import EnhancedSignalsPanel from '../components/EnhancedSignalsPanel';
import PendingSignalsModal from '../components/PendingSignalsModal';
import StrategyConfigModal from '../components/StrategyConfigModal';
import StrategyScanner from '../components/StrategyScanner';
import AISignalsPanel from '../components/AISignalsPanel';
import TradeHistory from '../components/TradeHistory';
import TradingModeToggle from '../components/TradingModeToggle';
import WalletBalanceCard from '../components/WalletBalanceCard';
import AutopilotRulesPanel from '../components/AutopilotRulesPanel';
import PanicButton from '../components/PanicButton';
import FuturesPositionsTable from '../components/FuturesPositionsTable';
import { futuresApi } from '../services/futuresApi';
import { apiService } from '../services/api';
import type { FuturesPosition } from '../types/futures';

export default function Dashboard() {
  const { metrics, positions, activeOrders } = useStore();
  const navigate = useNavigate();
  const [showPendingSignalsModal, setShowPendingSignalsModal] = useState(false);
  const [showStrategyConfigModal, setShowStrategyConfigModal] = useState(false);
  const [futuresPositions, setFuturesPositions] = useState<FuturesPosition[]>([]);
  const [tradingMode, setTradingMode] = useState<{ mode: 'paper' | 'live'; mode_label: string } | null>(null);
  const [positionsView, setPositionsView] = useState<'spot' | 'futures' | 'all'>('all');

  // Fetch futures positions and trading mode
  useEffect(() => {
    const fetchData = async () => {
      try {
        const [futuresPos, modeData] = await Promise.all([
          futuresApi.getPositions(),
          apiService.getTradingMode(),
        ]);
        setFuturesPositions(futuresPos);
        setTradingMode(modeData);
      } catch (error) {
        console.error('Error fetching data:', error);
      }
    };

    fetchData();
    const interval = setInterval(fetchData, 5000);
    return () => clearInterval(interval);
  }, []);

  const refreshPositions = async () => {
    try {
      const futuresPos = await futuresApi.getPositions();
      setFuturesPositions(futuresPos);
    } catch (error) {
      console.error('Error refreshing positions:', error);
    }
  };

  const formatCurrency = (value: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(value);
  };

  return (
    <div className="space-y-6">
      {/* Visual Strategy Builder Highlight */}
      <div className="bg-gradient-to-r from-blue-600 to-purple-600 rounded-lg p-6 shadow-xl border border-blue-500/50">
        <div className="flex items-center justify-between">
          <div className="flex-1">
            <div className="flex items-center gap-3 mb-2">
              <Sparkles className="w-8 h-8 text-yellow-300" />
              <h2 className="text-2xl font-bold text-white">Visual Strategy Builder</h2>
              <span className="px-2 py-1 bg-green-500 text-white text-xs font-semibold rounded-full">NEW</span>
            </div>
            <p className="text-blue-100 mb-4 max-w-2xl">
              Create advanced trading strategies with drag-and-drop nodes, visual condition builder,
              multiple indicators, backtesting, and chart visualization. Build complex logic with AND/OR
              conditions like "Buy when LTP &gt;= EMA(20) AND RSI &lt; 30".
            </p>
            <div className="flex flex-wrap gap-2 text-sm text-blue-100">
              <span className="px-3 py-1 bg-white/10 rounded-full">✓ 9 Technical Indicators</span>
              <span className="px-3 py-1 bg-white/10 rounded-full">✓ Visual Conditions</span>
              <span className="px-3 py-1 bg-white/10 rounded-full">✓ Backtesting</span>
              <span className="px-3 py-1 bg-white/10 rounded-full">✓ Chart Integration</span>
              <span className="px-3 py-1 bg-white/10 rounded-full">✓ Risk Management</span>
            </div>
          </div>
          <button
            onClick={() => navigate('/visual-strategy-advanced')}
            className="bg-white hover:bg-gray-100 text-blue-600 px-6 py-3 rounded-lg font-bold transition-all hover:scale-105 flex items-center gap-2 shadow-lg ml-6"
          >
            Launch Builder
            <ArrowRight className="w-5 h-5" />
          </button>
        </div>
      </div>

      {/* Trading Mode, Live/Paper Indicator & Action Buttons */}
      <div className="flex items-center justify-between gap-4 flex-wrap">
        {/* Left side - Trading Mode & Live/Paper Indicator */}
        <div className="flex items-center gap-4">
          <TradingModeToggle />

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
        </div>

        {/* Right side - Action Buttons & Panic Button */}
        <div className="flex gap-3 flex-wrap">
          <button
            onClick={() => setShowStrategyConfigModal(true)}
            className="bg-primary-600 hover:bg-primary-700 text-white px-4 py-2 rounded-lg font-semibold transition-colors flex items-center"
          >
            <Settings className="w-5 h-5 mr-2" />
            Configure Strategies
          </button>
          <button
            onClick={() => setShowPendingSignalsModal(true)}
            className="bg-yellow-600 hover:bg-yellow-700 text-white px-4 py-2 rounded-lg font-semibold transition-colors flex items-center animate-pulse"
          >
            <Bell className="w-5 h-5 mr-2" />
            Pending Signals
          </button>

          {/* Panic Button - Close All Positions */}
          <PanicButton type="all" onComplete={refreshPositions} />
        </div>
      </div>

      {/* Wallet Balance & Autopilot Control - Top Section */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <WalletBalanceCard />
        <AutopilotRulesPanel />
      </div>

      {/* Metrics Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="metric-card">
          <div className="flex items-center justify-between">
            <div>
              <div className="metric-label">Total P&L</div>
              <div
                className={`metric-value ${
                  (metrics?.total_pnl || 0) >= 0 ? 'text-positive' : 'text-negative'
                }`}
              >
                {formatCurrency(metrics?.total_pnl || 0)}
              </div>
            </div>
            <DollarSign className="w-8 h-8 text-primary-500" />
          </div>
        </div>

        <div className="metric-card">
          <div className="flex items-center justify-between">
            <div>
              <div className="metric-label">Win Rate</div>
              <div className="metric-value text-primary-400">
                {(metrics?.win_rate || 0).toFixed(1)}%
              </div>
              <div className="text-xs text-gray-400 mt-1">
                {metrics?.winning_trades || 0}W / {metrics?.losing_trades || 0}L
              </div>
            </div>
            <Activity className="w-8 h-8 text-primary-500" />
          </div>
        </div>

        <div className="metric-card">
          <div className="flex items-center justify-between">
            <div>
              <div className="metric-label">Open Positions</div>
              <div className="metric-value">{positions.length}</div>
              <div className="text-xs text-gray-400 mt-1">
                {activeOrders.length} active orders
              </div>
            </div>
            <TrendingUp className="w-8 h-8 text-success" />
          </div>
        </div>

        <div className="metric-card">
          <div className="flex items-center justify-between">
            <div>
              <div className="metric-label">Total Trades</div>
              <div className="metric-value">{metrics?.total_trades || 0}</div>
              <div className="text-xs text-gray-400 mt-1">
                PF: {(metrics?.profit_factor || 0).toFixed(2)}
              </div>
            </div>
            <TrendingDown className="w-8 h-8 text-gray-400" />
          </div>
        </div>
      </div>

      {/* Performance Stats */}
      {metrics && (
        <div className="card">
          <div className="card-header">
            <h2 className="text-lg font-semibold">Performance Statistics</h2>
          </div>
          <div className="card-body">
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <div>
                <div className="text-sm text-gray-400">Average Win</div>
                <div className="text-lg font-semibold text-positive">
                  {formatCurrency(metrics.average_win)}
                </div>
              </div>
              <div>
                <div className="text-sm text-gray-400">Average Loss</div>
                <div className="text-lg font-semibold text-negative">
                  {formatCurrency(metrics.average_loss)}
                </div>
              </div>
              <div>
                <div className="text-sm text-gray-400">Largest Win</div>
                <div className="text-lg font-semibold text-positive">
                  {formatCurrency(metrics.largest_win)}
                </div>
              </div>
              <div>
                <div className="text-sm text-gray-400">Largest Loss</div>
                <div className="text-lg font-semibold text-negative">
                  {formatCurrency(metrics.largest_loss)}
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Positions - Spot & Futures */}
      <div className="card">
        <div className="card-header flex items-center justify-between">
          <div className="flex items-center gap-4">
            <h2 className="text-lg font-semibold">Open Positions</h2>
            {/* Position counts */}
            <div className="flex items-center gap-2 text-sm">
              <span className="px-2 py-1 bg-blue-500/20 text-blue-400 rounded">
                Spot: {positions.length}
              </span>
              <span className="px-2 py-1 bg-purple-500/20 text-purple-400 rounded">
                Futures: {futuresPositions.length}
              </span>
            </div>
          </div>
          {/* View Toggle */}
          <div className="flex items-center gap-1 bg-gray-700 rounded-lg p-1">
            <button
              onClick={() => setPositionsView('all')}
              className={`px-3 py-1 rounded text-sm font-medium transition-colors ${
                positionsView === 'all' ? 'bg-primary-600 text-white' : 'text-gray-400 hover:text-white'
              }`}
            >
              All
            </button>
            <button
              onClick={() => setPositionsView('spot')}
              className={`px-3 py-1 rounded text-sm font-medium transition-colors ${
                positionsView === 'spot' ? 'bg-blue-600 text-white' : 'text-gray-400 hover:text-white'
              }`}
            >
              Spot
            </button>
            <button
              onClick={() => setPositionsView('futures')}
              className={`px-3 py-1 rounded text-sm font-medium transition-colors ${
                positionsView === 'futures' ? 'bg-purple-600 text-white' : 'text-gray-400 hover:text-white'
              }`}
            >
              Futures
            </button>
          </div>
        </div>
        <div className="card-body p-0">
          {/* Spot Positions */}
          {(positionsView === 'all' || positionsView === 'spot') && (
            <div>
              {positionsView === 'all' && positions.length > 0 && (
                <div className="px-4 py-2 bg-blue-500/10 border-b border-gray-700 flex items-center gap-2">
                  <span className="text-blue-400 font-semibold text-sm">SPOT POSITIONS</span>
                  <span className="px-2 py-0.5 bg-blue-500/20 text-blue-300 rounded text-xs">
                    {tradingMode?.mode === 'live' ? 'LIVE' : 'PAPER'}
                  </span>
                </div>
              )}
              <PositionsTable />
            </div>
          )}

          {/* Futures Positions */}
          {(positionsView === 'all' || positionsView === 'futures') && (
            <div>
              {positionsView === 'all' && (
                <div className="px-4 py-2 bg-purple-500/10 border-b border-gray-700 flex items-center gap-2">
                  <span className="text-purple-400 font-semibold text-sm">FUTURES POSITIONS</span>
                  <span className="px-2 py-0.5 bg-purple-500/20 text-purple-300 rounded text-xs">
                    {tradingMode?.mode === 'live' ? 'LIVE' : 'PAPER'}
                  </span>
                </div>
              )}
              {futuresPositions.length > 0 ? (
                <FuturesPositionsTable />
              ) : (
                positionsView === 'futures' && (
                  <div className="text-center py-8 text-gray-400">
                    No open futures positions
                  </div>
                )
              )}
            </div>
          )}

          {/* Empty state */}
          {positions.length === 0 && futuresPositions.length === 0 && (
            <div className="text-center py-8 text-gray-400">
              No open positions
            </div>
          )}
        </div>
      </div>

      {/* Trade History */}
      <div className="card">
        <div className="card-header">
          <div className="flex items-center gap-2">
            <History className="w-5 h-5 text-gray-400" />
            <h2 className="text-lg font-semibold">Trade History</h2>
          </div>
        </div>
        <div className="card-body">
          <TradeHistory />
        </div>
      </div>

      {/* Two Column Layout */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Active Orders */}
        <div className="card">
          <div className="card-header">
            <h2 className="text-lg font-semibold">Active Orders</h2>
          </div>
          <div className="card-body p-0">
            <OrdersTable />
          </div>
        </div>

        {/* Strategies */}
        <div className="card">
          <div className="card-header">
            <h2 className="text-lg font-semibold">Active Strategies</h2>
          </div>
          <div className="card-body p-0">
            <StrategiesPanel />
          </div>
        </div>
      </div>

      {/* Market Screener and Signals */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="card">
          <div className="card-header">
            <h2 className="text-lg font-semibold">Market Opportunities</h2>
          </div>
          <div className="card-body p-0">
            <ScreenerResults />
          </div>
        </div>

        <div className="card">
          <div className="card-header">
            <h2 className="text-lg font-semibold">Signal Management</h2>
          </div>
          <div className="card-body">
            <EnhancedSignalsPanel />
          </div>
        </div>

        {/* Strategy Scanner - Signal Proximity */}
        <div className="card">
          <div className="card-header">
            <h2 className="text-lg font-semibold">Strategy Scanner - Signal Proximity</h2>
          </div>
          <div className="card-body p-0">
            <StrategyScanner />
          </div>
        </div>

        {/* AI Signals - Autopilot Decisions */}
        <div className="card">
          <div className="card-header">
            <div className="flex items-center gap-2">
              <Brain className="w-5 h-5 text-purple-400" />
              <h2 className="text-lg font-semibold">AI Signals - Autopilot Decisions</h2>
            </div>
          </div>
          <div className="card-body p-0">
            <AISignalsPanel />
          </div>
        </div>
      </div>

      {/* Modals */}
      <PendingSignalsModal
        isOpen={showPendingSignalsModal}
        onClose={() => setShowPendingSignalsModal(false)}
      />
      <StrategyConfigModal
        isOpen={showStrategyConfigModal}
        onClose={() => setShowStrategyConfigModal(false)}
      />
    </div>
  );
}
