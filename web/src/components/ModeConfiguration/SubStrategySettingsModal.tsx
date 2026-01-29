// Sub-Strategy Settings Modal
// Epic 11: Position Decision Engine - Story 11.46: Volume Imbalance UI Components

import React, { useState, useCallback, useEffect } from 'react';
import {
  X,
  Save,
  RefreshCw,
  AlertTriangle,
  CheckCircle2,
  Loader2,
  Brain,
  Target,
  TrendingUp,
  Clock,
  BarChart3,
  Shield,
  Info,
  ChevronDown,
  ChevronUp,
  Plus,
  Trash2,
  Wallet,
} from 'lucide-react';
import type {
  VolumeImbalanceSettings,
  TrailingStopMilestone,
  DEFAULT_VOLUME_IMBALANCE_SETTINGS,
} from '../../types/strategyHierarchy';

// ==================== Interfaces ====================

interface SubStrategySettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
  strategyId: string;
  strategyName: string;
  settings: VolumeImbalanceSettings;
  onSave: (settings: VolumeImbalanceSettings) => Promise<void>;
  isLoading?: boolean;
}

interface SectionProps {
  title: string;
  icon: React.ReactNode;
  description?: string;
  defaultExpanded?: boolean;
  children: React.ReactNode;
}

// ==================== Collapsible Section Component ====================

function SettingsSection({
  title,
  icon,
  description,
  defaultExpanded = true,
  children,
}: SectionProps) {
  const [isExpanded, setIsExpanded] = useState(defaultExpanded);

  return (
    <div className="border border-gray-700 rounded-lg overflow-hidden">
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className="w-full flex items-center justify-between p-3 bg-gray-700/30 hover:bg-gray-700/50 transition-colors"
      >
        <div className="flex items-center gap-2">
          <span className="text-purple-400">{icon}</span>
          <span className="text-sm font-medium text-white">{title}</span>
        </div>
        {isExpanded ? (
          <ChevronUp className="w-4 h-4 text-gray-400" />
        ) : (
          <ChevronDown className="w-4 h-4 text-gray-400" />
        )}
      </button>

      {isExpanded && (
        <div className="p-4 space-y-4">
          {description && (
            <p className="text-xs text-gray-500 mb-4">{description}</p>
          )}
          {children}
        </div>
      )}
    </div>
  );
}

// ==================== Input Components ====================

interface NumberInputProps {
  label: string;
  value: number;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  unit?: string;
  disabled?: boolean;
}

function NumberInput({
  label,
  value,
  onChange,
  min,
  max,
  step = 1,
  unit,
  disabled = false,
}: NumberInputProps) {
  return (
    <div className="flex items-center justify-between">
      <label className="text-sm text-gray-300">{label}</label>
      <div className="flex items-center gap-2">
        <input
          type="number"
          value={value}
          onChange={(e) => onChange(Number(e.target.value))}
          min={min}
          max={max}
          step={step}
          disabled={disabled}
          className="w-24 px-2 py-1 text-sm text-right bg-gray-700 border border-gray-600 rounded focus:outline-none focus:border-purple-500 disabled:opacity-50"
        />
        {unit && <span className="text-xs text-gray-500 w-8">{unit}</span>}
      </div>
    </div>
  );
}

interface ToggleInputProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  description?: string;
  disabled?: boolean;
}

function ToggleInput({
  label,
  checked,
  onChange,
  description,
  disabled = false,
}: ToggleInputProps) {
  return (
    <div className="flex items-start justify-between py-2">
      <div className="flex-1">
        <label className="text-sm text-gray-300">{label}</label>
        {description && (
          <p className="text-xs text-gray-500 mt-0.5">{description}</p>
        )}
      </div>
      <button
        type="button"
        onClick={() => !disabled && onChange(!checked)}
        disabled={disabled}
        className={`
          relative inline-flex h-5 w-9 items-center rounded-full transition-colors
          ${checked ? 'bg-purple-600' : 'bg-gray-600'}
          ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
        `}
      >
        <span
          className={`
            inline-block h-3 w-3 transform rounded-full bg-white transition-transform
            ${checked ? 'translate-x-5' : 'translate-x-1'}
          `}
        />
      </button>
    </div>
  );
}

interface SelectInputProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
  disabled?: boolean;
}

