// Strategy Group Panel
// Epic 11: Position Decision Engine - Story 11.46: Volume Imbalance UI Components

import React, { useState, useCallback } from 'react';
import {
  ChevronDown,
  ChevronUp,
  Settings,
  ToggleLeft,
  ToggleRight,
  Clock,
  DollarSign,
  TrendingUp,
  Layers,
  BarChart3,
  Loader2,
  AlertTriangle,
  CheckCircle2,
  RefreshCw,
} from 'lucide-react';
import type {
  StrategyGroupSettings,
  StrategyGroupBaseSettings,
  SubStrategyInfo,
  UpdateStrategyGroupRequest,
} from '../../types/strategyHierarchy';

// ==================== Interfaces ====================

interface StrategyGroupPanelProps {
  group: StrategyGroupSettings;
  onToggleGroup: (enabled: boolean) => Promise<void>;
  onUpdateBaseSettings: (settings: Partial<StrategyGroupBaseSettings>) => Promise<void>;
  onToggleSubStrategy: (subStrategyId: string, enabled: boolean) => Promise<void>;
  onOpenSubStrategySettings: (subStrategy: SubStrategyInfo) => void;
  isLoading?: boolean;
  disabled?: boolean;
}

interface BaseSettingInputProps {
  label: string;
  value: number | string;
  unit?: string;
  icon: React.ReactNode;
  onChange: (value: number | string) => void;
  min?: number;
  max?: number;
  step?: number;
  type?: 'number' | 'select';
  options?: { value: string; label: string }[];
  disabled?: boolean;
}

// ==================== Base Setting Input Component ====================

function BaseSettingInput({
  label,
  value,
  unit,
  icon,
  onChange,
  min,
  max,
  step = 1,
  type = 'number',
  options,
  disabled = false,
}: BaseSettingInputProps) {
  const inputId = label.toLowerCase().replace(/\s+/g, '-') + '-input';

  return (
    <div className="flex items-center justify-between py-2 border-b border-gray-700/50 last:border-0">
      <div className="flex items-center gap-2">
        <span className="text-gray-500">{icon}</span>
        <label htmlFor={inputId} className="text-sm text-gray-300">{label}</label>
      </div>
      <div className="flex items-center gap-2">
        {type === 'number' ? (
          <input
            id={inputId}
            type="number"
            value={value}
            onChange={(e) => onChange(Number(e.target.value))}
            min={min}
            max={max}
            step={step}
            disabled={disabled}
            className="w-24 px-2 py-1 text-sm text-right bg-gray-700 border border-gray-600 rounded focus:outline-none focus:border-purple-500 disabled:opacity-50"
          />
        ) : (
          <select
            id={inputId}
            value={value}
            onChange={(e) => onChange(e.target.value)}
            disabled={disabled}
            className="px-2 py-1 text-sm bg-gray-700 border border-gray-600 rounded focus:outline-none focus:border-purple-500 disabled:opacity-50"
          >
            {options?.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        )}
        {unit && <span className="text-xs text-gray-500">{unit}</span>}
      </div>
    </div>
  );
}

// ==================== Toggle Switch Component ====================

interface ToggleSwitchProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  size?: 'sm' | 'md';
}

function ToggleSwitch({ checked, onChange, disabled = false, size = 'md' }: ToggleSwitchProps) {
  const sizeClasses = size === 'sm' ? 'h-5 w-9' : 'h-6 w-11';
  const dotSize = size === 'sm' ? 'h-3 w-3' : 'h-4 w-4';
  const translate = size === 'sm' ? 'translate-x-5' : 'translate-x-6';

  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      onClick={() => !disabled && onChange(!checked)}
      disabled={disabled}
      className={`
        relative inline-flex ${sizeClasses} items-center rounded-full transition-colors
        ${checked ? 'bg-purple-600' : 'bg-gray-600'}
        ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
      `}
    >
      <span
        className={`
          inline-block ${dotSize} transform rounded-full bg-white transition-transform
          ${checked ? translate : 'translate-x-1'}
        `}
      />
    </button>
  );
}

// ==================== Sub-Strategy Row Component ====================

interface SubStrategyRowProps {
  subStrategy: SubStrategyInfo;
  onToggle: (enabled: boolean) => void;
  onOpenSettings: () => void;
  disabled?: boolean;
}

