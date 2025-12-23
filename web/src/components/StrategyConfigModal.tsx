import { useState, useEffect } from 'react';
import { X, Plus, Settings } from 'lucide-react';
import { apiService } from '../services/api';

interface StrategyConfig {
  id?: number;
  name: string;
  symbol: string;
  timeframe: string;
  indicator_type: string;
  autopilot: boolean;
  enabled: boolean;
  position_size: number;
  stop_loss_percent: number;
  take_profit_percent: number;
}

interface Props {
  isOpen: boolean;
  onClose: () => void;
  onSaved?: () => void;
}

const TIMEFRAMES = [
  { value: '1m', label: '1 Minute' },
  { value: '3m', label: '3 Minutes' },
  { value: '5m', label: '5 Minutes' },
  { value: '10m', label: '10 Minutes' },
  { value: '15m', label: '15 Minutes' },
  { value: '30m', label: '30 Minutes' },
  { value: '1h', label: '1 Hour' },
  { value: '4h', label: '4 Hours' },
  { value: '1d', label: '1 Day' },
];

const INDICATORS = [
  { value: 'swing_trading', label: 'Swing Trading (Advanced)' },
  { value: 'ema_crossover', label: 'EMA Crossover' },
  { value: 'rsi', label: 'RSI Oversold/Overbought' },
  { value: 'macd', label: 'MACD Crossover' },
  { value: 'bollinger_bands', label: 'Bollinger Bands' },
  { value: 'stochastic', label: 'Stochastic Oscillator' },
  { value: 'volume_spike', label: 'Volume Spike' },
  { value: 'breakout', label: 'Breakout Strategy' },
  { value: 'support_test', label: 'Support Test' },
  { value: 'pivot_breakout', label: 'Pivot Point Breakout' },
];

