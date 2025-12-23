import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useStore } from '../store';
import { TrendingUp, Activity, DollarSign, Bell, Settings, Sparkles, ArrowRight, Brain, History, Zap, AlertCircle } from 'lucide-react';
import PositionsTable from '../components/PositionsTable';
import OrdersTable from '../components/OrdersTable';
import StrategiesPanel from '../components/StrategiesPanel';
import ScreenerResults from '../components/ScreenerResults';
import EnhancedSignalsPanel from '../components/EnhancedSignalsPanel';
import PendingSignalsModal from '../components/PendingSignalsModal';
import StrategyConfigModal from '../components/StrategyConfigModal';
import StrategyScanner from '../components/StrategyScanner';
import AISignalsPanel from '../components/AISignalsPanel';
import AITradeStatusPanel from '../components/AITradeStatusPanel';
import TradeHistory from '../components/TradeHistory';
import TradingModeToggle from '../components/TradingModeToggle';
import WalletBalanceCard from '../components/WalletBalanceCard';
import PanicButton from '../components/PanicButton';
import SpotAutopilotPanel from '../components/SpotAutopilotPanel';

export default function Dashboard() {
  const { metrics, positions } = useStore();
  const navigate = useNavigate();
  const [showPendingSignalsModal, setShowPendingSignalsModal] = useState(false);
  const [showStrategyConfigModal, setShowStrategyConfigModal] = useState(false);
  const [tradingMode, setTradingMode] = useState<{ mode: 'paper' | 'live'; mode_label: string } | null>(null);
  const [activeTab, setActiveTab] = useState<'ai-trader' | 'signals' | 'scanner'>('ai-trader');

  // Handle trading mode changes from TradingModeToggle component
  const handleTradingModeChange = (mode: { mode: 'paper' | 'live'; mode_label: string }) => {
    setTradingMode(mode);
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
      {/* Header Row - Trading Mode & Actions */}
      <div className="flex items-center justify-between gap-4 flex-wrap">
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

          {/* Spot Trading Badge */}
          <div className="flex items-center gap-2 px-3 py-2 bg-blue-500/20 border border-blue-500/50 rounded-lg">
            <DollarSign className="w-4 h-4 text-blue-400" />
            <span className="text-blue-400 font-semibold text-sm">SPOT TRADING</span>
          </div>
        </div>

        {/* Right side - Action Buttons */}
        <div className="flex gap-3 flex-wrap">
          <button
            onClick={() => setShowStrategyConfigModal(true)}
            className="bg-gray-700 hover:bg-gray-600 text-white px-4 py-2 rounded-lg font-medium transition-colors flex items-center"
          >
            <Settings className="w-4 h-4 mr-2" />
            Strategies
          </button>
          <button
            onClick={() => setShowPendingSignalsModal(true)}
            className="bg-yellow-600 hover:bg-yellow-700 text-white px-4 py-2 rounded-lg font-medium transition-colors flex items-center"
          >
            <Bell className="w-4 h-4 mr-2" />
            Signals
          </button>
          <PanicButton type="spot" onComplete={() => {}} />
        </div>
      </div>

      {/* Wallet Balance */}
      <WalletBalanceCard />

      {/* Main Content - Two Column Layout */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
        {/* Left Column - AI Trader & Controls (2/3 width) */}
        <div className="xl:col-span-2 space-y-6">
          {/* Spot AI Trader - Primary Trading AI */}
          <div className="bg-gradient-to-br from-blue-900/40 to-purple-900/40 rounded-xl border border-blue-500/30 p-6">
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-blue-500/20 rounded-lg">
                  <Brain className="w-6 h-6 text-blue-400" />
                </div>
                <div>
                  <h2 className="text-xl font-bold text-white">Spot AI Trader</h2>
                  <p className="text-sm text-gray-400">Autonomous spot trading with AI signals</p>
                </div>
              </div>
            </div>
            <SpotAutopilotPanel />
          </div>

          {/* Tab Navigation for AI Features */}
          <div className="bg-gray-800 rounded-lg p-1 flex">
            <button
              onClick={() => setActiveTab('ai-trader')}
              className={`flex-1 flex items-center justify-center gap-2 py-3 rounded-md text-sm font-medium transition-colors ${
                activeTab === 'ai-trader' ? 'bg-blue-600 text-white' : 'text-gray-400 hover:text-white'
              }`}
            >
              <Brain className="w-4 h-4" />
              AI Decisions
            </button>
            <button
              onClick={() => setActiveTab('signals')}
              className={`flex-1 flex items-center justify-center gap-2 py-3 rounded-md text-sm font-medium transition-colors ${
                activeTab === 'signals' ? 'bg-blue-600 text-white' : 'text-gray-400 hover:text-white'
              }`}
            >
              <Activity className="w-4 h-4" />
              Signal Management
            </button>
            <button
              onClick={() => setActiveTab('scanner')}
              className={`flex-1 flex items-center justify-center gap-2 py-3 rounded-md text-sm font-medium transition-colors ${
                activeTab === 'scanner' ? 'bg-blue-600 text-white' : 'text-gray-400 hover:text-white'
              }`}
            >
              <Sparkles className="w-4 h-4" />
              Strategy Scanner
            </button>
          </div>

          {/* Tab Content */}
          <div className="min-h-[400px]">
            {activeTab === 'ai-trader' && (
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                <AITradeStatusPanel />
                <div className="card">
                  <div className="card-header">
                    <div className="flex items-center gap-2">
                      <Brain className="w-5 h-5 text-purple-400" />
                      <h2 className="text-lg font-semibold">AI Signals</h2>
                    </div>
                  </div>
                  <div className="card-body p-0">
                    <AISignalsPanel />
                  </div>
                </div>
              </div>
            )}

            {activeTab === 'signals' && (
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                <div className="card">
                  <div className="card-header">
                    <h2 className="text-lg font-semibold">Signal Management</h2>
                  </div>
                  <div className="card-body">
                    <EnhancedSignalsPanel />
                  </div>
                </div>
                <div className="card">
                  <div className="card-header">
                    <h2 className="text-lg font-semibold">Market Opportunities</h2>
                  </div>
                  <div className="card-body p-0">
                    <ScreenerResults />
                  </div>
                </div>
              </div>
            )}

            {activeTab === 'scanner' && (
              <div className="card">
                <div className="card-header">
                  <h2 className="text-lg font-semibold">Strategy Scanner - Signal Proximity</h2>
                </div>
                <div className="card-body p-0">
                  <StrategyScanner />
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Right Column - Stats & Quick Info (1/3 width) */}
        <div className="space-y-6">
          {/* Performance Metrics */}
          <div className="card">
            <div className="card-header">
              <h2 className="text-lg font-semibold">Performance</h2>
            </div>
            <div className="card-body space-y-4">
              <div className="flex items-center justify-between p-3 bg-gray-800 rounded-lg">
                <div className="flex items-center gap-2">
                  <DollarSign className="w-5 h-5 text-gray-400" />
                  <span className="text-sm text-gray-400">Total P&L</span>
                </div>
                <span className={`text-lg font-bold ${(metrics?.total_pnl || 0) >= 0 ? 'text-green-500' : 'text-red-500'}`}>
                  {formatCurrency(metrics?.total_pnl || 0)}
                </span>
              </div>

              <div className="flex items-center justify-between p-3 bg-gray-800 rounded-lg">
                <div className="flex items-center gap-2">
                  <Activity className="w-5 h-5 text-gray-400" />
                  <span className="text-sm text-gray-400">Win Rate</span>
                </div>
                <span className="text-lg font-bold text-blue-400">
                  {(metrics?.win_rate || 0).toFixed(1)}%
                </span>
              </div>

              <div className="flex items-center justify-between p-3 bg-gray-800 rounded-lg">
                <div className="flex items-center gap-2">
                  <TrendingUp className="w-5 h-5 text-gray-400" />
                  <span className="text-sm text-gray-400">Total Trades</span>
                </div>
                <span className="text-lg font-bold text-white">
                  {metrics?.total_trades || 0}
                </span>
              </div>

              <div className="grid grid-cols-2 gap-2 text-sm">
                <div className="p-2 bg-green-500/10 rounded text-center">
                  <div className="text-green-400 font-semibold">{metrics?.winning_trades || 0}</div>
                  <div className="text-gray-500 text-xs">Wins</div>
                </div>
                <div className="p-2 bg-red-500/10 rounded text-center">
                  <div className="text-red-400 font-semibold">{metrics?.losing_trades || 0}</div>
                  <div className="text-gray-500 text-xs">Losses</div>
                </div>
              </div>
            </div>
          </div>

          {/* Open Positions Summary */}
          <div className="card">
            <div className="card-header flex items-center justify-between">
              <h2 className="text-lg font-semibold">Open Positions</h2>
              <span className="px-2 py-1 bg-blue-500/20 text-blue-400 rounded text-sm font-medium">
                {positions.length}
              </span>
            </div>
            <div className="card-body p-0 max-h-[300px] overflow-y-auto">
              <PositionsTable />
            </div>
          </div>

          {/* Active Strategies */}
          <div className="card">
            <div className="card-header">
              <h2 className="text-lg font-semibold">Active Strategies</h2>
            </div>
            <div className="card-body p-0">
              <StrategiesPanel />
            </div>
          </div>

          {/* Visual Strategy Builder Link */}
          <div
            onClick={() => navigate('/visual-strategy-advanced')}
            className="bg-gradient-to-r from-blue-600/80 to-purple-600/80 rounded-lg p-4 cursor-pointer hover:from-blue-600 hover:to-purple-600 transition-all group"
          >
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <Sparkles className="w-6 h-6 text-yellow-300" />
                <div>
                  <h3 className="font-semibold text-white">Visual Strategy Builder</h3>
                  <p className="text-xs text-blue-100">Create custom trading strategies</p>
                </div>
              </div>
              <ArrowRight className="w-5 h-5 text-white group-hover:translate-x-1 transition-transform" />
            </div>
          </div>
        </div>
      </div>

      {/* Bottom Section - Orders & History */}
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