function SubStrategyRow({ subStrategy, onToggle, onOpenSettings, disabled = false }: SubStrategyRowProps) {
  return (
    <div
      className={`
        flex items-center justify-between p-3 rounded-lg transition-colors
        ${subStrategy.enabled ? 'bg-gray-700/50' : 'bg-gray-800/30'}
        ${disabled ? 'opacity-50' : ''}
      `}
    >
      <div className="flex items-center gap-3">
        <ToggleSwitch
          checked={subStrategy.enabled}
          onChange={onToggle}
          disabled={disabled}
          size="sm"
        />
        <div>
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium text-gray-200">{subStrategy.name}</span>
            <span className="px-1.5 py-0.5 text-[10px] bg-gray-600 text-gray-300 rounded">
              P{subStrategy.priority}
            </span>
          </div>
          <p className="text-xs text-gray-500 mt-0.5">{subStrategy.description}</p>
        </div>
      </div>
      <button
        onClick={onOpenSettings}
        disabled={disabled}
        className="p-2 text-gray-400 hover:text-purple-400 hover:bg-gray-700 rounded transition-colors disabled:opacity-50"
        title="Configure sub-strategy"
      >
        <Settings className="w-4 h-4" />
      </button>
    </div>
  );
}

// ==================== Main Component ====================