function SelectInput({
  label,
  value,
  onChange,
  options,
  disabled = false,
}: SelectInputProps) {
  return (
    <div className="flex items-center justify-between">
      <label className="text-sm text-gray-300">{label}</label>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        className="px-2 py-1 text-sm bg-gray-700 border border-gray-600 rounded focus:outline-none focus:border-purple-500 disabled:opacity-50"
      >
        {options.map((opt) => (
          <option key={opt.value} value={opt.value}>
            {opt.label}
          </option>
        ))}
      </select>
    </div>
  );
}

// ==================== Milestone Editor Component ====================

interface MilestoneEditorProps {
  milestones: TrailingStopMilestone[];
  onChange: (milestones: TrailingStopMilestone[]) => void;
  disabled?: boolean;
}

function MilestoneEditor({ milestones, onChange, disabled = false }: MilestoneEditorProps) {
  const addMilestone = () => {
    const lastMilestone = milestones[milestones.length - 1];
    const newMilestone: TrailingStopMilestone = {
      trigger_profit_pct: lastMilestone ? lastMilestone.trigger_profit_pct + 1 : 2,
      trail_distance_pct: 0.3,
      label: `TP${milestones.length + 1}`,
    };
    onChange([...milestones, newMilestone]);
  };

  const updateMilestone = (index: number, field: keyof TrailingStopMilestone, value: number | string) => {
    const updated = [...milestones];
    updated[index] = { ...updated[index], [field]: value };
    onChange(updated);
  };

  const removeMilestone = (index: number) => {
    onChange(milestones.filter((_, i) => i !== index));
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between mb-2">
        <span className="text-sm text-gray-400">Trailing Stop Milestones</span>
        <button
          onClick={addMilestone}
          disabled={disabled || milestones.length >= 5}
          className="flex items-center gap-1 px-2 py-1 text-xs text-purple-400 hover:text-purple-300 hover:bg-purple-500/10 rounded transition-colors disabled:opacity-50"
        >
          <Plus className="w-3 h-3" />
          Add
        </button>
      </div>

      {milestones.length === 0 ? (
        <p className="text-xs text-gray-500 text-center py-2">No milestones configured</p>
      ) : (
        <div className="space-y-2">
          {milestones.map((milestone, index) => (
            <div
              key={index}
              className="flex items-center gap-2 p-2 bg-gray-700/30 rounded"
            >
              <input
                type="text"
                value={milestone.label || ''}
                onChange={(e) => updateMilestone(index, 'label', e.target.value)}
                disabled={disabled}
                placeholder="Label"
                className="w-16 px-2 py-1 text-xs bg-gray-700 border border-gray-600 rounded"
              />
              <div className="flex items-center gap-1">
                <span className="text-xs text-gray-500">At</span>
                <input
                  type="number"
                  value={milestone.trigger_profit_pct}
                  onChange={(e) => updateMilestone(index, 'trigger_profit_pct', Number(e.target.value))}
                  disabled={disabled}
                  min={0}
                  max={100}
                  step={0.5}
                  className="w-14 px-2 py-1 text-xs text-right bg-gray-700 border border-gray-600 rounded"
                />
                <span className="text-xs text-gray-500">%</span>
              </div>
              <div className="flex items-center gap-1">
                <span className="text-xs text-gray-500">Trail</span>
                <input
                  type="number"
                  value={milestone.trail_distance_pct}
                  onChange={(e) => updateMilestone(index, 'trail_distance_pct', Number(e.target.value))}
                  disabled={disabled}
                  min={0.1}
                  max={10}
                  step={0.1}
                  className="w-14 px-2 py-1 text-xs text-right bg-gray-700 border border-gray-600 rounded"
                />
                <span className="text-xs text-gray-500">%</span>
              </div>
              <button
                onClick={() => removeMilestone(index)}
                disabled={disabled}
                className="p-1 text-gray-500 hover:text-red-400 transition-colors disabled:opacity-50"
              >
                <Trash2 className="w-3 h-3" />
              </button>
            </div>
          ))}
        </div>
      )}

      <p className="text-[10px] text-gray-600 mt-1">
        Milestones tighten the trailing stop as profit increases
      </p>
    </div>
  );
}

// ==================== R:R Display Component ====================

interface RiskRewardDisplayProps {
  risk: number;
  reward: number;
  onRiskChange: (value: number) => void;
  onRewardChange: (value: number) => void;
  minRatio: number;
  onMinRatioChange: (value: number) => void;
  disabled?: boolean;
}