export default function StrategyConfigModal({ isOpen, onClose, onSaved }: Props) {
  const [configs, setConfigs] = useState<StrategyConfig[]>([]);
  const [symbols, setSymbols] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadingSymbols, setLoadingSymbols] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [formData, setFormData] = useState<StrategyConfig>({
    name: '',
    symbol: 'BTCUSDT',
    timeframe: '15m',
    indicator_type: 'swing_trading',
    autopilot: false,
    enabled: true,
    position_size: 0.10,
    stop_loss_percent: 2.0,
    take_profit_percent: 5.0,
  });

  useEffect(() => {
    if (isOpen) {
      fetchConfigs();
      fetchSymbols();
    }
  }, [isOpen]);

  const fetchConfigs = async () => {
    try {
      const data = await apiService.getStrategyConfigs();
      setConfigs(data);
    } catch (err) {
      console.error('Failed to fetch configs:', err);
      setError('Failed to load strategy configurations');
    }
  };

  const fetchSymbols = async () => {
    setLoadingSymbols(true);
    try {
      const data = await apiService.getBinanceSymbols();
      setSymbols(data);
    } catch (err) {
      console.error('Failed to fetch symbols:', err);
      // Fallback to popular symbols if API fails
      setSymbols(['BTCUSDT', 'ETHUSDT', 'BNBUSDT', 'ADAUSDT', 'SOLUSDT', 'XRPUSDT', 'DOTUSDT', 'DOGEUSDT', 'MATICUSDT', 'LTCUSDT']);
    } finally {
      setLoadingSymbols(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      if (formData.id) {
        await apiService.updateStrategyConfig(formData.id, formData);
      } else {
        await apiService.createStrategyConfig(formData);
      }
      await fetchConfigs();
      setShowForm(false);
      setFormData({
        name: '',
        symbol: 'BTCUSDT',
        timeframe: '15m',
        indicator_type: 'swing_trading',
        autopilot: false,
        enabled: true,
        position_size: 0.10,
        stop_loss_percent: 2.0,
        take_profit_percent: 5.0,
      });
      if (onSaved) onSaved();
    } catch (err: any) {
      setError(err.response?.data?.message || 'Failed to save strategy');
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm('Are you sure you want to delete this strategy?')) return;

    try {
      await apiService.deleteStrategyConfig(id);
      await fetchConfigs();
      if (onSaved) onSaved();
    } catch (err) {
      setError('Failed to delete strategy');
    }
  };

  const toggleAutopilot = async (config: StrategyConfig) => {
    try {
      console.log('[StrategyConfigModal] Toggling autopilot for:', config.name, 'from', config.autopilot, 'to', !config.autopilot);
      const updatePayload = {
        symbol: config.symbol,
        timeframe: config.timeframe,
        indicator_type: config.indicator_type,
        autopilot: !config.autopilot,
        enabled: config.enabled,
        position_size: config.position_size,
        stop_loss_percent: config.stop_loss_percent,
        take_profit_percent: config.take_profit_percent,
      };
      console.log('[StrategyConfigModal] Update payload:', updatePayload);
      const result = await apiService.updateStrategyConfig(config.id!, updatePayload);
      console.log('[StrategyConfigModal] Update result:', result);
      await fetchConfigs();
    } catch (err: any) {
      console.error('[StrategyConfigModal] Toggle autopilot error:', err);
      setError('Failed to update autopilot setting: ' + (err.message || err.response?.data?.message || 'Unknown error'));
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      <div className="flex items-center justify-center min-h-screen px-4 pt-4 pb-20 text-center sm:p-0">
        <div className="fixed inset-0 transition-opacity bg-black bg-opacity-75" onClick={onClose} />

        <div className="relative inline-block w-full max-w-5xl p-6 my-8 overflow-hidden text-left align-middle transition-all transform bg-dark-800 shadow-xl rounded-xl max-h-[90vh] overflow-y-auto">
          {/* Header */}
          <div className="flex items-center justify-between mb-6">
            <h3 className="text-2xl font-bold text-white flex items-center">
              <Settings className="w-6 h-6 mr-3" />
              Strategy Configuration
            </h3>
            <button onClick={onClose} className="text-gray-400 hover:text-white transition-colors">
              <X className="w-6 h-6" />
            </button>
          </div>

          {error && (
            <div className="mb-4 p-3 bg-red-500/10 border border-red-500 rounded-lg text-red-500 text-sm">
              {error}
            </div>
          )}

          {!showForm ? (
            <>
              {/* Add New Button */}
              <button
                onClick={() => setShowForm(true)}
                className="w-full mb-4 bg-primary-600 hover:bg-primary-700 text-white px-4 py-3 rounded-lg font-semibold transition-colors flex items-center justify-center"
              >
                <Plus className="w-5 h-5 mr-2" />
                Add New Strategy
              </button>

              {/* Configs List */}
              <div className="space-y-3">
                {configs.length === 0 ? (
                  <div className="text-center py-12 text-gray-400">
                    <Settings className="w-16 h-16 mx-auto mb-4 opacity-50" />
                    <div className="text-lg mb-2">No strategies configured</div>
                    <div className="text-sm">Click "Add New Strategy" to create your first strategy</div>
                  </div>
                ) : (
                  configs.map((config) => (
                    <div
                      key={config.id}
                      className="bg-dark-700 rounded-lg p-4 border border-dark-600 hover:border-primary-500 transition-colors"
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex-1">
                          <div className="flex items-center gap-3 mb-2">
                            <span className="text-lg font-semibold text-white">{config.name}</span>
                            <span className="px-2 py-1 bg-primary-500/20 text-primary-400 text-xs rounded">
                              {config.symbol}
                            </span>
                            <span className="px-2 py-1 bg-gray-700 text-gray-300 text-xs rounded">
                              {config.timeframe}
                            </span>
                            {config.autopilot && (
                              <span className="px-2 py-1 bg-green-500/20 text-green-400 text-xs rounded animate-pulse">
                                AUTOPILOT
                              </span>
                            )}
                            {!config.enabled && (
                              <span className="px-2 py-1 bg-red-500/20 text-red-400 text-xs rounded">
                                DISABLED
                              </span>
                            )}
                          </div>
                          <div className="text-sm text-gray-400">
                            {INDICATORS.find((i) => i.value === config.indicator_type)?.label || config.indicator_type}
                            {' • '}
                            Position: {(config.position_size * 100).toFixed(1)}%
                            {' • '}
                            SL: {config.stop_loss_percent}%
                            {' • '}
                            TP: {config.take_profit_percent}%
                          </div>
                        </div>
                        <div className="flex gap-2">
                          <button
                            onClick={() => toggleAutopilot(config)}
                            className={`px-3 py-1 rounded text-sm font-semibold transition-colors ${
                              config.autopilot
                                ? 'bg-yellow-600 hover:bg-yellow-700 text-white'
                                : 'bg-gray-700 hover:bg-gray-600 text-gray-300'
                            }`}
                          >
                            {config.autopilot ? 'Auto' : 'Manual'}
                          </button>
                          <button
                            onClick={() => handleDelete(config.id!)}
                            className="px-3 py-1 bg-red-600 hover:bg-red-700 text-white rounded text-sm font-semibold transition-colors"
                          >
                            Delete
                          </button>
                        </div>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </>
          ) : (
            // Strategy Form
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-semibold text-gray-300 mb-2">Strategy Name</label>
                  <input
                    type="text"
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    required
                    className="w-full bg-dark-700 border border-dark-600 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-primary-500"
                    placeholder="My Strategy"
                  />
                </div>

                <div>
                  <label className="block text-sm font-semibold text-gray-300 mb-2">
                    Symbol {loadingSymbols && <span className="text-xs text-gray-500">(loading...)</span>}
                  </label>
                  <select
                    value={formData.symbol}
                    onChange={(e) => setFormData({ ...formData, symbol: e.target.value })}
                    disabled={loadingSymbols}
                    className="w-full bg-dark-700 border border-dark-600 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-primary-500 disabled:opacity-50"
                  >
                    {symbols.map((symbol) => (
                      <option key={symbol} value={symbol}>{symbol}</option>
                    ))}
                  </select>
                  <div className="text-xs text-gray-500 mt-1">
                    {symbols.length} symbols available
                  </div>
                </div>

                <div>
                  <label className="block text-sm font-semibold text-gray-300 mb-2">Timeframe</label>
                  <select
                    value={formData.timeframe}
                    onChange={(e) => setFormData({ ...formData, timeframe: e.target.value })}
                    className="w-full bg-dark-700 border border-dark-600 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-primary-500"
                  >
                    {TIMEFRAMES.map((tf) => (
                      <option key={tf.value} value={tf.value}>{tf.label}</option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block text-sm font-semibold text-gray-300 mb-2">Indicator/Strategy Type</label>
                  <select
                    value={formData.indicator_type}
                    onChange={(e) => setFormData({ ...formData, indicator_type: e.target.value })}
                    className="w-full bg-dark-700 border border-dark-600 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-primary-500"
                  >
                    {INDICATORS.map((ind) => (
                      <option key={ind.value} value={ind.value}>{ind.label}</option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block text-sm font-semibold text-gray-300 mb-2">Position Size (%)</label>
                  <input
                    type="number"
                    step="0.01"
                    min="0.01"
                    max="1"
                    value={formData.position_size}
                    onChange={(e) => setFormData({ ...formData, position_size: parseFloat(e.target.value) })}
                    required
                    className="w-full bg-dark-700 border border-dark-600 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-primary-500"
                  />
                  <div className="text-xs text-gray-500 mt-1">
                    {(formData.position_size * 100).toFixed(1)}% of balance per trade
                  </div>
                </div>

                <div>
                  <label className="block text-sm font-semibold text-gray-300 mb-2">Stop Loss (%)</label>
                  <input
                    type="number"
                    step="0.1"
                    min="0.1"
                    value={formData.stop_loss_percent}
                    onChange={(e) => setFormData({ ...formData, stop_loss_percent: parseFloat(e.target.value) })}
                    required
                    className="w-full bg-dark-700 border border-dark-600 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-primary-500"
                  />
                </div>

                <div>
                  <label className="block text-sm font-semibold text-gray-300 mb-2">Take Profit (%)</label>
                  <input
                    type="number"
                    step="0.1"
                    min="0.1"
                    value={formData.take_profit_percent}
                    onChange={(e) => setFormData({ ...formData, take_profit_percent: parseFloat(e.target.value) })}
                    required
                    className="w-full bg-dark-700 border border-dark-600 rounded-lg px-3 py-2 text-white focus:outline-none focus:border-primary-500"
                  />
                </div>
              </div>

              <div className="flex items-center space-x-6 p-4 bg-dark-700 rounded-lg">
                <label className="flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    checked={formData.autopilot}
                    onChange={(e) => setFormData({ ...formData, autopilot: e.target.checked })}
                    className="w-5 h-5 text-primary-600 bg-dark-600 border-dark-500 rounded focus:ring-primary-500 focus:ring-2"
                  />
                  <span className="ml-3 text-sm font-semibold text-gray-300">
                    Autopilot Mode
                    {formData.autopilot && <span className="ml-2 text-yellow-500 animate-pulse">⚡</span>}
                  </span>
                </label>
                <div className="text-xs text-gray-500">
                  {formData.autopilot
                    ? 'Trades will be executed automatically without confirmation'
                    : 'You will be asked to confirm each trade before execution'}
                </div>
              </div>

              <div className="flex gap-3 mt-6">
                <button
                  type="button"
                  onClick={() => setShowForm(false)}
                  className="flex-1 bg-gray-700 hover:bg-gray-600 text-white px-4 py-2 rounded-lg font-semibold transition-colors"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={loading}
                  className="flex-1 bg-primary-600 hover:bg-primary-700 text-white px-4 py-2 rounded-lg font-semibold transition-colors disabled:opacity-50"
                >
                  {loading ? 'Saving...' : 'Save Strategy'}
                </button>
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  );
}