export default function StrategyGroupPanel({
  group,
  onToggleGroup,
  onUpdateBaseSettings,
  onToggleSubStrategy,
  onOpenSubStrategySettings,
  isLoading = false,
  disabled = false,
}: StrategyGroupPanelProps) {
  const [isExpanded, setIsExpanded] = useState(true);
  const [isSettingsExpanded, setIsSettingsExpanded] = useState(false);
  const [localSettings, setLocalSettings] = useState(group.base_settings);
  const [hasChanges, setHasChanges] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const isDisabled = disabled || isLoading;

  // Timeframe options
  const timeframeOptions = [
    { value: '1m', label: '1 minute' },
    { value: '3m', label: '3 minutes' },
    { value: '5m', label: '5 minutes' },
    { value: '15m', label: '15 minutes' },
    { value: '30m', label: '30 minutes' },
    { value: '1h', label: '1 hour' },
    { value: '4h', label: '4 hours' },
    { value: '1d', label: '1 day' },
  ];

  // Handle local setting change
  const handleSettingChange = useCallback(
    (key: keyof StrategyGroupBaseSettings, value: number | string) => {
      setLocalSettings((prev) => ({ ...prev, [key]: value }));
      setHasChanges(true);
      setSaveError(null);
    },
    []
  );

  // Save settings
  const handleSaveSettings = async () => {
    setIsSaving(true);
    setSaveError(null);

    try {
      await onUpdateBaseSettings(localSettings);
      setHasChanges(false);
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : 'Failed to save settings');
    } finally {
      setIsSaving(false);
    }
  };

  // Reset to original
  const handleResetSettings = () => {
    setLocalSettings(group.base_settings);
    setHasChanges(false);
    setSaveError(null);
  };

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between p-4 border-b border-gray-700">
        <div className="flex items-center gap-3">
          {/* Enable/Disable Toggle */}
          <div className="flex items-center">
            {group.enabled ? (
              <ToggleRight
                className="w-8 h-8 text-purple-400 cursor-pointer hover:text-purple-300 transition-colors"
                onClick={() => !isDisabled && onToggleGroup(false)}
              />
            ) : (
              <ToggleLeft
                className="w-8 h-8 text-gray-500 cursor-pointer hover:text-gray-400 transition-colors"
                onClick={() => !isDisabled && onToggleGroup(true)}
              />
            )}
          </div>

          {/* Group Info */}
          <div>
            <div className="flex items-center gap-2">
              <h3 className="font-semibold text-white">{group.name}</h3>
              {isLoading && <Loader2 className="w-4 h-4 animate-spin text-purple-400" />}
              {!group.enabled && (
                <span className="px-2 py-0.5 text-xs bg-gray-600 text-gray-300 rounded">
                  Disabled
                </span>
              )}
            </div>
            <p className="text-xs text-gray-500 mt-0.5">{group.description}</p>
          </div>
        </div>

        {/* Expand/Collapse */}
        <button
          onClick={() => setIsExpanded(!isExpanded)}
          aria-expanded={isExpanded}
          aria-label={`${isExpanded ? 'Collapse' : 'Expand'} strategy group settings`}
          className="p-2 text-gray-400 hover:text-white hover:bg-gray-700 rounded transition-colors"
        >
          {isExpanded ? (
            <ChevronUp className="w-5 h-5" />
          ) : (
            <ChevronDown className="w-5 h-5" />
          )}
        </button>
      </div>

      {/* Content */}
      {isExpanded && (
        <div className="p-4">
          {/* Base Settings Section */}
          <div className="mb-4">
            <button
              onClick={() => setIsSettingsExpanded(!isSettingsExpanded)}
              className="flex items-center justify-between w-full py-2 text-sm font-medium text-gray-400 hover:text-white transition-colors"
            >
              <div className="flex items-center gap-2">
                <Settings className="w-4 h-4" />
                <span>Base Settings</span>
                {hasChanges && (
                  <span className="px-1.5 py-0.5 text-[10px] bg-yellow-500/20 text-yellow-400 rounded">
                    Unsaved
                  </span>
                )}
              </div>
              {isSettingsExpanded ? (
                <ChevronUp className="w-4 h-4" />
              ) : (
                <ChevronDown className="w-4 h-4" />
              )}
            </button>

            {isSettingsExpanded && (
              <div className="mt-2 p-3 bg-gray-700/30 rounded-lg">
                <BaseSettingInput
                  label="Timeframe"
                  value={localSettings.timeframe}
                  icon={<Clock className="w-4 h-4" />}
                  onChange={(v) => handleSettingChange('timeframe', v)}
                  type="select"
                  options={timeframeOptions}
                  disabled={isDisabled}
                />
                <BaseSettingInput
                  label="Position Size"
                  value={localSettings.position_size_percent}
                  unit="%"
                  icon={<DollarSign className="w-4 h-4" />}
                  onChange={(v) => handleSettingChange('position_size_percent', v)}
                  min={0.1}
                  max={100}
                  step={0.1}
                  disabled={isDisabled}
                />
                <BaseSettingInput
                  label="Max Leverage"
                  value={localSettings.max_leverage}
                  unit="x"
                  icon={<TrendingUp className="w-4 h-4" />}
                  onChange={(v) => handleSettingChange('max_leverage', v)}
                  min={1}
                  max={125}
                  step={1}
                  disabled={isDisabled}
                />
                <BaseSettingInput
                  label="Max Positions"
                  value={localSettings.max_positions}
                  icon={<Layers className="w-4 h-4" />}
                  onChange={(v) => handleSettingChange('max_positions', v)}
                  min={1}
                  max={50}
                  step={1}
                  disabled={isDisabled}
                />
                <BaseSettingInput
                  label="Min Volume"
                  value={localSettings.min_volume_usdt}
                  unit="USDT"
                  icon={<BarChart3 className="w-4 h-4" />}
                  onChange={(v) => handleSettingChange('min_volume_usdt', v)}
                  min={0}
                  max={100000000}
                  step={1000}
                  disabled={isDisabled}
                />

                {/* Save/Reset buttons */}
                {hasChanges && (
                  <div className="flex items-center gap-2 mt-4 pt-3 border-t border-gray-600">
                    <button
                      onClick={handleSaveSettings}
                      disabled={isSaving || isDisabled}
                      className="flex items-center gap-2 px-3 py-1.5 text-sm bg-purple-600 hover:bg-purple-500 text-white rounded transition-colors disabled:opacity-50"
                    >
                      {isSaving ? (
                        <Loader2 className="w-4 h-4 animate-spin" />
                      ) : (
                        <CheckCircle2 className="w-4 h-4" />
                      )}
                      Save
                    </button>
                    <button
                      onClick={handleResetSettings}
                      disabled={isSaving || isDisabled}
                      className="flex items-center gap-2 px-3 py-1.5 text-sm text-gray-300 hover:text-white hover:bg-gray-600 rounded transition-colors disabled:opacity-50"
                    >
                      <RefreshCw className="w-4 h-4" />
                      Reset
                    </button>
                  </div>
                )}

                {/* Error message */}
                {saveError && (
                  <div className="flex items-center gap-2 mt-2 p-2 bg-red-500/10 border border-red-500/30 rounded text-sm text-red-400">
                    <AlertTriangle className="w-4 h-4" />
                    {saveError}
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Sub-Strategies List */}
          <div>
            <div className="flex items-center gap-2 mb-3">
              <Layers className="w-4 h-4 text-gray-400" />
              <span className="text-sm font-medium text-gray-400">Sub-Strategies</span>
              <span className="px-1.5 py-0.5 text-[10px] bg-gray-600 text-gray-300 rounded">
                {group.sub_strategies.filter((s) => s.enabled).length}/{group.sub_strategies.length} active
              </span>
            </div>

            <div className="space-y-2">
              {group.sub_strategies.map((subStrategy) => (
                <SubStrategyRow
                  key={subStrategy.id}
                  subStrategy={subStrategy}
                  onToggle={(enabled) => onToggleSubStrategy(subStrategy.id, enabled)}
                  onOpenSettings={() => onOpenSubStrategySettings(subStrategy)}
                  disabled={isDisabled || !group.enabled}
                />
              ))}

              {group.sub_strategies.length === 0 && (
                <div className="p-4 text-center text-sm text-gray-500 bg-gray-700/20 rounded-lg">
                  No sub-strategies available for this group
                </div>
              )}
            </div>
          </div>

          {/* Supported Regimes */}
          {group.supported_regimes.length > 0 && (
            <div className="mt-4 pt-4 border-t border-gray-700">
              <span className="text-xs text-gray-500">Supported Regimes: </span>
              <div className="flex flex-wrap gap-1 mt-1">
                {group.supported_regimes.map((regime) => (
                  <span
                    key={regime}
                    className="px-2 py-0.5 text-[10px] bg-blue-500/10 text-blue-400 border border-blue-500/30 rounded"
                  >
                    {regime}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
