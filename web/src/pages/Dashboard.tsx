import { useState } from 'react';
import { useStore } from '../store';
import { TrendingUp, TrendingDown, Activity, DollarSign, Bell, Settings } from 'lucide-react';
import PositionsTable from '../components/PositionsTable';
import OrdersTable from '../components/OrdersTable';
import StrategiesPanel from '../components/StrategiesPanel';
import ScreenerResults from '../components/ScreenerResults';
import SignalsPanel from '../components/SignalsPanel';
import PendingSignalsModal from '../components/PendingSignalsModal';
import StrategyConfigModal from '../components/StrategyConfigModal';

export default function Dashboard() {
  const { metrics, positions, activeOrders } = useStore();
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
            <h2 className="text-lg font-semibold">Recent Signals</h2>
          </div>
          <div className="card-body p-0">
            <SignalsPanel />
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
