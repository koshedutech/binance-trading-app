import { useEffect, useState } from 'react';
import { DollarSign, Zap, TrendingUp, Shield, Clock, Edit2, Save, X, AlertCircle, RefreshCw } from 'lucide-react';
import { futuresApi, formatUSD } from '../services/futuresApi';

interface ModeAllocation {
  mode: string;
  allocated_percent: number;
  allocated_usd: number;
  used_usd: number;
  available_usd: number;
  current_positions: number;
  max_positions: number;
  capacity_percent: number;
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
    const used = alloc?.used_usd || 0;
    const available = alloc?.available_usd || 0;
    const capacity = alloc?.capacity_percent || 0;
    const currentPos = alloc?.current_positions || 0;
    const maxPos = alloc?.max_positions || 0;

    return (
      <div key={mode} className="bg-gray-800 rounded-lg p-4 border border-gray-700">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <div className={`p-2 bg-gradient-to-br ${modeColors[mode]} rounded text-white`}>
              {modeIcons[mode]}
            </div>
            <div>
              <h3 className="font-semibold text-white">{modeNames[mode]}</h3>
              <p className="text-sm text-gray-400">{percent.toFixed(1)}% allocation</p>
            </div>
          </div>
        </div>

        <div className="space-y-2">
          {/* Capital utilization bar */}
          <div>
            <div className="flex justify-between text-sm mb-1">
              <span className="text-gray-300">Capital Used</span>
              <span className="text-gray-400">{formatUSD(used)} / {formatUSD(used + available)}</span>
            </div>
            <div className="w-full bg-gray-700 rounded-full h-2">
              <div
                className={`bg-gradient-to-r ${modeColors[mode]} h-2 rounded-full`}
                style={{ width: `${Math.min((used / (used + available)) * 100, 100)}%` }}
              />
            </div>
          </div>

          {/* Position utilization bar */}
          <div>
            <div className="flex justify-between text-sm mb-1">
              <span className="text-gray-300">Positions</span>
              <span className="text-gray-400">{currentPos} / {maxPos}</span>
            </div>
            <div className="w-full bg-gray-700 rounded-full h-2">
              <div
                className={`bg-gradient-to-r ${modeColors[mode]} h-2 rounded-full`}
                style={{ width: `${Math.min((currentPos / maxPos) * 100, 100)}%` }}
              />
            </div>
          </div>

          {/* Available capital indicator */}
          <div className="pt-2 border-t border-gray-700">
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
          {!showEdit ? (
            <button
              onClick={() => setShowEdit(true)}
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
    </div>
  );
}
