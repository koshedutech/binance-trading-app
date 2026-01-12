import { useEffect, useState } from 'react';
import { DollarSign, Zap, TrendingUp, Shield, Clock, Edit2, Save, X, AlertCircle, RefreshCw, RotateCcw } from 'lucide-react';
import { futuresApi, formatUSD, loadCapitalAllocationDefaults, ConfigResetPreview, SettingDiff, saveAdminDefaults } from '../services/futuresApi';
import ResetConfirmDialog from './ResetConfirmDialog';
import { useFuturesStore } from '../store/futuresStore';
import { useAuth } from '../contexts/AuthContext';

interface ModeAllocation {
  mode: string;
  allocated_percent: number;
  allocated_usd: number;
  used_usd: number;
  available_usd: number;
  current_positions: number;
  max_positions: number;
  capacity_percent: number;
  // Position size warning fields
  per_position_usd?: number;
  round_trip_fee_percent?: number;
  round_trip_fee_usd?: number;
  per_position_fee_usd?: number;
  break_even_move_percent?: number;
  min_recommended_usd?: number;
  optimal_min_usd?: number;
  warning_level?: 'critical' | 'warning' | 'ok';
  position_warning?: string;
}

interface ModeAllocations {
  success: boolean;
  allocations: ModeAllocation[];
  total_modes: number;
}

interface AllocationConfig {
  ultra_fast_percent: number;
  scalp_percent: number;
  swing_percent: number;
  position_percent: number;
}