function RiskRewardDisplay({
  risk,
  reward,
  onRiskChange,
  onRewardChange,
  minRatio,
  onMinRatioChange,
  disabled = false,
}: RiskRewardDisplayProps) {
  const ratio = reward / risk;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-center gap-4 p-4 bg-gray-700/30 rounded-lg">
        <div className="text-center">
          <label className="text-xs text-gray-500 block mb-1">Risk</label>
          <input
            type="number"
            value={risk}
            onChange={(e) => onRiskChange(Number(e.target.value))}
            disabled={disabled}
            min={1}
            max={10}
            className="w-12 px-2 py-1 text-center text-lg font-bold bg-red-500/10 border border-red-500/30 text-red-400 rounded"
          />
        </div>
        <span className="text-2xl font-bold text-gray-400">:</span>
        <div className="text-center">
          <label className="text-xs text-gray-500 block mb-1">Reward</label>
          <input
            type="number"
            value={reward}
            onChange={(e) => onRewardChange(Number(e.target.value))}
            disabled={disabled}
            min={1}
            max={20}
            className="w-12 px-2 py-1 text-center text-lg font-bold bg-green-500/10 border border-green-500/30 text-green-400 rounded"
          />
        </div>
        <div className="ml-4 text-center">
          <span className="text-xs text-gray-500 block">Ratio</span>
          <span className="text-lg font-bold text-purple-400">{ratio.toFixed(1)}x</span>
        </div>
      </div>

      <NumberInput
        label="Minimum R:R to accept trade"
        value={minRatio}
        onChange={onMinRatioChange}
        min={1}
        max={20}
        step={0.5}
        unit="x"
        disabled={disabled}
      />
    </div>
  );
}

// ==================== Main Modal Component ====================

