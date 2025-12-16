import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useStore } from '../store';
import { TrendingUp, TrendingDown, Activity, DollarSign, Bell, Settings, Sparkles, ArrowRight } from 'lucide-react';
import PositionsTable from '../components/PositionsTable';
import OrdersTable from '../components/OrdersTable';
import StrategiesPanel from '../components/StrategiesPanel';
import ScreenerResults from '../components/ScreenerResults';
import EnhancedSignalsPanel from '../components/EnhancedSignalsPanel';
import PendingSignalsModal from '../components/PendingSignalsModal';
import StrategyConfigModal from '../components/StrategyConfigModal';
import StrategyScanner from '../components/StrategyScanner';

export default function Dashboard() {
  const { metrics, positions, activeOrders } = useStore();
  const navigate = useNavigate();
  const [showPendingSignalsModal, setShowPendingSignalsModal] = useState(false);
  const [showStrategyConfigModal, setShowStrategyConfigModal] = useState(false);

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

      {/* Action Buttons */}
      <div className="flex justify-end gap-3">
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

      {/* Positions */}
      <div className="card">
        <div className="card-header">
          <h2 className="text-lg font-semibold">Open Positions</h2>
        </div>
        <div className="card-body p-0">
          <PositionsTable />
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