export default function ModeAllocationPanel() {
  const [allocations, setAllocations] = useState<ModeAllocation[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showEdit, setShowEdit] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);

  // CRITICAL: Subscribe to trading mode changes to refresh mode-specific data (paper vs live)
  const tradingMode = useFuturesStore((state) => state.tradingMode);

  // Get admin status for reset dialog
  const { user } = useAuth();
  const isAdmin = user?.is_admin ?? false;

  const [config, setConfig] = useState<AllocationConfig>({
    ultra_fast_percent: 20,
    scalp_percent: 30,
    swing_percent: 35,
    position_percent: 15,
  });

  const [inputs, setInputs] = useState<AllocationConfig>({
    ultra_fast_percent: 20,
    scalp_percent: 30,
    swing_percent: 35,
    position_percent: 15,
  });

  // Reset Dialog state
  const [resetDialog, setResetDialog] = useState<{
    open: boolean;
    title: string;
    configType: string;
    loading: boolean;
    allMatch: boolean;
    differences: SettingDiff[];
    totalChanges: number;
    onConfirm: () => Promise<void>;
    // Admin-specific props
    isAdmin?: boolean;
    defaultValue?: any;
    onSaveDefaults?: (values: Record<string, any>) => Promise<void>;
  }>({
    open: false,
    title: '',
    configType: '',
    loading: false,
    allMatch: false,
    differences: [],
    totalChanges: 0,
    onConfirm: async () => {},
    isAdmin: false,
    defaultValue: undefined,
    onSaveDefaults: undefined,
  });
  // Key to force dialog remount on each open (ensures fresh state)
  const [dialogKey, setDialogKey] = useState(0);

  const modeNames: { [key: string]: string } = {
    ultra_fast: 'Ultra-Fast Scalping',
    scalp: 'Scalp',
    swing: 'Swing',
    position: 'Position',
  };

  const modeIcons: { [key: string]: React.ReactNode } = {
    ultra_fast: <Zap className="w-4 h-4" />,
    scalp: <TrendingUp className="w-4 h-4" />,
    swing: <Clock className="w-4 h-4" />,
    position: <Shield className="w-4 h-4" />,
  };

  const modeColors: { [key: string]: string } = {
    ultra_fast: 'from-red-500 to-orange-500',
    scalp: 'from-orange-500 to-yellow-500',
    swing: 'from-blue-500 to-cyan-500',
    position: 'from-green-500 to-emerald-500',
  };

  const fetchAllocations = async () => {
    setLoading(true);
    try {
      const data: ModeAllocations = await futuresApi.getModeAllocations();
      setAllocations(data.allocations || []);

      // Update config state with current values from API
      const ultraFast = data.allocations.find(a => a.mode === 'ultra_fast');
      const scalp = data.allocations.find(a => a.mode === 'scalp');
      const swing = data.allocations.find(a => a.mode === 'swing');
      const position = data.allocations.find(a => a.mode === 'position');

      if (ultraFast && scalp && swing && position) {
        const currentConfig = {
          ultra_fast_percent: ultraFast.allocated_percent,
          scalp_percent: scalp.allocated_percent,
          swing_percent: swing.allocated_percent,
          position_percent: position.allocated_percent,
        };
        setConfig(currentConfig);
      }

      setError(null);
      setLastUpdate(new Date());
    } catch (err) {
      setError('Failed to fetch mode allocations');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAllocations();
    const interval = setInterval(fetchAllocations, 5000);
    return () => clearInterval(interval);
  }, []);

  // CRITICAL: Refresh allocations when trading mode changes (paper <-> live)
  useEffect(() => {
    console.log('ModeAllocationPanel: Trading mode changed to', tradingMode.mode, '- refreshing');
    fetchAllocations();
  }, [tradingMode.dryRun]);

  // Define save handlers outside of the state update to ensure they're always available
  const handleAdminSaveDefaults = async (values: Record<string, any>) => {
    console.log('[CAPITAL-ALLOC] handleAdminSaveDefaults called with:', values);
    // Save to default-settings.json for new users
    await saveAdminDefaults('capital_allocation', values);
    // Also apply to current user's allocation immediately
    await futuresApi.updateModeAllocations({
      ultra_fast_percent: values['ultra_fast_percent'],
      scalp_percent: values['scalp_percent'],
      swing_percent: values['swing_percent'],
      position_percent: values['position_percent'],
    });
    setResetDialog(prev => ({ ...prev, open: false }));
    await fetchAllocations();
  };

  const handleUserResetConfirm = async () => {
    try {
      await loadCapitalAllocationDefaults(false);
      setResetDialog(prev => ({ ...prev, open: false }));
      await fetchAllocations();
      setError(null);
    } catch (err) {
      setError('Failed to reset capital allocation');
      console.error(err);
    }
  };

  const handleResetCapitalAllocation = async () => {
    try {
      // Increment key to force dialog remount with fresh state
      setDialogKey(prev => prev + 1);
      // Open dialog with loading state
      setResetDialog({
        open: true,
        title: 'Reset Capital Allocation to Defaults?',
        configType: 'capital_allocation',
        loading: true,
        allMatch: false,
        differences: [],
        totalChanges: 0,
        isAdmin: false,
        defaultValue: undefined,
        onConfirm: handleUserResetConfirm,
        onSaveDefaults: handleAdminSaveDefaults,
      });

      // Fetch preview
      const preview = await loadCapitalAllocationDefaults(true) as any;
      console.log('[CAPITAL-ALLOC] Preview response:', preview);

      // Check if this is an admin preview response
      if (preview.is_admin && preview.default_value !== undefined) {
        console.log('[CAPITAL-ALLOC] Admin preview detected, setting defaultValue');
        // Admin user: Show editable default values
        setResetDialog(prev => ({
          ...prev,
          loading: false,
          isAdmin: true,
          defaultValue: preview.default_value,
          allMatch: false,
          differences: [],
          totalChanges: 0,
          // Explicitly preserve handlers
          onConfirm: handleUserResetConfirm,
          onSaveDefaults: handleAdminSaveDefaults,
        }));
      } else {
        console.log('[CAPITAL-ALLOC] User preview detected');
        // Regular user: Show comparison
        setResetDialog(prev => ({
          ...prev,
          loading: false,
          isAdmin: false,
          defaultValue: undefined,
          allMatch: preview.all_match || false,
          differences: preview.differences || [],
          totalChanges: preview.total_changes || 0,
          // Explicitly preserve handlers
          onConfirm: handleUserResetConfirm,
          onSaveDefaults: handleAdminSaveDefaults,
        }));
      }
    } catch (err) {
      setError('Failed to load capital allocation defaults preview');
      console.error(err);
      setResetDialog(prev => ({ ...prev, open: false }));
    }
  };

  const handleInputChange = (field: keyof AllocationConfig, value: string) => {
    const numValue = parseFloat(value) || 0;
    setInputs({ ...inputs, [field]: numValue });
  };

  const validateAndSave = async () => {
    const total = inputs.ultra_fast_percent + inputs.scalp_percent + inputs.swing_percent + inputs.position_percent;

    if (total < 99 || total > 101) {
      setError(`Allocation percentages must sum to 100% (currently ${total.toFixed(1)}%)`);
      return;
    }

    setIsSaving(true);
    try {
      await futuresApi.updateModeAllocations({
        ultra_fast_percent: inputs.ultra_fast_percent,
        scalp_percent: inputs.scalp_percent,
        swing_percent: inputs.swing_percent,
        position_percent: inputs.position_percent,
      });

      setConfig(inputs);
      setShowEdit(false);
      setError(null);
      await fetchAllocations();
    } catch (err) {
      setError('Failed to update allocations');
      console.error(err);
    } finally {
      setIsSaving(false);
    }
  };

  const handleCancel = () => {
    setInputs(config);
    setShowEdit(false);
    setError(null);
  };

  const getModeAllocation = (mode: string): ModeAllocation | undefined => {
    return allocations.find(a => a.mode === mode);
  };

  const renderModeCard = (mode: string, alloc: ModeAllocation | undefined) => {
    const percent = alloc?.allocated_percent || 0;
    const allocatedUsd = alloc?.allocated_usd || 0;
    const used = alloc?.used_usd || 0;
    const available = alloc?.available_usd || 0;
    const capacity = alloc?.capacity_percent || 0;
    const currentPos = alloc?.current_positions || 0;
    const maxPos = alloc?.max_positions || 0;
    const perPositionUsd = alloc?.per_position_usd || 0;
    const warningLevel = alloc?.warning_level || 'ok';
    const positionWarning = alloc?.position_warning || '';
    const breakEvenMove = alloc?.break_even_move_percent || 0;

    const borderColor = warningLevel === 'critical' ? 'border-red-700' : warningLevel === 'warning' ? 'border-yellow-700' : 'border-gray-700';

    return (
      <div key={mode} className={`bg-gray-800 rounded-lg p-4 border ${borderColor}`}>
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <div className={`p-2 bg-gradient-to-br ${modeColors[mode]} rounded text-white`}>
              {modeIcons[mode]}
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h3 className="font-semibold text-white">{modeNames[mode]}</h3>
                {warningLevel === 'critical' && (
                  <span className="px-2 py-0.5 text-xs font-medium bg-red-900 text-red-300 rounded-full border border-red-700">⚠️ Too Small</span>
                )}
                {warningLevel === 'warning' && (
                  <span className="px-2 py-0.5 text-xs font-medium bg-yellow-900 text-yellow-300 rounded-full border border-yellow-700">⚡ Small</span>
                )}
              </div>
              <p className="text-sm text-gray-400">{percent.toFixed(1)}% allocation</p>
            </div>
          </div>
        </div>

        <div className="space-y-2">
          <div>
            <div className="flex justify-between text-sm mb-1">
              <span className="text-gray-300">Capital Allocated</span>
              <span className="text-white font-medium">{formatUSD(allocatedUsd)}</span>
            </div>
            <div className="flex justify-between text-sm mb-1">
              <span className="text-gray-300">Capital Used</span>
              <span className="text-gray-400">{formatUSD(used)} / {formatUSD(allocatedUsd)}</span>
            </div>
            <div className="w-full bg-gray-700 rounded-full h-2">
              <div className={`bg-gradient-to-r ${modeColors[mode]} h-2 rounded-full`} style={{ width: `${Math.min(allocatedUsd > 0 ? (used / allocatedUsd) * 100 : 0, 100)}%` }} />
            </div>
          </div>

          <div>
            <div className="flex justify-between text-sm mb-1">
              <span className="text-gray-300">Positions</span>
              <span className="text-gray-400">{currentPos} / {maxPos}</span>
            </div>
            <div className="w-full bg-gray-700 rounded-full h-2">
              <div className={`bg-gradient-to-r ${modeColors[mode]} h-2 rounded-full`} style={{ width: `${Math.min((currentPos / maxPos) * 100, 100)}%` }} />
            </div>
          </div>

          <div className="pt-2 border-t border-gray-700">
            <div className="flex justify-between text-xs mb-1">
              <span className="text-gray-400">Per Position:</span>
              <span className={warningLevel === 'critical' ? 'text-red-400' : warningLevel === 'warning' ? 'text-yellow-400' : 'text-green-400'}>{formatUSD(perPositionUsd)}</span>
            </div>
            <div className="flex justify-between text-xs text-gray-500">
              <span>Break-even:</span>
              <span>{breakEvenMove.toFixed(3)}%</span>
            </div>
          </div>

          {positionWarning && (
            <div className={`p-2 rounded text-xs ${warningLevel === 'critical' ? 'bg-red-900/50 text-red-300 border border-red-800' : 'bg-yellow-900/50 text-yellow-300 border border-yellow-800'}`}>
              <div className="flex items-start gap-2">
                <AlertCircle className="w-3 h-3 mt-0.5 flex-shrink-0" />
                <span>{positionWarning}</span>
              </div>
            </div>
          )}

          <div className="pt-1">
            <p className="text-xs text-gray-400">Available: {formatUSD(available)}</p>
            {capacity > 80 && (
              <div className="mt-2 flex items-center gap-2 text-xs text-yellow-400">
                <AlertCircle className="w-3 h-3" />
                <span>High capacity usage ({capacity.toFixed(0)}%)</span>
              </div>
            )}
          </div>
        </div>
      </div>
    );
  };

  const totalPercent = inputs.ultra_fast_percent + inputs.scalp_percent + inputs.swing_percent + inputs.position_percent;
  const percentError = totalPercent < 99 || totalPercent > 101;

  return (
    <div className="bg-gray-900 rounded-lg border border-gray-700 p-6">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <div className="p-3 bg-gradient-to-br from-purple-500 to-blue-500 rounded-lg text-white">
            <DollarSign className="w-6 h-6" />
          </div>
          <div>
            <h2 className="text-2xl font-bold text-white">Mode Capital Allocation</h2>
            <p className="text-gray-400 text-sm">Manage capital distribution across trading modes</p>
          </div>
        </div>
        <div className="flex gap-2">
          <button
            onClick={fetchAllocations}
            disabled={loading}
            className="p-2 hover:bg-gray-800 rounded text-gray-400 hover:text-gray-200 transition"
          >
            <RefreshCw className={`w-5 h-5 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button
            onClick={handleResetCapitalAllocation}
            className="flex items-center gap-2 px-4 py-2 bg-gray-700 hover:bg-gray-600 text-gray-300 hover:text-blue-400 rounded transition"
            title="Reset Capital Allocation to defaults"
          >
            <RotateCcw className="w-4 h-4" />
            Reset
          </button>
          {!showEdit ? (
            <button
              onClick={() => {
                setInputs(config); // Copy current config to inputs before editing
                setShowEdit(true);
              }}
              className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded transition"
            >
              <Edit2 className="w-4 h-4" />
              Edit
            </button>
          ) : null}
        </div>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-900 border border-red-700 rounded text-red-200 text-sm flex items-center gap-2">
          <AlertCircle className="w-4 h-4 flex-shrink-0" />
          {error}
        </div>
      )}

      {!showEdit ? (
        // View Mode
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {['ultra_fast', 'scalp', 'swing', 'position'].map(mode =>
            renderModeCard(mode, getModeAllocation(mode))
          )}
        </div>
      ) : (
        // Edit Mode
        <div className="space-y-4">
          <div className="bg-gray-800 rounded-lg p-4 border border-gray-700">
            <h3 className="font-semibold text-white mb-4">Adjust Allocation Percentages</h3>

            <div className="space-y-3">
              {[
                { key: 'ultra_fast_percent', label: 'Ultra-Fast Scalping' },
                { key: 'scalp_percent', label: 'Scalp' },
                { key: 'swing_percent', label: 'Swing' },
                { key: 'position_percent', label: 'Position' },
              ].map(({ key, label }) => (
                <div key={key}>
                  <label className="block text-sm text-gray-300 mb-1">{label}</label>
                  <div className="flex items-center gap-2">
                    <input
                      type="number"
                      min="0"
                      max="100"
                      step="0.1"
                      value={inputs[key as keyof AllocationConfig]}
                      onChange={(e) => handleInputChange(key as keyof AllocationConfig, e.target.value)}
                      className="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white"
                    />
                    <span className="text-gray-400 w-8">%</span>
                  </div>
                </div>
              ))}
            </div>

            <div className={`mt-4 p-3 rounded ${percentError ? 'bg-red-900 border border-red-700' : 'bg-green-900 border border-green-700'}`}>
              <p className={`text-sm ${percentError ? 'text-red-200' : 'text-green-200'}`}>
                Total: {totalPercent.toFixed(1)}% (must be 100%)
              </p>
            </div>

            <div className="flex gap-2 mt-4">
              <button
                onClick={validateAndSave}
                disabled={isSaving || percentError}
                className="flex-1 flex items-center justify-center gap-2 px-4 py-2 bg-green-600 hover:bg-green-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white rounded transition"
              >
                <Save className="w-4 h-4" />
                {isSaving ? 'Saving...' : 'Save'}
              </button>
              <button
                onClick={handleCancel}
                className="flex-1 flex items-center justify-center gap-2 px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded transition"
              >
                <X className="w-4 h-4" />
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      {lastUpdate && (
        <p className="text-xs text-gray-500 mt-4 text-right">
          Last updated: {lastUpdate.toLocaleTimeString()}
        </p>
      )}

      {/* Reset Confirm Dialog - key forces remount for fresh state */}
      <ResetConfirmDialog
        key={dialogKey}
        open={resetDialog.open}
        onClose={() => setResetDialog(prev => ({ ...prev, open: false }))}
        onConfirm={resetDialog.onConfirm}
        title={resetDialog.title}
        configType={resetDialog.configType}
        loading={resetDialog.loading}
        allMatch={resetDialog.allMatch}
        differences={resetDialog.differences}
        totalChanges={resetDialog.totalChanges}
        isAdmin={resetDialog.isAdmin}
        defaultValue={resetDialog.defaultValue}
        onSaveDefaults={resetDialog.onSaveDefaults}
        onSaveSuccess={fetchAllocations}
      />
    </div>
  );
}