export default function SubStrategySettingsModal({
  isOpen,
  onClose,
  strategyId,
  strategyName,
  settings: initialSettings,
  onSave,
  isLoading = false,
}: SubStrategySettingsModalProps) {
  const [settings, setSettings] = useState<VolumeImbalanceSettings>(initialSettings);
  const [hasChanges, setHasChanges] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  // Reset when modal opens with new settings
  useEffect(() => {
    setSettings(initialSettings);
    setHasChanges(false);
    setError(null);
    setSuccess(false);
  }, [initialSettings, isOpen]);

  // Update setting helper
  const updateSetting = useCallback(<K extends keyof VolumeImbalanceSettings>(
    key: K,
    value: VolumeImbalanceSettings[K]
  ) => {
    setSettings((prev) => ({ ...prev, [key]: value }));
    setHasChanges(true);
    setError(null);
    setSuccess(false);
  }, []);

  // Update nested setting helper
  const updateNestedSetting = useCallback(<
    K extends keyof VolumeImbalanceSettings,
    NK extends keyof NonNullable<VolumeImbalanceSettings[K]>
  >(
    key: K,
    nestedKey: NK,
    value: unknown
  ) => {
    setSettings((prev) => ({
      ...prev,
      [key]: {
        ...(prev[key] as object),
        [nestedKey]: value,
      },
    }));
    setHasChanges(true);
    setError(null);
    setSuccess(false);
  }, []);

  // Handle save
  const handleSave = async () => {
    setIsSaving(true);
    setError(null);

    try {
      await onSave(settings);
      setSuccess(true);
      setHasChanges(false);
      setTimeout(() => setSuccess(false), 2000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save settings');
    } finally {
      setIsSaving(false);
    }
  };

  // Handle reset
  const handleReset = () => {
    setSettings(initialSettings);
    setHasChanges(false);
    setError(null);
    setSuccess(false);
  };

  if (!isOpen) return null;

  const isDisabled = isLoading || isSaving;

  // LLM provider options
  const llmProviders = [
    { value: 'openai', label: 'OpenAI' },
    { value: 'anthropic', label: 'Anthropic' },
    { value: 'groq', label: 'Groq' },
    { value: 'gemini', label: 'Google Gemini' },
  ];

  // Timeframe options (including backtested 1m and 3m)
  const timeframeOptions = [
    { value: '1m', label: '1 minute' },
    { value: '3m', label: '3 minutes (backtested)' },
    { value: '5m', label: '5 minutes' },
    { value: '15m', label: '15 minutes' },
    { value: '30m', label: '30 minutes' },
    { value: '1h', label: '1 hour' },
    { value: '4h', label: '4 hours' },
  ];

  // Position sizing options
  const positionSizingOptions = [
    { value: 'all_in', label: 'All In (100% of budget)' },
    { value: 'fixed_percent', label: 'Fixed Percent' },
    { value: 'kelly', label: 'Kelly Criterion' },
  ];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4">
      <div className="relative w-full max-w-2xl flex flex-col bg-gray-800 rounded-xl border border-gray-700 shadow-2xl" style={{ maxHeight: 'calc(100vh - 2rem)' }}>
        {/* Header */}
        <div className="flex-shrink-0 flex items-center justify-between p-4 border-b border-gray-700">
          <div>
            <h2 className="text-lg font-semibold text-white">{strategyName} Settings</h2>
            <p className="text-xs text-gray-500">Configure strategy parameters (scroll down for all options)</p>
          </div>
          <button
            onClick={onClose}
            className="p-2 text-gray-400 hover:text-white hover:bg-gray-700 rounded-lg transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content - Scrollable */}
        <div className="flex-1 p-4 overflow-y-auto space-y-4">
          {/* Status Messages */}
          {error && (
            <div className="flex items-center gap-2 p-3 bg-red-500/10 border border-red-500/30 rounded-lg text-sm text-red-400">
              <AlertTriangle className="w-4 h-4" />
              {error}
            </div>
          )}

          {success && (
            <div className="flex items-center gap-2 p-3 bg-green-500/10 border border-green-500/30 rounded-lg text-sm text-green-400">
              <CheckCircle2 className="w-4 h-4" />
              Settings saved successfully
            </div>
          )}

          {/* Risk/Reward Section */}
          <SettingsSection
            title="Risk/Reward Ratio"
            icon={<Target className="w-4 h-4" />}
            description="Configure the target risk to reward ratio for trades"
          >
            <RiskRewardDisplay
              risk={settings.risk_reward.risk}
              reward={settings.risk_reward.reward}
              onRiskChange={(v) => updateNestedSetting('risk_reward', 'risk', v)}
              onRewardChange={(v) => updateNestedSetting('risk_reward', 'reward', v)}
              minRatio={settings.risk_reward.min_ratio}
              onMinRatioChange={(v) => updateNestedSetting('risk_reward', 'min_ratio', v)}
              disabled={isDisabled}
            />
          </SettingsSection>

          {/* LLM Validation Section */}
          <SettingsSection
            title="LLM Validation"
            icon={<Brain className="w-4 h-4" />}
            description="AI-powered trade validation before execution"
          >
            <ToggleInput
              label="Enable LLM Validation"
              checked={settings.llm_validation_enabled}
              onChange={(v) => updateSetting('llm_validation_enabled', v)}
              description="Require AI approval before executing trades"
              disabled={isDisabled}
            />
            {settings.llm_validation_enabled && (
              <SelectInput
                label="LLM Provider"
                value={settings.llm_provider || 'anthropic'}
                onChange={(v) => updateSetting('llm_provider', v)}
                options={llmProviders}
                disabled={isDisabled}
              />
            )}
          </SettingsSection>

          {/* Trailing Stop Section */}
          <SettingsSection
            title="Trailing Stop"
            icon={<Shield className="w-4 h-4" />}
            description="Dynamic trailing stop with profit-based milestones"
          >
            <ToggleInput
              label="Enable Trailing Stop"
              checked={settings.trailing_stop.enabled}
              onChange={(v) => updateNestedSetting('trailing_stop', 'enabled', v)}
              disabled={isDisabled}
            />
            {settings.trailing_stop.enabled && (
              <>
                <NumberInput
                  label="Activation Profit"
                  value={settings.trailing_stop.activation_profit_pct}
                  onChange={(v) => updateNestedSetting('trailing_stop', 'activation_profit_pct', v)}
                  min={0.1}
                  max={10}
                  step={0.1}
                  unit="%"
                  disabled={isDisabled}
                />
                <NumberInput
                  label="Initial Trail Distance"
                  value={settings.trailing_stop.initial_trail_pct}
                  onChange={(v) => updateNestedSetting('trailing_stop', 'initial_trail_pct', v)}
                  min={0.1}
                  max={5}
                  step={0.1}
                  unit="%"
                  disabled={isDisabled}
                />
                <div className="mt-4 pt-4 border-t border-gray-700">
                  <MilestoneEditor
                    milestones={settings.trailing_stop.milestones}
                    onChange={(milestones) =>
                      updateSetting('trailing_stop', {
                        ...settings.trailing_stop,
                        milestones,
                      })
                    }
                    disabled={isDisabled}
                  />
                </div>
              </>
            )}
          </SettingsSection>

          {/* Pattern Detection Section */}
          <SettingsSection
            title="Pattern Detection"
            icon={<BarChart3 className="w-4 h-4" />}
            description="Backtested parameters (Dec 2025 - Jan 2026: 51 trades, 47.1% WR, +1147% return)"
            defaultExpanded={true}
          >
            {/* Reference Candle Selection */}
            <div className="mb-3">
              <span className="text-xs text-purple-400 font-medium">Reference Candle Selection</span>
            </div>
            <NumberInput
              label="Lookback Candles"
              value={settings.pattern_detection.reference_lookback_candles || 5}
              onChange={(v) => updateNestedSetting('pattern_detection', 'reference_lookback_candles', v)}
              min={1}
              max={50}
              step={1}
              disabled={isDisabled}
            />
            <NumberInput
              label="Volume Spike Threshold"
              value={settings.pattern_detection.volume_spike_threshold || 3.0}
              onChange={(v) => updateNestedSetting('pattern_detection', 'volume_spike_threshold', v)}
              min={1}
              max={10}
              step={0.1}
              unit="x"
              disabled={isDisabled}
            />

            {/* Consolidation Parameters */}
            <div className="mt-4 pt-4 border-t border-gray-700 mb-3">
              <span className="text-xs text-purple-400 font-medium">Consolidation Phase</span>
            </div>
            <NumberInput
              label="Min Consolidation Candles"
              value={settings.pattern_detection.min_consolidation_candles || 1}
              onChange={(v) => updateNestedSetting('pattern_detection', 'min_consolidation_candles', v)}
              min={0}
              max={20}
              step={1}
              disabled={isDisabled}
            />
            <NumberInput
              label="Max Consolidation Candles"
              value={settings.pattern_detection.max_consolidation_candles || 999}
              onChange={(v) => updateNestedSetting('pattern_detection', 'max_consolidation_candles', v)}
              min={1}
              max={999}
              step={1}
              disabled={isDisabled}
            />
            <NumberInput
              label="Range Tolerance"
              value={(settings.pattern_detection.consolidation_range_tolerance || 0.01) * 100}
              onChange={(v) => updateNestedSetting('pattern_detection', 'consolidation_range_tolerance', v / 100)}
              min={0.1}
              max={5}
              step={0.1}
              unit="%"
              disabled={isDisabled}
            />

            {/* Breakout Parameters */}
            <div className="mt-4 pt-4 border-t border-gray-700 mb-3">
              <span className="text-xs text-purple-400 font-medium">Breakout Confirmation</span>
            </div>
            <NumberInput
              label="Breakout Volume Surge"
              value={settings.pattern_detection.breakout_volume_surge || 1.0}
              onChange={(v) => updateNestedSetting('pattern_detection', 'breakout_volume_surge', v)}
              min={0.5}
              max={5}
              step={0.1}
              unit="x"
              disabled={isDisabled}
            />
            <NumberInput
              label="Entry Vol vs Reference"
              value={settings.pattern_detection.entry_volume_vs_reference || 1.0}
              onChange={(v) => updateNestedSetting('pattern_detection', 'entry_volume_vs_reference', v)}
              min={0.5}
              max={5}
              step={0.1}
              unit="x"
              disabled={isDisabled}
            />

            {/* Risk Management */}
            <div className="mt-4 pt-4 border-t border-gray-700 mb-3">
              <span className="text-xs text-purple-400 font-medium">Risk Management</span>
            </div>
            <NumberInput
              label="Max Stop Loss"
              value={(settings.pattern_detection.max_sl_percent || 1.5)}
              onChange={(v) => updateNestedSetting('pattern_detection', 'max_sl_percent', v)}
              min={0.5}
              max={5}
              step={0.1}
              unit="%"
              disabled={isDisabled}
            />

            {/* HTF Confirmation */}
            <div className="mt-4 pt-4 border-t border-gray-700">
              <ToggleInput
                label="Require HTF Confirmation"
                checked={settings.pattern_detection.require_htf_confirmation || false}
                onChange={(v) => updateNestedSetting('pattern_detection', 'require_htf_confirmation', v)}
                description="Validate pattern with higher timeframe analysis"
                disabled={isDisabled}
              />
              {settings.pattern_detection.require_htf_confirmation && (
                <SelectInput
                  label="HTF Timeframe"
                  value={settings.pattern_detection.htf_timeframe || '15m'}
                  onChange={(v) => updateNestedSetting('pattern_detection', 'htf_timeframe', v)}
                  options={timeframeOptions}
                  disabled={isDisabled}
                />
              )}
            </div>
          </SettingsSection>

          {/* Budget Allocation Section */}
          <SettingsSection
            title="Budget Allocation"
            icon={<Wallet className="w-4 h-4" />}
            description="Per-strategy capital management for independent equity tracking"
            defaultExpanded={true}
          >
            <NumberInput
              label="Assigned Budget"
              value={settings.budget_allocation?.assigned_budget_usd || 100}
              onChange={(v) => {
                const ba = settings.budget_allocation || {
                  assigned_budget_usd: 100,
                  max_concurrent_trades: 1,
                  position_sizing: 'all_in',
                  use_incremental_equity: true,
                };
                updateSetting('budget_allocation', { ...ba, assigned_budget_usd: v });
              }}
              min={10}
              max={100000}
              step={10}
              unit="USD"
              disabled={isDisabled}
            />
            <NumberInput
              label="Max Concurrent Trades"
              value={settings.budget_allocation?.max_concurrent_trades || 1}
              onChange={(v) => {
                const ba = settings.budget_allocation || {
                  assigned_budget_usd: 100,
                  max_concurrent_trades: 1,
                  position_sizing: 'all_in',
                  use_incremental_equity: true,
                };
                updateSetting('budget_allocation', { ...ba, max_concurrent_trades: v });
              }}
              min={1}
              max={10}
              step={1}
              disabled={isDisabled}
            />
            <SelectInput
              label="Position Sizing"
              value={settings.budget_allocation?.position_sizing || 'all_in'}
              onChange={(v) => {
                const ba = settings.budget_allocation || {
                  assigned_budget_usd: 100,
                  max_concurrent_trades: 1,
                  position_sizing: 'all_in',
                  use_incremental_equity: true,
                };
                updateSetting('budget_allocation', { ...ba, position_sizing: v });
              }}
              options={positionSizingOptions}
              disabled={isDisabled}
            />
            <ToggleInput
              label="Use Incremental Equity"
              checked={settings.budget_allocation?.use_incremental_equity ?? true}
              onChange={(v) => {
                const ba = settings.budget_allocation || {
                  assigned_budget_usd: 100,
                  max_concurrent_trades: 1,
                  position_sizing: 'all_in',
                  use_incremental_equity: true,
                };
                updateSetting('budget_allocation', { ...ba, use_incremental_equity: v });
              }}
              description="Profits compound into position sizing (grows with wins)"
              disabled={isDisabled}
            />
          </SettingsSection>

          {/* General Settings Section */}
          <SettingsSection
            title="General Settings"
            icon={<TrendingUp className="w-4 h-4" />}
            defaultExpanded={false}
          >
            <NumberInput
              label="Max Concurrent Patterns"
              value={settings.max_concurrent_patterns}
              onChange={(v) => updateSetting('max_concurrent_patterns', v)}
              min={1}
              max={20}
              step={1}
              disabled={isDisabled}
            />
            <NumberInput
              label="Priority"
              value={settings.priority}
              onChange={(v) => updateSetting('priority', v)}
              min={1}
              max={10}
              step={1}
              disabled={isDisabled}
            />
          </SettingsSection>
        </div>

        {/* Footer - Always visible at bottom */}
        <div className="flex-shrink-0 flex items-center justify-between p-4 border-t border-gray-700 bg-gray-800/50">
          <div className="flex items-center gap-2">
            {hasChanges && (
              <span className="px-2 py-1 text-xs bg-yellow-500/20 text-yellow-400 rounded">
                Unsaved changes
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={handleReset}
              disabled={!hasChanges || isDisabled}
              className="flex items-center gap-2 px-3 py-2 text-sm text-gray-300 hover:text-white hover:bg-gray-700 rounded-lg transition-colors disabled:opacity-50"
            >
              <RefreshCw className="w-4 h-4" />
              Reset
            </button>
            <button
              onClick={handleSave}
              disabled={!hasChanges || isDisabled}
              className="flex items-center gap-2 px-4 py-2 text-sm bg-purple-600 hover:bg-purple-500 text-white rounded-lg transition-colors disabled:opacity-50"
            >
              {isSaving ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <Save className="w-4 h-4" />
              )}
              Save Changes
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
