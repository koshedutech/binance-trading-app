import { useState, useEffect, useCallback } from 'react';
import { RefreshCw, TrendingUp, TrendingDown, ChevronDown, ChevronUp, Shield, Layers, ArrowLeftRight, DollarSign, AlertTriangle, Activity, Target, Settings, Save } from 'lucide-react';
import { futuresApi, HedgeModePositionData, HedgeModeConfig } from '../services/futuresApi';

interface HedgeModeMonitorProps {
  autoRefresh?: boolean;
  refreshInterval?: number;
}

const HedgeModeMonitor = ({ autoRefresh = true, refreshInterval = 5000 }: HedgeModeMonitorProps) => {
  const [positions, setPositions] = useState<HedgeModePositionData[]>([]);
  const [config, setConfig] = useState<HedgeModeConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expandedPosition, setExpandedPosition] = useState<string | null>(null);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const [count, setCount] = useState(0);
  const [toggling, setToggling] = useState(false);
  const [showConfig, setShowConfig] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editConfig, setEditConfig] = useState<Partial<HedgeModeConfig>>({});

  const fetchData = useCallback(async () => {
    try {
      const [posData, configData] = await Promise.all([
        futuresApi.getHedgeModePositions(),
        futuresApi.getHedgeModeConfig()
      ]);
      setPositions(posData.positions || []);
      setCount(posData.count);
      setConfig(configData);
      setLastUpdate(new Date());
      setError(null);
    } catch (err: any) {
      setError(err?.response?.data?.error || 'Failed to fetch hedge mode data');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
    if (autoRefresh) {
      const interval = setInterval(fetchData, refreshInterval);
      return () => clearInterval(interval);
    }
  }, [fetchData, autoRefresh, refreshInterval]);

  const handleToggleHedgeMode = async () => {
    if (!config || toggling) return;
    setToggling(true);
    try {
      await futuresApi.toggleHedgeMode(!config.hedge_mode_enabled);
      // Refresh to get updated config
      await fetchData();
    } catch (err: any) {
      setError(err?.response?.data?.error || 'Failed to toggle hedge mode');
    } finally {
      setToggling(false);
    }
  };

  const updateEditConfig = (key: keyof HedgeModeConfig, value: any) => {
    setEditConfig(prev => ({ ...prev, [key]: value }));
  };

  const handleSaveConfig = async () => {
    if (saving || Object.keys(editConfig).length === 0) return;
    setSaving(true);
    try {
      await futuresApi.updateHedgeModeConfig(editConfig);
      setEditConfig({});
      await fetchData();
    } catch (err: any) {
      setError(err?.response?.data?.error || 'Failed to save config');
    } finally {
      setSaving(false);
    }
  };

  const getConfigValue = (key: keyof HedgeModeConfig) => {
    return editConfig[key] !== undefined ? editConfig[key] : config?.[key];
  };

  const formatPrice = (price: number) => {
    if (price === 0) return '-';
    if (price < 0.01) return price.toFixed(8);
    if (price < 1) return price.toFixed(6);
    if (price < 100) return price.toFixed(4);
    return price.toFixed(2);
  };

  const formatPnL = (pnl: number) => {
    const sign = pnl >= 0 ? '+' : '';
    return `${sign}$${pnl.toFixed(2)}`;
  };

  const formatPercent = (pct: number) => {
    const sign = pct >= 0 ? '+' : '';
    return `${sign}${pct.toFixed(2)}%`;
  };

  const getTriggerTypeColor = (type: string) => {
    switch (type) {
      case 'profit':
        return 'text-green-400 bg-green-900/30';
      case 'loss':
        return 'text-red-400 bg-red-900/30';
      default:
        return 'text-gray-400 bg-gray-700/30';
    }
  };

  const renderProgressBar = (current: number, target: number, color: string) => {
    const percentage = Math.min((current / target) * 100, 100);
    return (
      <div className="w-full bg-gray-700 rounded-full h-1.5">
        <div
          className={`h-1.5 rounded-full ${color}`}
          style={{ width: `${percentage}%` }}
        />
      </div>
    );
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center p-4">
        <RefreshCw className="w-4 h-4 text-gray-400 animate-spin" />
        <span className="ml-2 text-xs text-gray-400">Loading hedge mode positions...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-2 bg-red-900/20 border border-red-700 rounded">
        <span className="text-xs text-red-400">{error}</span>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {/* Header with Config Status */}
      <div className="flex items-center justify-between p-2 bg-gray-800/50 rounded border border-gray-700">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-1.5">
            <Layers className="w-4 h-4 text-purple-400" />
            <span className="text-sm font-medium text-gray-200">Hedge Mode</span>
          </div>
          <div className="text-xs text-gray-400">
            <span className="font-medium text-gray-300">{count}</span> active hedges
          </div>
          {config && (
            <>
              <div className="text-xs text-gray-400">
                Exit @ <span className="font-medium text-green-400">{config.combined_roi_exit_pct}%</span> ROI
              </div>
              <div className="text-xs text-gray-400">
                Max <span className="font-medium text-yellow-400">{config.max_position_multiple}x</span> position
              </div>
            </>
          )}
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={handleToggleHedgeMode}
            disabled={toggling}
            className={`text-[9px] px-2 py-1 rounded transition-colors cursor-pointer ${
              config?.hedge_mode_enabled
                ? 'bg-green-900/50 text-green-400 border border-green-700 hover:bg-green-900/70'
                : 'bg-gray-700/50 text-gray-400 border border-gray-600 hover:bg-gray-600/50'
            } ${toggling ? 'opacity-50 cursor-wait' : ''}`}
            title={config?.hedge_mode_enabled ? 'Click to disable hedge mode' : 'Click to enable hedge mode'}
          >
            {toggling ? '...' : config?.hedge_mode_enabled ? 'ENABLED' : 'DISABLED'}
          </button>
          {lastUpdate && (
            <span className="text-[9px] text-gray-500">
              Updated {lastUpdate.toLocaleTimeString()}
            </span>
          )}
          <button
            onClick={fetchData}
            className="p-1 hover:bg-gray-700 rounded transition-colors"
            title="Refresh"
          >
            <RefreshCw className="w-3 h-3 text-gray-400" />
          </button>
          <button
            onClick={() => setShowConfig(!showConfig)}
            className={`p-1 rounded transition-colors ${showConfig ? 'bg-orange-900/50 text-orange-400' : 'hover:bg-gray-700 text-gray-400'}`}
            title="Configure hedge mode settings"
          >
            <Settings className="w-3 h-3" />
          </button>
        </div>
      </div>

      {/* Configuration Panel */}
      {showConfig && config && (
        <div className="p-3 bg-gray-800/50 rounded border border-orange-700/50 space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-orange-400">Hedge Mode Configuration</span>
            <button
              onClick={handleSaveConfig}
              disabled={saving || Object.keys(editConfig).length === 0}
              className={`flex items-center gap-1 px-2 py-1 rounded text-[10px] transition-colors ${
                Object.keys(editConfig).length > 0
                  ? 'bg-green-900/50 text-green-400 border border-green-700 hover:bg-green-900/70'
                  : 'bg-gray-700/50 text-gray-500 border border-gray-600 cursor-not-allowed'
              }`}
            >
              <Save className="w-3 h-3" />
              {saving ? 'Saving...' : 'Save Changes'}
            </button>
          </div>

          {/* Trigger Settings */}
          <div className="border border-gray-700 rounded p-2">
            <div className="text-[10px] font-medium text-gray-400 mb-2">Trigger Settings</div>
            <div className="grid grid-cols-3 gap-2">
              <label className="flex items-center gap-1.5 text-[10px] text-gray-300">
                <input
                  type="checkbox"
                  checked={getConfigValue('trigger_on_profit_tp') as boolean}
                  onChange={(e) => updateEditConfig('trigger_on_profit_tp', e.target.checked)}
                  className="w-3 h-3"
                />
                Trigger on Profit TP
              </label>
              <label className="flex items-center gap-1.5 text-[10px] text-gray-300">
                <input
                  type="checkbox"
                  checked={getConfigValue('trigger_on_loss_tp') as boolean}
                  onChange={(e) => updateEditConfig('trigger_on_loss_tp', e.target.checked)}
                  className="w-3 h-3"
                />
                Trigger on Loss TP
              </label>
              <label className="flex items-center gap-1.5 text-[10px] text-gray-300">
                <input
                  type="checkbox"
                  checked={getConfigValue('dca_on_loss') as boolean}
                  onChange={(e) => updateEditConfig('dca_on_loss', e.target.checked)}
                  className="w-3 h-3"
                />
                DCA on Loss
              </label>
            </div>
          </div>

          {/* Position & Exit Settings */}
          <div className="border border-gray-700 rounded p-2">
            <div className="text-[10px] font-medium text-gray-400 mb-2">Position & Exit</div>
            <div className="grid grid-cols-3 gap-3">
              <div>
                <label className="block text-[9px] text-gray-500 mb-1">Max Position Multiple</label>
                <input
                  type="number"
                  min="1.5"
                  max="5"
                  step="0.5"
                  value={getConfigValue('max_position_multiple') as number}
                  onChange={(e) => updateEditConfig('max_position_multiple', parseFloat(e.target.value))}
                  className="w-full px-1.5 py-1 bg-gray-700 border border-gray-600 rounded text-white text-[10px]"
                />
              </div>
              <div>
                <label className="block text-[9px] text-gray-500 mb-1">Combined ROI Exit %</label>
                <input
                  type="number"
                  min="0.5"
                  max="10"
                  step="0.5"
                  value={getConfigValue('combined_roi_exit_pct') as number}
                  onChange={(e) => updateEditConfig('combined_roi_exit_pct', parseFloat(e.target.value))}
                  className="w-full px-1.5 py-1 bg-gray-700 border border-gray-600 rounded text-white text-[10px]"
                />
              </div>
              <div>
                <label className="block text-[9px] text-gray-500 mb-1">Wide SL ATR Multiplier</label>
                <input
                  type="number"
                  min="1"
                  max="5"
                  step="0.5"
                  value={getConfigValue('wide_sl_atr_multiplier') as number}
                  onChange={(e) => updateEditConfig('wide_sl_atr_multiplier', parseFloat(e.target.value))}
                  className="w-full px-1.5 py-1 bg-gray-700 border border-gray-600 rounded text-white text-[10px]"
                />
              </div>
            </div>
          </div>

          {/* Rally Exit Settings */}
          <div className="border border-gray-700 rounded p-2">
            <div className="flex items-center justify-between mb-2">
              <span className="text-[10px] font-medium text-gray-400">Rally Exit Settings</span>
              <label className="flex items-center gap-1.5 text-[10px] text-gray-300">
                <input
                  type="checkbox"
                  checked={getConfigValue('rally_exit_enabled') as boolean}
                  onChange={(e) => updateEditConfig('rally_exit_enabled', e.target.checked)}
                  className="w-3 h-3"
                />
                Enabled
              </label>
            </div>
            <div className="grid grid-cols-3 gap-3">
              <div>
                <label className="block text-[9px] text-gray-500 mb-1">ADX Threshold</label>
                <input
                  type="number"
                  min="15"
                  max="40"
                  step="1"
                  value={getConfigValue('rally_adx_threshold') as number}
                  onChange={(e) => updateEditConfig('rally_adx_threshold', parseInt(e.target.value))}
                  className="w-full px-1.5 py-1 bg-gray-700 border border-gray-600 rounded text-white text-[10px]"
                />
              </div>
              <div>
                <label className="block text-[9px] text-gray-500 mb-1">Sustained Move %</label>
                <input
                  type="number"
                  min="1"
                  max="10"
                  step="0.5"
                  value={getConfigValue('rally_sustained_move_pct') as number}
                  onChange={(e) => updateEditConfig('rally_sustained_move_pct', parseFloat(e.target.value))}
                  className="w-full px-1.5 py-1 bg-gray-700 border border-gray-600 rounded text-white text-[10px]"
                />
              </div>
              <label className="flex items-center gap-1.5 text-[10px] text-gray-300 pt-4">
                <input
                  type="checkbox"
                  checked={getConfigValue('disable_ai_sl') as boolean}
                  onChange={(e) => updateEditConfig('disable_ai_sl', e.target.checked)}
                  className="w-3 h-3"
                />
                Disable AI SL
              </label>
            </div>
          </div>

          {/* Negative TP (DCA) Settings */}
          <div className="border border-gray-700 rounded p-2">
            <div className="text-[10px] font-medium text-gray-400 mb-2">Negative TP Levels (DCA Triggers)</div>
            <div className="grid grid-cols-3 gap-3">
              {/* TP1 */}
              <div className="space-y-1">
                <div className="text-[9px] text-red-400 font-medium">Neg TP1</div>
                <div className="flex gap-1">
                  <div className="flex-1">
                    <label className="block text-[8px] text-gray-500">Trigger %</label>
                    <input
                      type="number"
                      min="0.1"
                      max="2"
                      step="0.1"
                      value={getConfigValue('neg_tp1_percent') as number}
                      onChange={(e) => updateEditConfig('neg_tp1_percent', parseFloat(e.target.value))}
                      className="w-full px-1 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-[10px]"
                    />
                  </div>
                  <div className="flex-1">
                    <label className="block text-[8px] text-gray-500">Add %</label>
                    <input
                      type="number"
                      min="10"
                      max="100"
                      step="5"
                      value={getConfigValue('neg_tp1_add_percent') as number}
                      onChange={(e) => updateEditConfig('neg_tp1_add_percent', parseInt(e.target.value))}
                      className="w-full px-1 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-[10px]"
                    />
                  </div>
                </div>
              </div>
              {/* TP2 */}
              <div className="space-y-1">
                <div className="text-[9px] text-red-400 font-medium">Neg TP2</div>
                <div className="flex gap-1">
                  <div className="flex-1">
                    <label className="block text-[8px] text-gray-500">Trigger %</label>
                    <input
                      type="number"
                      min="0.2"
                      max="3"
                      step="0.1"
                      value={getConfigValue('neg_tp2_percent') as number}
                      onChange={(e) => updateEditConfig('neg_tp2_percent', parseFloat(e.target.value))}
                      className="w-full px-1 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-[10px]"
                    />
                  </div>
                  <div className="flex-1">
                    <label className="block text-[8px] text-gray-500">Add %</label>
                    <input
                      type="number"
                      min="10"
                      max="100"
                      step="5"
                      value={getConfigValue('neg_tp2_add_percent') as number}
                      onChange={(e) => updateEditConfig('neg_tp2_add_percent', parseInt(e.target.value))}
                      className="w-full px-1 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-[10px]"
                    />
                  </div>
                </div>
              </div>
              {/* TP3 */}
              <div className="space-y-1">
                <div className="text-[9px] text-red-400 font-medium">Neg TP3</div>
                <div className="flex gap-1">
                  <div className="flex-1">
                    <label className="block text-[8px] text-gray-500">Trigger %</label>
                    <input
                      type="number"
                      min="0.3"
                      max="5"
                      step="0.1"
                      value={getConfigValue('neg_tp3_percent') as number}
                      onChange={(e) => updateEditConfig('neg_tp3_percent', parseFloat(e.target.value))}
                      className="w-full px-1 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-[10px]"
                    />
                  </div>
                  <div className="flex-1">
                    <label className="block text-[8px] text-gray-500">Add %</label>
                    <input
                      type="number"
                      min="10"
                      max="100"
                      step="5"
                      value={getConfigValue('neg_tp3_add_percent') as number}
                      onChange={(e) => updateEditConfig('neg_tp3_add_percent', parseInt(e.target.value))}
                      className="w-full px-1 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-[10px]"
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Help Text */}
          <div className="text-[9px] text-gray-500 italic">
            Hedge mode opens opposite positions on TP hits. DCA adds to losing side at negative TP levels. Exit when combined ROI reaches target or rally detected.
          </div>
        </div>
      )}

      {/* Positions List */}
      {positions.length === 0 ? (
        <div className="p-4 text-center text-xs text-gray-500">
          No positions with active hedge mode
        </div>
      ) : (
        <div className="space-y-2">
          {positions.map((pos) => (
            <div
              key={pos.symbol}
              className="bg-gray-800/50 rounded border border-gray-700 overflow-hidden"
            >
              {/* Position Header */}
              <div
                className="p-2 cursor-pointer hover:bg-gray-700/30 transition-colors"
                onClick={() => setExpandedPosition(expandedPosition === pos.symbol ? null : pos.symbol)}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    {pos.original_side === 'LONG' ? (
                      <TrendingUp className="w-4 h-4 text-green-400" />
                    ) : (
                      <TrendingDown className="w-4 h-4 text-red-400" />
                    )}
                    <span className="font-medium text-sm text-gray-200">{pos.symbol}</span>
                    <span className={`text-[9px] px-1.5 py-0.5 rounded ${pos.original_side === 'LONG' ? 'bg-green-900/50 text-green-400' : 'bg-red-900/50 text-red-400'}`}>
                      {pos.original_side}
                    </span>
                    {pos.hedge.active && (
                      <span className="text-[9px] px-1.5 py-0.5 rounded bg-purple-900/50 text-purple-400 border border-purple-700 flex items-center gap-1">
                        <ArrowLeftRight className="w-3 h-3" /> HEDGED
                      </span>
                    )}
                    {pos.hedge.trigger_type && (
                      <span className={`text-[9px] px-1.5 py-0.5 rounded ${getTriggerTypeColor(pos.hedge.trigger_type)}`}>
                        {pos.hedge.trigger_type.toUpperCase()} trigger
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-3">
                    <div className="text-right">
                      <div className={`text-sm font-bold ${pos.combined.roi_percent >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                        {formatPercent(pos.combined.roi_percent)} ROI
                      </div>
                      <div className="text-[9px] text-gray-400">
                        Combined: <span className={pos.combined.total_pnl >= 0 ? 'text-green-400' : 'text-red-400'}>{formatPnL(pos.combined.total_pnl)}</span>
                      </div>
                    </div>
                    {expandedPosition === pos.symbol ? (
                      <ChevronUp className="w-4 h-4 text-gray-400" />
                    ) : (
                      <ChevronDown className="w-4 h-4 text-gray-400" />
                    )}
                  </div>
                </div>

                {/* ROI Progress Bar */}
                {config && pos.hedge.active && (
                  <div className="mt-2">
                    <div className="flex items-center justify-between text-[9px] text-gray-500 mb-1">
                      <span>Combined ROI Progress</span>
                      <span>{formatPercent(pos.combined.roi_percent)} / {config.combined_roi_exit_pct}%</span>
                    </div>
                    {renderProgressBar(
                      Math.max(0, pos.combined.roi_percent),
                      config.combined_roi_exit_pct,
                      pos.combined.roi_percent >= config.combined_roi_exit_pct ? 'bg-green-500' : 'bg-blue-500'
                    )}
                  </div>
                )}
              </div>

              {/* Expanded Details */}
              {expandedPosition === pos.symbol && (
                <div className="px-2 pb-2 border-t border-gray-700 space-y-3">
                  {/* Two-Column Layout: Original vs Hedge */}
                  <div className="grid grid-cols-2 gap-3 mt-2">
                    {/* Original Position */}
                    <div className="p-2 bg-gray-700/30 rounded border border-gray-600">
                      <div className="flex items-center gap-1.5 mb-2">
                        <Target className="w-3.5 h-3.5 text-blue-400" />
                        <span className="text-[10px] font-medium text-blue-400">ORIGINAL ({pos.original_side})</span>
                      </div>
                      <div className="space-y-1 text-[10px]">
                        <div className="flex justify-between">
                          <span className="text-gray-500">Entry</span>
                          <span className="text-gray-300">{formatPrice(pos.entry_price)}</span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-gray-500">Avg (BE)</span>
                          <span className="text-gray-300">{formatPrice(pos.original.current_be)}</span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-gray-500">Qty</span>
                          <span className="text-gray-300">{pos.original.remaining_qty.toFixed(4)}</span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-gray-500">TP Level</span>
                          <span className="text-gray-300">TP{pos.original.tp_level}</span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-gray-500">Realized</span>
                          <span className={pos.original.accum_profit >= 0 ? 'text-green-400' : 'text-red-400'}>
                            {formatPnL(pos.original.accum_profit)}
                          </span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-gray-500">Unrealized</span>
                          <span className={pos.original.unrealized_pnl >= 0 ? 'text-green-400' : 'text-red-400'}>
                            {formatPnL(pos.original.unrealized_pnl)}
                          </span>
                        </div>
                      </div>
                    </div>

                    {/* Hedge Position */}
                    <div className={`p-2 rounded border ${pos.hedge.active ? 'bg-purple-900/20 border-purple-700' : 'bg-gray-700/30 border-gray-600'}`}>
                      <div className="flex items-center gap-1.5 mb-2">
                        <ArrowLeftRight className="w-3.5 h-3.5 text-purple-400" />
                        <span className="text-[10px] font-medium text-purple-400">
                          HEDGE ({pos.hedge.side || 'N/A'})
                        </span>
                        {!pos.hedge.active && (
                          <span className="text-[8px] px-1 py-0.5 bg-gray-700 rounded text-gray-500">INACTIVE</span>
                        )}
                      </div>
                      {pos.hedge.active ? (
                        <div className="space-y-1 text-[10px]">
                          <div className="flex justify-between">
                            <span className="text-gray-500">Entry</span>
                            <span className="text-gray-300">{formatPrice(pos.hedge.entry_price)}</span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-gray-500">Avg (BE)</span>
                            <span className="text-gray-300">{formatPrice(pos.hedge.current_be)}</span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-gray-500">Qty</span>
                            <span className="text-gray-300">{pos.hedge.remaining_qty.toFixed(4)}</span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-gray-500">TP Level</span>
                            <span className="text-gray-300">TP{pos.hedge.tp_level}</span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-gray-500">Realized</span>
                            <span className={pos.hedge.accum_profit >= 0 ? 'text-green-400' : 'text-red-400'}>
                              {formatPnL(pos.hedge.accum_profit)}
                            </span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-gray-500">Unrealized</span>
                            <span className={pos.hedge.unrealized_pnl >= 0 ? 'text-green-400' : 'text-red-400'}>
                              {formatPnL(pos.hedge.unrealized_pnl)}
                            </span>
                          </div>
                        </div>
                      ) : (
                        <div className="text-[10px] text-gray-500 italic">
                          Hedge not yet triggered
                        </div>
                      )}
                    </div>
                  </div>

                  {/* DCA Info */}
                  {pos.dca.enabled && (
                    <div className="p-2 bg-yellow-900/20 border border-yellow-700/50 rounded">
                      <div className="flex items-center gap-1.5 mb-1">
                        <Layers className="w-3.5 h-3.5 text-yellow-400" />
                        <span className="text-[10px] font-medium text-yellow-400">DCA (Dollar Cost Averaging)</span>
                      </div>
                      <div className="grid grid-cols-3 gap-2 text-[10px]">
                        <div>
                          <span className="text-gray-500">Additions: </span>
                          <span className="text-gray-300">{pos.dca.additions_count}</span>
                        </div>
                        <div>
                          <span className="text-gray-500">Total Qty: </span>
                          <span className="text-gray-300">{pos.dca.total_qty.toFixed(4)}</span>
                        </div>
                        <div>
                          <span className="text-gray-500">Neg TP Triggered: </span>
                          <span className="text-red-400">{pos.dca.neg_tp_triggered}</span>
                        </div>
                      </div>
                    </div>
                  )}

                  {/* Wide SL Info */}
                  {pos.wide_sl.price > 0 && (
                    <div className="p-2 bg-orange-900/20 border border-orange-700/50 rounded">
                      <div className="flex items-center gap-1.5 mb-1">
                        <Shield className="w-3.5 h-3.5 text-orange-400" />
                        <span className="text-[10px] font-medium text-orange-400">Wide Stop Loss (ATR-based)</span>
                      </div>
                      <div className="grid grid-cols-3 gap-2 text-[10px]">
                        <div>
                          <span className="text-gray-500">SL Price: </span>
                          <span className="text-gray-300">{formatPrice(pos.wide_sl.price)}</span>
                        </div>
                        <div>
                          <span className="text-gray-500">ATR Mult: </span>
                          <span className="text-gray-300">{pos.wide_sl.atr_multiplier}x</span>
                        </div>
                        <div>
                          <span className="text-gray-500">AI Blocked: </span>
                          <span className={pos.wide_sl.ai_blocked ? 'text-green-400' : 'text-red-400'}>
                            {pos.wide_sl.ai_blocked ? 'YES' : 'NO'}
                          </span>
                        </div>
                      </div>
                    </div>
                  )}

                  {/* Combined PnL Summary */}
                  <div className="p-2 bg-gray-700/30 rounded border border-gray-600">
                    <div className="flex items-center gap-1.5 mb-1">
                      <DollarSign className="w-3.5 h-3.5 text-gray-400" />
                      <span className="text-[10px] font-medium text-gray-400">Combined P&L</span>
                    </div>
                    <div className="grid grid-cols-4 gap-2 text-[10px]">
                      <div className="text-center p-1.5 bg-gray-800 rounded">
                        <div className="text-gray-500">ROI</div>
                        <div className={`font-medium ${pos.combined.roi_percent >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {formatPercent(pos.combined.roi_percent)}
                        </div>
                      </div>
                      <div className="text-center p-1.5 bg-gray-800 rounded">
                        <div className="text-gray-500">Realized</div>
                        <div className={`font-medium ${pos.combined.realized_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {formatPnL(pos.combined.realized_pnl)}
                        </div>
                      </div>
                      <div className="text-center p-1.5 bg-gray-800 rounded">
                        <div className="text-gray-500">Unrealized</div>
                        <div className={`font-medium ${pos.combined.unrealized_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {formatPnL(pos.combined.unrealized_pnl)}
                        </div>
                      </div>
                      <div className="text-center p-1.5 bg-gray-800 rounded">
                        <div className="text-gray-500">Total</div>
                        <div className={`font-medium ${pos.combined.total_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {formatPnL(pos.combined.total_pnl)}
                        </div>
                      </div>
                    </div>
                  </div>

                  {/* Debug Log */}
                  {pos.debug_log && pos.debug_log.length > 0 && (
                    <div className="p-2 bg-gray-900/50 rounded border border-gray-600">
                      <div className="flex items-center gap-1.5 mb-1">
                        <Activity className="w-3.5 h-3.5 text-gray-400" />
                        <span className="text-[10px] font-medium text-gray-400">Debug Log (Last 5)</span>
                      </div>
                      <div className="space-y-0.5 max-h-24 overflow-y-auto">
                        {pos.debug_log.slice(-5).map((log, idx) => (
                          <div key={idx} className="text-[9px] text-gray-500 font-mono truncate" title={log}>
                            {log}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Current Price */}
                  <div className="text-[9px] text-gray-500 text-right">
                    Current Price: <span className="text-gray-400">{formatPrice(pos.current_price)}</span>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default HedgeModeMonitor;
