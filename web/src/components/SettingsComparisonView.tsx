import { useState, useEffect, useMemo, useCallback } from 'react';
import {
  CheckCircle2,
  XCircle,
  ChevronDown,
  ChevronUp,
  AlertTriangle,
  Loader2,
  RefreshCw,
  Info,
  Save,
  Settings,
  Shield,
  Brain,
  Wallet,
  Target,
  FileText,
} from 'lucide-react';
import {
  loadModeDefaults,
  loadCircuitBreakerDefaults,
  loadLLMConfigDefaults,
  loadCapitalAllocationDefaults,
  ConfigResetPreview,
  FieldComparison,
} from '../services/futuresApi';

// ==================== INTERFACES ====================

interface SettingGroupComparison {
  groupName: string;
  groupKey: string;
  allMatch: boolean;
  totalFields: number;
  matchingFields: number;
  differentFields: number;
  fields: FieldComparison[];
}

interface ModeComparisonResult {
  mode: string;
  modeName: string;
  allMatch: boolean;
  totalChanges: number;
  totalFields: number;
  groups: SettingGroupComparison[];
  isAdmin?: boolean;
  configNotFound?: boolean;
  rawData?: any;
}

interface OtherSettingComparison {
  settingType: string;
  settingName: string;
  icon: React.ReactNode;
  allMatch: boolean;
  totalChanges: number;
  totalFields: number;
  fields: FieldComparison[];
  isAdmin?: boolean;
  configNotFound?: boolean;
  rawData?: any;
}

interface SettingsComparisonViewProps {
  modes?: string[];
  isAdmin?: boolean;
  // Mode resets
  onResetAllModes?: () => void;
  onResetMode?: (mode: string) => void;
  onResetModeGroup?: (mode: string, group: string) => void;
  // Other settings resets
  onResetAllOther?: () => void;
  onResetCircuitBreaker?: () => void;
  onResetLLMConfig?: () => void;
  onResetCapitalAllocation?: () => void;
  // Admin save handlers
  onSaveMode?: (mode: string, data: any) => void;
  onSaveOtherSetting?: (settingType: string, data: any) => void;
}

// ==================== CONSTANTS ====================

// Define ALL setting groups for each mode configuration
const SETTING_GROUPS: Record<string, { name: string; prefixes: string[]; description: string }> = {
  enabled: {
    name: 'Mode Status',
    prefixes: ['enabled'],
    description: 'Whether this trading mode is enabled',
  },
  timeframe: {
    name: 'Timeframe Settings',
    prefixes: ['timeframe.'],
    description: 'Chart timeframes for trend, entry, and analysis',
  },
  confidence: {
    name: 'Confidence Settings',
    prefixes: ['confidence.'],
    description: 'Minimum, high, and ultra confidence thresholds',
  },
  size: {
    name: 'Size Settings',
    prefixes: ['size.'],
    description: 'Position sizing, leverage, and risk multipliers',
  },
  sltp: {
    name: 'SL/TP Settings',
    prefixes: ['sltp.'],
    description: 'Stop loss, take profit, and trailing stop configuration',
  },
  risk: {
    name: 'Risk Settings',
    prefixes: ['risk.'],
    description: 'Risk level, drawdown limits, and ADX thresholds',
  },
  circuit_breaker: {
    name: 'Circuit Breaker',
    prefixes: ['circuit_breaker.'],
    description: 'Mode-specific loss limits and cooldowns',
  },
  hedge: {
    name: 'Hedge Settings',
    prefixes: ['hedge.'],
    description: 'Hedge mode configuration',
  },
  averaging: {
    name: 'Position Averaging',
    prefixes: ['averaging.'],
    description: 'Average up/down entry rules',
  },
  stale_release: {
    name: 'Stale Position Release',
    prefixes: ['stale_release.'],
    description: 'Auto-close stale positions configuration',
  },
  assignment: {
    name: 'Mode Assignment',
    prefixes: ['assignment.'],
    description: 'Volatility, confidence, and profit potential rules',
  },
  mtf: {
    name: 'Multi-Timeframe (MTF)',
    prefixes: ['mtf.'],
    description: 'Multi-timeframe analysis configuration',
  },
  dynamic_ai_exit: {
    name: 'Dynamic AI Exit',
    prefixes: ['dynamic_ai_exit.'],
    description: 'LLM-based exit decision configuration',
  },
  reversal: {
    name: 'Reversal Entry',
    prefixes: ['reversal.'],
    description: 'MTF reversal pattern detection',
  },
  funding_rate: {
    name: 'Funding Rate',
    prefixes: ['funding_rate.'],
    description: 'Funding rate thresholds and blocking rules',
  },
  trend_divergence: {
    name: 'Trend Divergence',
    prefixes: ['trend_divergence.'],
    description: 'Multi-timeframe trend alignment checks',
  },
  position_optimization: {
    name: 'Position Optimization',
    prefixes: ['position_optimization.'],
    description: 'Progressive TP, DCA, re-entry settings',
  },
  trend_filters: {
    name: 'Trend Filters',
    prefixes: ['trend_filters.'],
    description: 'BTC trend, EMA, VWAP, candlestick alignment',
  },
  early_warning: {
    name: 'Early Warning',
    prefixes: ['early_warning.'],
    description: 'AI-based early exit monitoring',
  },
  entry: {
    name: 'Entry Settings',
    prefixes: ['entry.'],
    description: 'Limit order gap and market entry settings',
  },
  other: {
    name: 'Other Settings',
    prefixes: [],
    description: 'Miscellaneous settings',
  },
};

const MODE_DISPLAY_NAMES: Record<string, string> = {
  ultra_fast: 'Ultra Fast',
  scalp: 'Scalp',
  swing: 'Swing',
  position: 'Position',
};

// ==================== UTILITY FUNCTIONS ====================

// Group ALL fields (both matching and different) by category
function groupAllFieldsByCategory(allFields: FieldComparison[]): SettingGroupComparison[] {
  const groupedResults: SettingGroupComparison[] = [];
  const usedPaths = new Set<string>();

  // Process each known group
  Object.entries(SETTING_GROUPS).forEach(([groupKey, groupConfig]) => {
    if (groupKey === 'other') return; // Handle 'other' at the end

    const groupFields: FieldComparison[] = [];

    allFields.forEach((field) => {
      const matchesGroup = groupConfig.prefixes.some(
        (prefix) => field.path.startsWith(prefix) || field.path === groupKey || field.path === prefix.replace('.', '')
      );
      if (matchesGroup && !usedPaths.has(field.path)) {
        groupFields.push(field);
        usedPaths.add(field.path);
      }
    });

    if (groupFields.length > 0) {
      const matchingCount = groupFields.filter((f) => f.match).length;
      const differentCount = groupFields.filter((f) => !f.match).length;

      groupedResults.push({
        groupName: groupConfig.name,
        groupKey,
        allMatch: differentCount === 0,
        totalFields: groupFields.length,
        matchingFields: matchingCount,
        differentFields: differentCount,
        fields: groupFields,
      });
    }
  });

  // Handle remaining fields as "Other"
  const otherFields = allFields.filter((f) => !usedPaths.has(f.path));
  if (otherFields.length > 0) {
    const matchingCount = otherFields.filter((f) => f.match).length;
    const differentCount = otherFields.filter((f) => !f.match).length;

    groupedResults.push({
      groupName: 'Other Settings',
      groupKey: 'other',
      allMatch: differentCount === 0,
      totalFields: otherFields.length,
      matchingFields: matchingCount,
      differentFields: differentCount,
      fields: otherFields,
    });
  }

  return groupedResults;
}

const formatValue = (value: any, isNotConfigured = false): string => {
  if (value === null || value === undefined) {
    return isNotConfigured ? 'Not configured' : 'N/A';
  }
  if (typeof value === 'boolean') return value ? 'Yes' : 'No';
  if (typeof value === 'number') {
    // Format numbers nicely
    if (Number.isInteger(value)) return value.toLocaleString();
    return value.toFixed(4).replace(/\.?0+$/, '');
  }
  if (Array.isArray(value)) return value.length > 0 ? value.join(', ') : '[]';
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
};

// Check if a field is "not configured" (exists in defaults but not in user DB)
const isFieldNotConfigured = (field: FieldComparison): boolean => {
  return field.current === null && field.default !== null && field.default !== undefined;
};

// ==================== SUB-COMPONENTS ====================

// Risk Badge Component
function RiskBadge({ risk }: { risk?: 'high' | 'medium' | 'low' }) {
  if (!risk) return null;
  const colors = {
    high: 'bg-red-500/20 text-red-400 border-red-500/30',
    medium: 'bg-orange-500/20 text-orange-400 border-orange-500/30',
    low: 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30',
  };
  return (
    <span className={`px-1.5 py-0.5 text-xs rounded border ${colors[risk] || colors.medium}`}>
      {risk}
    </span>
  );
}

// Reset Button Component
function ResetButton({
  onClick,
  label,
  size = 'small',
  disabled = false,
}: {
  onClick: () => void;
  label: string;
  size?: 'small' | 'medium';
  disabled?: boolean;
}) {
  return (
    <button
      onClick={(e) => {
        e.stopPropagation();
        onClick();
      }}
      disabled={disabled}
      className={`flex items-center gap-1 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white rounded transition-colors ${
        size === 'small' ? 'px-2 py-1 text-xs' : 'px-3 py-1.5 text-sm'
      }`}
      title={label}
    >
      <RefreshCw className={size === 'small' ? 'w-3 h-3' : 'w-4 h-4'} />
      {label}
    </button>
  );
}

// Save Button Component (Admin only)
function SaveButton({
  onClick,
  label,
  disabled = false,
}: {
  onClick: () => void;
  label: string;
  disabled?: boolean;
}) {
  return (
    <button
      onClick={(e) => {
        e.stopPropagation();
        onClick();
      }}
      disabled={disabled}
      className="flex items-center gap-1 px-3 py-1.5 bg-green-600 hover:bg-green-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white text-sm rounded transition-colors"
      title={label}
    >
      <Save className="w-4 h-4" />
      {label}
    </button>
  );
}

// Collapsible Section Header
function SectionHeader({
  title,
  icon,
  isExpanded,
  onToggle,
  allMatch,
  totalItems,
  matchingItems,
  resetButton,
  children,
}: {
  title: string;
  icon: React.ReactNode;
  isExpanded: boolean;
  onToggle: () => void;
  allMatch: boolean;
  totalItems: number;
  matchingItems: number;
  resetButton?: React.ReactNode;
  children?: React.ReactNode;
}) {
  return (
    <div
      className={`rounded-lg border overflow-hidden ${
        allMatch ? 'bg-green-900/20 border-green-500/30' : 'bg-orange-900/20 border-orange-500/30'
      }`}
    >
      <button
        onClick={onToggle}
        className={`w-full p-4 flex items-center justify-between transition-colors ${
          allMatch ? 'hover:bg-green-900/30' : 'hover:bg-orange-900/30'
        }`}
      >
        <div className="flex items-center gap-3">
          {icon}
          <div className="text-left">
            <h3 className="text-lg font-semibold text-white">{title}</h3>
            <p className="text-sm text-gray-400">
              {matchingItems}/{totalItems} items match defaults
            </p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          {resetButton}
          <span
            className={`px-3 py-1 text-sm rounded-full font-medium ${
              allMatch ? 'bg-green-500/30 text-green-300' : 'bg-orange-500/30 text-orange-300'
            }`}
          >
            {allMatch ? 'All Match' : `${totalItems - matchingItems} Differences`}
          </span>
          {isExpanded ? (
            <ChevronUp className="w-5 h-5 text-gray-400" />
          ) : (
            <ChevronDown className="w-5 h-5 text-gray-400" />
          )}
        </div>
      </button>
      {isExpanded && <div className="border-t border-gray-700/50">{children}</div>}
    </div>
  );
}

// Known dropdown options for specific fields
const DROPDOWN_OPTIONS: Record<string, string[]> = {
  margin_type: ['ISOLATED', 'CROSS'],
  risk_level: ['conservative', 'moderate', 'aggressive'],
  volatility_min: ['low', 'medium', 'high'],
  volatility_max: ['low', 'medium', 'high'],
  trend_timeframe: ['1m', '5m', '15m', '30m', '1h', '4h', '1d'],
  entry_timeframe: ['1m', '5m', '15m', '30m', '1h', '4h'],
  analysis_timeframe: ['15m', '30m', '1h', '4h', '1d'],
  primary_timeframe: ['5m', '15m', '30m', '1h', '4h', '1d'],
  secondary_timeframe: ['5m', '15m', '30m', '1h', '4h'],
  tertiary_timeframe: ['1m', '5m', '15m', '30m', '1h'],
  stale_zone_close_action: ['close', 'hold', 'reduce'],
  trailing_activation_mode: ['percent', 'atr', 'price'],
};

// Detect field type from value and field name
function getFieldType(fieldName: string, value: any): 'boolean' | 'dropdown' | 'number' | 'text' {
  // Boolean fields
  if (typeof value === 'boolean') return 'boolean';

  // Check if field has dropdown options
  if (DROPDOWN_OPTIONS[fieldName]) return 'dropdown';

  // Check field name patterns for booleans
  if (fieldName.startsWith('enabled') ||
      fieldName.startsWith('use_') ||
      fieldName.startsWith('allow_') ||
      fieldName.startsWith('requires_') ||
      fieldName.endsWith('_enabled') ||
      fieldName.endsWith('_check') ||
      fieldName === 'block_on_divergence' ||
      fieldName === 'block_on_disagreement' ||
      fieldName === 'use_market_entry' ||
      fieldName === 'auto_size_enabled' ||
      fieldName === 'trailing_stop_enabled' ||
      fieldName === 'use_single_tp' ||
      fieldName === 'use_roi_based_sltp' ||
      fieldName === 'staged_entry_enabled' ||
      fieldName === 'use_llm_for_averaging' ||
      fieldName === 'use_llm_for_loss' ||
      fieldName === 'use_llm_for_profit' ||
      fieldName === 'trend_stability_check' ||
      fieldName === 'mover_gainers' ||
      fieldName === 'mover_losers' ||
      fieldName === 'only_underwater') {
    return 'boolean';
  }

  // Number fields
  if (typeof value === 'number') return 'number';

  return 'text';
}

// Admin Input Component - renders appropriate input based on field type
function AdminInput({
  fieldName,
  value,
  onChange,
  isEdited,
}: {
  fieldName: string;
  value: any;
  onChange: (value: any) => void;
  isEdited: boolean;
}) {
  const fieldType = getFieldType(fieldName, value);

  // Boolean - Checkbox/Toggle
  if (fieldType === 'boolean') {
    const boolValue = typeof value === 'boolean' ? value : value === 'true' || value === 'Yes';
    return (
      <div className="flex items-center gap-2">
        <label className="relative inline-flex items-center cursor-pointer">
          <input
            type="checkbox"
            checked={boolValue}
            onChange={(e) => onChange(e.target.checked)}
            className="sr-only peer"
          />
          <div className="w-9 h-5 bg-gray-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-green-600"></div>
          <span className={`ml-2 text-xs ${boolValue ? 'text-green-400' : 'text-gray-400'}`}>
            {boolValue ? 'Yes' : 'No'}
          </span>
        </label>
        {isEdited && <span className="text-xs text-orange-400">*</span>}
      </div>
    );
  }

  // Dropdown
  if (fieldType === 'dropdown' && DROPDOWN_OPTIONS[fieldName]) {
    return (
      <div className="flex items-center gap-2">
        <select
          value={String(value)}
          onChange={(e) => onChange(e.target.value)}
          className="px-2 py-1 bg-gray-800 border border-gray-600 rounded text-white text-xs font-mono focus:border-blue-500 focus:outline-none"
        >
          {DROPDOWN_OPTIONS[fieldName].map((opt) => (
            <option key={opt} value={opt}>
              {opt}
            </option>
          ))}
        </select>
        {isEdited && <span className="text-xs text-orange-400">*</span>}
      </div>
    );
  }

  // Number
  if (fieldType === 'number') {
    return (
      <div className="flex items-center gap-2">
        <input
          type="number"
          step="any"
          value={value}
          onChange={(e) => onChange(parseFloat(e.target.value) || 0)}
          className="w-32 px-2 py-1 bg-gray-800 border border-gray-600 rounded text-white text-xs font-mono focus:border-blue-500 focus:outline-none"
        />
        {isEdited && <span className="text-xs text-orange-400">*</span>}
      </div>
    );
  }

  // Text (default)
  return (
    <div className="flex items-center gap-2">
      <input
        type="text"
        value={String(value ?? '')}
        onChange={(e) => onChange(e.target.value)}
        className="w-full px-2 py-1 bg-gray-800 border border-gray-600 rounded text-white text-xs font-mono focus:border-blue-500 focus:outline-none"
      />
      {isEdited && <span className="text-xs text-orange-400">*</span>}
    </div>
  );
}

// Field Table for displaying comparison data
function FieldTable({
  fields,
  isAdmin,
  onFieldChange,
  editedValues,
}: {
  fields: FieldComparison[];
  isAdmin: boolean;
  onFieldChange?: (path: string, value: any) => void;
  editedValues?: Record<string, any>;
}) {
  return (
    <div className="bg-gray-900/30">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-gray-700 text-gray-400">
            <th className="text-left p-2 pl-4 w-1/4">Setting</th>
            {isAdmin ? (
              <th className="text-left p-2 w-1/2">Value</th>
            ) : (
              <>
                <th className="text-left p-2 w-1/4">Your Value</th>
                <th className="text-left p-2 w-1/4">Default</th>
              </>
            )}
            <th className="text-left p-2 pr-4 w-1/4">Status</th>
          </tr>
        </thead>
        <tbody>
          {fields.map((field, idx) => {
            const isEdited = editedValues && editedValues[field.path] !== undefined;
            const displayValue = isEdited ? editedValues[field.path] : field.current;
            const fieldName = field.path.split('.').pop() || '';
            const notConfigured = isFieldNotConfigured(field);

            return (
              <tr
                key={idx}
                className={`border-b border-gray-700/30 ${
                  notConfigured
                    ? 'bg-red-900/10'
                    : field.match
                    ? 'bg-green-900/5'
                    : 'bg-orange-900/10'
                }`}
              >
                <td className="p-2 pl-4 font-mono text-white text-xs">
                  {fieldName}
                </td>
                {isAdmin ? (
                  <td className="p-2">
                    <AdminInput
                      fieldName={fieldName}
                      value={displayValue}
                      onChange={(newValue) => onFieldChange?.(field.path, newValue)}
                      isEdited={isEdited}
                    />
                  </td>
                ) : (
                  <>
                    <td
                      className={`p-2 font-mono text-xs ${
                        notConfigured
                          ? 'text-red-400 italic'
                          : field.match
                          ? 'text-green-400'
                          : 'text-orange-400'
                      }`}
                    >
                      {notConfigured ? 'Not in database' : formatValue(field.current)}
                    </td>
                    <td className="p-2 font-mono text-xs text-blue-400">
                      {formatValue(field.default)}
                    </td>
                  </>
                )}
                <td className="p-2 pr-4">
                  {isAdmin ? (
                    isEdited ? (
                      <span className="flex items-center gap-1 text-orange-400 text-xs">
                        <AlertTriangle className="w-3 h-3" />
                        Modified
                      </span>
                    ) : (
                      <span className="flex items-center gap-1 text-gray-400 text-xs">
                        <CheckCircle2 className="w-3 h-3" />
                        Default
                      </span>
                    )
                  ) : notConfigured ? (
                    <span className="flex items-center gap-1 text-red-400 text-xs">
                      <AlertTriangle className="w-3 h-3" />
                      Not configured
                    </span>
                  ) : field.match ? (
                    <span className="flex items-center gap-1 text-green-400 text-xs">
                      <CheckCircle2 className="w-3 h-3" />
                      Match
                    </span>
                  ) : (
                    <div className="flex items-center gap-2">
                      <span className="flex items-center gap-1 text-orange-400 text-xs">
                        <XCircle className="w-3 h-3" />
                        Different
                      </span>
                      <RiskBadge risk={field.risk_level} />
                    </div>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

// Mode Card Component
function ModeCard({
  comparison,
  isAdmin,
  isExpanded,
  expandedGroups,
  onToggleExpand,
  onToggleGroup,
  onResetMode,
  onResetGroup,
  onSaveMode,
  editedValues,
  onFieldChange,
}: {
  comparison: ModeComparisonResult;
  isAdmin: boolean;
  isExpanded: boolean;
  expandedGroups: Set<string>;
  onToggleExpand: () => void;
  onToggleGroup: (key: string) => void;
  onResetMode?: () => void;
  onResetGroup?: (group: string) => void;
  onSaveMode?: () => void;
  editedValues?: Record<string, any>;
  onFieldChange?: (path: string, value: any) => void;
}) {
  const hasEdits = editedValues && Object.keys(editedValues).length > 0;

  return (
    <div
      className={`rounded-lg border overflow-hidden ${
        comparison.configNotFound
          ? 'bg-gray-800 border-gray-700'
          : comparison.allMatch
          ? 'bg-green-900/20 border-green-500/30'
          : 'bg-orange-900/20 border-orange-500/30'
      }`}
    >
      {/* Mode Header */}
      <button
        onClick={onToggleExpand}
        className={`w-full p-4 flex items-center justify-between transition-colors ${
          comparison.allMatch ? 'hover:bg-green-900/30' : 'hover:bg-orange-900/30'
        }`}
      >
        <div className="flex items-center gap-3">
          {comparison.configNotFound ? (
            <AlertTriangle className="w-6 h-6 text-gray-400" />
          ) : comparison.isAdmin ? (
            <Info className="w-6 h-6 text-purple-400" />
          ) : comparison.allMatch ? (
            <CheckCircle2 className="w-6 h-6 text-green-400" />
          ) : (
            <XCircle className="w-6 h-6 text-orange-400" />
          )}
          <div className="text-left">
            <h4 className="text-lg font-semibold text-white">Mode: {comparison.modeName}</h4>
            <p className="text-sm text-gray-400">
              {comparison.configNotFound
                ? 'Not configured in database'
                : comparison.isAdmin
                ? isAdmin
                  ? 'Editing default values'
                  : 'Admin - showing defaults'
                : comparison.allMatch
                ? `All ${comparison.totalFields} settings match defaults`
                : `${comparison.totalChanges} of ${comparison.totalFields} settings differ from defaults`}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-3">
          {/* Reset/Save buttons */}
          {isAdmin && onSaveMode && hasEdits && (
            <SaveButton onClick={onSaveMode} label="Save" />
          )}
          {onResetMode && !comparison.allMatch && !comparison.configNotFound && (
            <ResetButton onClick={onResetMode} label="Reset Mode" size="medium" />
          )}
          {!comparison.configNotFound && !comparison.isAdmin && (
            <span
              className={`px-3 py-1 text-sm rounded-full font-medium ${
                comparison.allMatch ? 'bg-green-500/30 text-green-300' : 'bg-orange-500/30 text-orange-300'
              }`}
            >
              {comparison.allMatch ? 'Up to Date' : `${comparison.totalChanges} Changes`}
            </span>
          )}
          {isExpanded ? (
            <ChevronUp className="w-5 h-5 text-gray-400" />
          ) : (
            <ChevronDown className="w-5 h-5 text-gray-400" />
          )}
        </div>
      </button>

      {/* Expanded Content */}
      {isExpanded && (
        <div className="border-t border-gray-700/50">
          {comparison.configNotFound ? (
            <div className="p-6 text-center">
              <AlertTriangle className="w-12 h-12 text-gray-500 mx-auto mb-3" />
              <p className="text-gray-400 mb-4">This mode has not been configured in your database.</p>
              {onResetMode && (
                <button
                  onClick={onResetMode}
                  className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
                >
                  Apply Default Configuration
                </button>
              )}
            </div>
          ) : comparison.groups.length === 0 ? (
            <div className="p-6 text-center">
              <Info className="w-12 h-12 text-purple-400 mx-auto mb-3" />
              <p className="text-gray-400">No settings data available.</p>
            </div>
          ) : (
            <div className="p-4 space-y-2">
              {/* Render ALL groups */}
              {comparison.groups.map((group) => {
                const groupExpandKey = `${comparison.mode}-${group.groupKey}`;
                const isGroupExpanded = expandedGroups.has(groupExpandKey);

                return (
                  <div
                    key={group.groupKey}
                    className={`border rounded-lg overflow-hidden ${
                      group.allMatch
                        ? 'border-green-500/30 bg-green-900/10'
                        : 'border-orange-500/30 bg-orange-900/10'
                    }`}
                  >
                    {/* Group Header */}
                    <button
                      onClick={() => onToggleGroup(groupExpandKey)}
                      className={`w-full p-3 flex items-center justify-between transition-colors ${
                        group.allMatch ? 'hover:bg-green-900/20' : 'hover:bg-orange-900/20'
                      }`}
                    >
                      <div className="flex items-center gap-3">
                        {group.allMatch ? (
                          <CheckCircle2 className="w-5 h-5 text-green-400" />
                        ) : (
                          <XCircle className="w-5 h-5 text-orange-400" />
                        )}
                        <span className={`font-medium ${group.allMatch ? 'text-green-300' : 'text-orange-300'}`}>
                          {group.groupName}
                        </span>
                        <span className="text-gray-500 text-sm">
                          ({group.matchingFields}/{group.totalFields} match)
                        </span>
                      </div>

                      <div className="flex items-center gap-2">
                        {onResetGroup && (
                          <ResetButton
                            onClick={() => onResetGroup(group.groupKey)}
                            label="Reset Group"
                            size="small"
                          />
                        )}
                        {group.allMatch ? (
                          <span className="px-2 py-0.5 text-xs bg-green-500/20 text-green-400 rounded">
                            All Match
                          </span>
                        ) : (
                          <span className="px-2 py-0.5 text-xs bg-orange-500/20 text-orange-400 rounded">
                            {group.differentFields} difference{group.differentFields !== 1 ? 's' : ''}
                          </span>
                        )}
                        {isGroupExpanded ? (
                          <ChevronUp className="w-4 h-4 text-gray-400" />
                        ) : (
                          <ChevronDown className="w-4 h-4 text-gray-400" />
                        )}
                      </div>
                    </button>

                    {/* Group Fields Table */}
                    {isGroupExpanded && (
                      <div className="border-t border-gray-700/50">
                        <FieldTable
                          fields={group.fields}
                          isAdmin={isAdmin}
                          onFieldChange={onFieldChange}
                          editedValues={editedValues}
                        />
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// Other Setting Card Component
function OtherSettingCard({
  setting,
  isAdmin,
  isExpanded,
  onToggleExpand,
  onReset,
  onSave,
  editedValues,
  onFieldChange,
}: {
  setting: OtherSettingComparison;
  isAdmin: boolean;
  isExpanded: boolean;
  onToggleExpand: () => void;
  onReset?: () => void;
  onSave?: () => void;
  editedValues?: Record<string, any>;
  onFieldChange?: (path: string, value: any) => void;
}) {
  const hasEdits = editedValues && Object.keys(editedValues).length > 0;

  return (
    <div
      className={`rounded-lg border overflow-hidden ${
        setting.configNotFound
          ? 'bg-gray-800 border-gray-700'
          : setting.allMatch
          ? 'bg-green-900/20 border-green-500/30'
          : 'bg-orange-900/20 border-orange-500/30'
      }`}
    >
      {/* Card Header */}
      <button
        onClick={onToggleExpand}
        className={`w-full p-4 flex items-center justify-between transition-colors ${
          setting.allMatch ? 'hover:bg-green-900/30' : 'hover:bg-orange-900/30'
        }`}
      >
        <div className="flex items-center gap-3">
          {setting.icon}
          <div className="text-left">
            <h4 className="text-lg font-semibold text-white">{setting.settingName}</h4>
            <p className="text-sm text-gray-400">
              {setting.configNotFound
                ? 'Not configured'
                : setting.allMatch
                ? `All ${setting.totalFields} settings match defaults`
                : `${setting.totalChanges} of ${setting.totalFields} settings differ`}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-3">
          {isAdmin && onSave && hasEdits && <SaveButton onClick={onSave} label="Save" />}
          {onReset && !setting.allMatch && !setting.configNotFound && (
            <ResetButton onClick={onReset} label="Reset" size="medium" />
          )}
          {!setting.configNotFound && (
            <span
              className={`px-3 py-1 text-sm rounded-full font-medium ${
                setting.allMatch ? 'bg-green-500/30 text-green-300' : 'bg-orange-500/30 text-orange-300'
              }`}
            >
              {setting.allMatch ? 'Up to Date' : `${setting.totalChanges} Changes`}
            </span>
          )}
          {isExpanded ? (
            <ChevronUp className="w-5 h-5 text-gray-400" />
          ) : (
            <ChevronDown className="w-5 h-5 text-gray-400" />
          )}
        </div>
      </button>

      {/* Expanded Content */}
      {isExpanded && (
        <div className="border-t border-gray-700/50">
          {setting.configNotFound ? (
            <div className="p-6 text-center">
              <AlertTriangle className="w-12 h-12 text-gray-500 mx-auto mb-3" />
              <p className="text-gray-400 mb-4">This setting has not been configured.</p>
              {onReset && (
                <button
                  onClick={onReset}
                  className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
                >
                  Apply Default Configuration
                </button>
              )}
            </div>
          ) : setting.fields.length === 0 ? (
            <div className="p-6 text-center">
              <Info className="w-12 h-12 text-purple-400 mx-auto mb-3" />
              <p className="text-gray-400">No settings data available.</p>
            </div>
          ) : (
            <div className="p-4">
              <FieldTable
                fields={setting.fields}
                isAdmin={isAdmin}
                onFieldChange={onFieldChange}
                editedValues={editedValues}
              />
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// Read-Only Info Card
function ReadOnlyCard({
  title,
  icon,
  data,
  isExpanded,
  onToggleExpand,
}: {
  title: string;
  icon: React.ReactNode;
  data: Record<string, any>;
  isExpanded: boolean;
  onToggleExpand: () => void;
}) {
  return (
    <div className="rounded-lg border bg-gray-800 border-gray-700 overflow-hidden">
      <button
        onClick={onToggleExpand}
        className="w-full p-4 flex items-center justify-between hover:bg-gray-700/50 transition-colors"
      >
        <div className="flex items-center gap-3">
          {icon}
          <div className="text-left">
            <h4 className="text-lg font-semibold text-white">{title}</h4>
            <p className="text-sm text-gray-400">Read-only information</p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <span className="px-3 py-1 text-sm rounded-full font-medium bg-gray-600/30 text-gray-300">
            Info Only
          </span>
          {isExpanded ? (
            <ChevronUp className="w-5 h-5 text-gray-400" />
          ) : (
            <ChevronDown className="w-5 h-5 text-gray-400" />
          )}
        </div>
      </button>

      {isExpanded && (
        <div className="border-t border-gray-700/50 p-4">
          <div className="bg-gray-900/50 rounded-lg p-4">
            <table className="w-full text-sm">
              <tbody>
                {Object.entries(data).map(([key, value]) => (
                  <tr key={key} className="border-b border-gray-700/30 last:border-0">
                    <td className="py-2 font-medium text-gray-400">{key}</td>
                    <td className="py-2 font-mono text-white">{formatValue(value)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

// ==================== MAIN COMPONENT ====================

export default function SettingsComparisonView({
  modes = ['ultra_fast', 'scalp', 'swing', 'position'],
  isAdmin = false,
  // Mode resets
  onResetAllModes,
  onResetMode,
  onResetModeGroup,
  // Other settings resets
  onResetAllOther,
  onResetCircuitBreaker,
  onResetLLMConfig,
  onResetCapitalAllocation,
  // Admin save handlers
  onSaveMode,
  onSaveOtherSetting,
}: SettingsComparisonViewProps) {
  // ==================== STATE ====================
  const [modeComparisons, setModeComparisons] = useState<ModeComparisonResult[]>([]);
  const [otherSettings, setOtherSettings] = useState<OtherSettingComparison[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Expansion states
  const [expandedSections, setExpandedSections] = useState<Set<string>>(new Set(['modes']));
  const [expandedModes, setExpandedModes] = useState<Set<string>>(new Set());
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set());
  const [expandedOtherSettings, setExpandedOtherSettings] = useState<Set<string>>(new Set());
  const [expandedReadOnly, setExpandedReadOnly] = useState<Set<string>>(new Set());

  // Edited values state (for admin mode)
  const [editedModeValues, setEditedModeValues] = useState<Record<string, Record<string, any>>>({});
  const [editedOtherValues, setEditedOtherValues] = useState<Record<string, Record<string, any>>>({});

  // Original values state (to track real changes)
  const [originalModeValues, setOriginalModeValues] = useState<Record<string, Record<string, any>>>({});
  const [originalOtherValues, setOriginalOtherValues] = useState<Record<string, Record<string, any>>>({});

  // Read-only metadata (placeholder - would come from API)
  const [metadata, setMetadata] = useState<Record<string, any>>({
    version: '1.0.0',
    schema_version: '2.0',
    last_updated: new Date().toISOString(),
  });

  // ==================== DATA LOADING ====================

  const loadModeComparisons = useCallback(async () => {
    const results: ModeComparisonResult[] = [];

    for (const mode of modes) {
      try {
        const preview = (await loadModeDefaults(mode, true)) as ConfigResetPreview;

        if (preview.is_admin) {
          // Admin view - convert default_value to fields for editing
          const allFields: FieldComparison[] = [];
          if (preview.default_value) {
            const flattenObject = (obj: any, prefix = ''): void => {
              for (const [key, value] of Object.entries(obj)) {
                const path = prefix ? `${prefix}.${key}` : key;
                if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
                  flattenObject(value, path);
                } else {
                  allFields.push({
                    path,
                    current: value,
                    default: value,
                    match: true,
                  });
                }
              }
            };
            flattenObject(preview.default_value);
          }

          const groups = groupAllFieldsByCategory(allFields);

          results.push({
            mode,
            modeName: MODE_DISPLAY_NAMES[mode] || mode,
            allMatch: true,
            totalChanges: 0,
            totalFields: allFields.length,
            groups,
            isAdmin: true,
            rawData: preview.default_value,
          });
        } else {
          // User view - use all_values if available
          const allFields =
            preview.all_values ||
            preview.differences.map((d) => ({
              path: d.path,
              current: d.current,
              default: d.default,
              match: false,
              risk_level: d.risk_level,
            }));

          const groups = groupAllFieldsByCategory(allFields);
          const totalFields = allFields.length;

          results.push({
            mode,
            modeName: MODE_DISPLAY_NAMES[mode] || mode,
            allMatch: preview.all_match,
            totalChanges: preview.total_changes,
            totalFields,
            groups,
            rawData: preview,
          });
        }
      } catch (err: any) {
        // Handle mode not configured
        if (err?.response?.status === 404) {
          results.push({
            mode,
            modeName: MODE_DISPLAY_NAMES[mode] || mode,
            allMatch: false,
            totalChanges: -1,
            totalFields: 0,
            groups: [],
            configNotFound: true,
          });
        } else {
          throw err;
        }
      }
    }

    return results;
  }, [modes]);

  const loadOtherSettings = useCallback(async () => {
    const results: OtherSettingComparison[] = [];

    // Circuit Breaker
    try {
      const preview = (await loadCircuitBreakerDefaults(true)) as ConfigResetPreview;
      const allFields =
        preview.all_values ||
        preview.differences.map((d) => ({
          path: d.path,
          current: d.current,
          default: d.default,
          match: false,
          risk_level: d.risk_level,
        }));

      results.push({
        settingType: 'circuit_breaker',
        settingName: 'Circuit Breaker (Global)',
        icon: <Shield className="w-6 h-6 text-red-400" />,
        allMatch: preview.all_match,
        totalChanges: preview.total_changes,
        totalFields: allFields.length,
        fields: allFields,
        isAdmin: preview.is_admin,
        rawData: preview,
      });
    } catch (err: any) {
      if (err?.response?.status === 404) {
        results.push({
          settingType: 'circuit_breaker',
          settingName: 'Circuit Breaker (Global)',
          icon: <Shield className="w-6 h-6 text-red-400" />,
          allMatch: false,
          totalChanges: -1,
          totalFields: 0,
          fields: [],
          configNotFound: true,
        });
      }
    }

    // LLM Config
    try {
      const preview = (await loadLLMConfigDefaults(true)) as ConfigResetPreview;
      const allFields =
        preview.all_values ||
        preview.differences.map((d) => ({
          path: d.path,
          current: d.current,
          default: d.default,
          match: false,
          risk_level: d.risk_level,
        }));

      results.push({
        settingType: 'llm_config',
        settingName: 'LLM Config',
        icon: <Brain className="w-6 h-6 text-purple-400" />,
        allMatch: preview.all_match,
        totalChanges: preview.total_changes,
        totalFields: allFields.length,
        fields: allFields,
        isAdmin: preview.is_admin,
        rawData: preview,
      });
    } catch (err: any) {
      if (err?.response?.status === 404) {
        results.push({
          settingType: 'llm_config',
          settingName: 'LLM Config',
          icon: <Brain className="w-6 h-6 text-purple-400" />,
          allMatch: false,
          totalChanges: -1,
          totalFields: 0,
          fields: [],
          configNotFound: true,
        });
      }
    }

    // Capital Allocation
    try {
      const preview = (await loadCapitalAllocationDefaults(true)) as ConfigResetPreview;
      const allFields =
        preview.all_values ||
        preview.differences.map((d) => ({
          path: d.path,
          current: d.current,
          default: d.default,
          match: false,
          risk_level: d.risk_level,
        }));

      results.push({
        settingType: 'capital_allocation',
        settingName: 'Capital Allocation',
        icon: <Wallet className="w-6 h-6 text-green-400" />,
        allMatch: preview.all_match,
        totalChanges: preview.total_changes,
        totalFields: allFields.length,
        fields: allFields,
        isAdmin: preview.is_admin,
        rawData: preview,
      });
    } catch (err: any) {
      if (err?.response?.status === 404) {
        results.push({
          settingType: 'capital_allocation',
          settingName: 'Capital Allocation',
          icon: <Wallet className="w-6 h-6 text-green-400" />,
          allMatch: false,
          totalChanges: -1,
          totalFields: 0,
          fields: [],
          configNotFound: true,
        });
      }
    }

    return results;
  }, []);

  const loadAllComparisons = useCallback(async () => {
    console.log('[SettingsComparison] Loading data...');
    setLoading(true);
    setError(null);

    try {
      const [modeResults, otherResults] = await Promise.all([
        loadModeComparisons(),
        loadOtherSettings(),
      ]);

      console.log('[SettingsComparison] Data loaded, setting state');
      setModeComparisons(modeResults);
      setOtherSettings(otherResults);

      // Capture original values for tracking real changes (admin mode)
      const modeOriginals: Record<string, Record<string, any>> = {};
      modeResults.forEach((result) => {
        modeOriginals[result.mode] = {};
        result.groups.forEach((group) => {
          group.fields.forEach((field) => {
            modeOriginals[result.mode][field.path] = field.current;
          });
        });
      });
      setOriginalModeValues(modeOriginals);

      const otherOriginals: Record<string, Record<string, any>> = {};
      otherResults.forEach((result) => {
        otherOriginals[result.settingType] = {};
        result.fields.forEach((field) => {
          otherOriginals[result.settingType][field.path] = field.current;
        });
      });
      setOriginalOtherValues(otherOriginals);

      // Clear any pending edits on fresh load
      setEditedModeValues({});
      setEditedOtherValues({});

      console.log('[SettingsComparison] Original values captured');
    } catch (err: any) {
      console.error('[SettingsComparison] Failed to load comparisons:', err);
      setError(err?.response?.data?.error || err?.message || 'Failed to load settings comparison');
    } finally {
      setLoading(false);
    }
  }, [loadModeComparisons, loadOtherSettings]);

  useEffect(() => {
    console.log('[SettingsComparison] Component mounted/deps changed, loading data');
    loadAllComparisons();
    return () => {
      console.log('[SettingsComparison] Component unmounting');
    };
  }, [loadAllComparisons]);

  // ==================== TOGGLE HANDLERS ====================

  const toggleSection = (section: string) => {
    setExpandedSections((prev) => {
      const next = new Set(prev);
      if (next.has(section)) {
        next.delete(section);
      } else {
        next.add(section);
      }
      return next;
    });
  };

  const toggleModeExpanded = (mode: string) => {
    setExpandedModes((prev) => {
      const next = new Set(prev);
      if (next.has(mode)) {
        next.delete(mode);
      } else {
        next.add(mode);
      }
      return next;
    });
  };

  const toggleGroupExpanded = (key: string) => {
    setExpandedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  const toggleOtherSettingExpanded = (key: string) => {
    setExpandedOtherSettings((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  const toggleReadOnlyExpanded = (key: string) => {
    setExpandedReadOnly((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  // ==================== EDIT HANDLERS (Admin) ====================

  // Helper to compare values (handles type coercion for booleans)
  const valuesAreEqual = (a: any, b: any): boolean => {
    // Handle boolean comparisons with type coercion
    if (typeof a === 'boolean' || typeof b === 'boolean') {
      const aBool = a === true || a === 'true' || a === 'Yes';
      const bBool = b === true || b === 'true' || b === 'Yes';
      return aBool === bBool;
    }
    // Handle number comparisons
    if (typeof a === 'number' || typeof b === 'number') {
      return Number(a) === Number(b);
    }
    // Handle string comparisons
    return String(a) === String(b);
  };

  const handleModeFieldChange = (mode: string, path: string, value: any) => {
    const originalValue = originalModeValues[mode]?.[path];

    // If value equals original, remove from edited (it's not really changed)
    if (valuesAreEqual(value, originalValue)) {
      setEditedModeValues((prev) => {
        const modeEdits = { ...(prev[mode] || {}) };
        delete modeEdits[path];
        // If no more edits for this mode, remove the mode entry
        if (Object.keys(modeEdits).length === 0) {
          const newPrev = { ...prev };
          delete newPrev[mode];
          return newPrev;
        }
        return { ...prev, [mode]: modeEdits };
      });
    } else {
      // Value is different from original, track it
      setEditedModeValues((prev) => ({
        ...prev,
        [mode]: {
          ...(prev[mode] || {}),
          [path]: value,
        },
      }));
    }
  };

  const handleOtherFieldChange = (settingType: string, path: string, value: any) => {
    const originalValue = originalOtherValues[settingType]?.[path];

    // If value equals original, remove from edited (it's not really changed)
    if (valuesAreEqual(value, originalValue)) {
      setEditedOtherValues((prev) => {
        const settingEdits = { ...(prev[settingType] || {}) };
        delete settingEdits[path];
        // If no more edits for this setting, remove the setting entry
        if (Object.keys(settingEdits).length === 0) {
          const newPrev = { ...prev };
          delete newPrev[settingType];
          return newPrev;
        }
        return { ...prev, [settingType]: settingEdits };
      });
    } else {
      // Value is different from original, track it
      setEditedOtherValues((prev) => ({
        ...prev,
        [settingType]: {
          ...(prev[settingType] || {}),
          [path]: value,
        },
      }));
    }
  };

  // ==================== SUMMARY STATS ====================

  const modeStats = useMemo(() => {
    const upToDate = modeComparisons.filter((c) => c.allMatch && !c.isAdmin && !c.configNotFound).length;
    const outOfDate = modeComparisons.filter((c) => !c.allMatch && !c.isAdmin && !c.configNotFound).length;
    const notConfigured = modeComparisons.filter((c) => c.configNotFound).length;
    const totalFields = modeComparisons.reduce((sum, c) => sum + c.totalFields, 0);
    const totalChanges = modeComparisons.reduce((sum, c) => sum + Math.max(0, c.totalChanges), 0);
    const allMatch = outOfDate === 0 && notConfigured === 0;
    return { upToDate, outOfDate, notConfigured, totalFields, totalChanges, allMatch };
  }, [modeComparisons]);

  const otherStats = useMemo(() => {
    const upToDate = otherSettings.filter((s) => s.allMatch && !s.configNotFound).length;
    const outOfDate = otherSettings.filter((s) => !s.allMatch && !s.configNotFound).length;
    const notConfigured = otherSettings.filter((s) => s.configNotFound).length;
    const totalFields = otherSettings.reduce((sum, s) => sum + s.totalFields, 0);
    const totalChanges = otherSettings.reduce((sum, s) => sum + Math.max(0, s.totalChanges), 0);
    const allMatch = outOfDate === 0;
    return { upToDate, outOfDate, notConfigured, totalFields, totalChanges, allMatch };
  }, [otherSettings]);

  // ==================== RENDER ====================

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center py-12 space-y-3">
        <Loader2 className="w-8 h-8 text-blue-500 animate-spin" />
        <p className="text-gray-400">Loading settings comparison...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-6">
        <div className="flex items-start gap-3">
          <AlertTriangle className="w-6 h-6 text-red-400 flex-shrink-0 mt-0.5" />
          <div className="flex-1">
            <h3 className="text-red-400 font-semibold mb-1">Failed to Load Comparison</h3>
            <p className="text-red-300 text-sm mb-3">{error}</p>
            <button
              onClick={loadAllComparisons}
              className="px-4 py-2 bg-red-600 hover:bg-red-700 text-white text-sm rounded-lg transition-colors flex items-center gap-2"
            >
              <RefreshCw className="w-4 h-4" />
              Retry
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header with Refresh */}
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-bold text-white">
          {isAdmin ? 'Settings Editor (Admin)' : 'Settings Comparison'}
        </h2>
        <button
          onClick={loadAllComparisons}
          className="p-2 text-gray-400 hover:text-white hover:bg-gray-700 rounded-lg transition-colors"
          title="Refresh"
        >
          <RefreshCw className="w-5 h-5" />
        </button>
      </div>

      {/* ==================== SECTION 1: MODE SETTINGS ==================== */}
      <SectionHeader
        title="Mode Settings"
        icon={<Settings className="w-6 h-6 text-blue-400" />}
        isExpanded={expandedSections.has('modes')}
        onToggle={() => toggleSection('modes')}
        allMatch={modeStats.allMatch}
        totalItems={modeComparisons.length}
        matchingItems={modeStats.upToDate}
        resetButton={
          onResetAllModes && !modeStats.allMatch ? (
            <ResetButton onClick={onResetAllModes} label="Reset All Modes" size="medium" />
          ) : undefined
        }
      >
        <div className="p-4 space-y-4">
          {/* Mode Summary Stats */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="bg-green-500/10 border border-green-500/30 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-green-400">{modeStats.upToDate}</div>
              <div className="text-xs text-green-300">Modes Up to Date</div>
            </div>
            <div className="bg-orange-500/10 border border-orange-500/30 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-orange-400">{modeStats.outOfDate}</div>
              <div className="text-xs text-orange-300">Modes with Changes</div>
            </div>
            <div className="bg-blue-500/10 border border-blue-500/30 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-blue-400">{modeStats.totalFields}</div>
              <div className="text-xs text-blue-300">Total Settings</div>
            </div>
            <div className="bg-purple-500/10 border border-purple-500/30 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-purple-400">{modeStats.totalChanges}</div>
              <div className="text-xs text-purple-300">Total Differences</div>
            </div>
          </div>

          {/* Mode Cards */}
          <div className="space-y-4">
            {modeComparisons.map((comparison) => (
              <ModeCard
                key={comparison.mode}
                comparison={comparison}
                isAdmin={isAdmin}
                isExpanded={expandedModes.has(comparison.mode)}
                expandedGroups={expandedGroups}
                onToggleExpand={() => toggleModeExpanded(comparison.mode)}
                onToggleGroup={toggleGroupExpanded}
                onResetMode={onResetMode ? () => onResetMode(comparison.mode) : undefined}
                onResetGroup={
                  onResetModeGroup ? (group) => onResetModeGroup(comparison.mode, group) : undefined
                }
                onSaveMode={
                  onSaveMode && editedModeValues[comparison.mode]
                    ? () => onSaveMode(comparison.mode, editedModeValues[comparison.mode])
                    : undefined
                }
                editedValues={editedModeValues[comparison.mode]}
                onFieldChange={(path, value) => handleModeFieldChange(comparison.mode, path, value)}
              />
            ))}
          </div>
        </div>
      </SectionHeader>

      {/* ==================== SECTION 2: OTHER SETTINGS ==================== */}
      <SectionHeader
        title="Other Settings"
        icon={<Settings className="w-6 h-6 text-purple-400" />}
        isExpanded={expandedSections.has('other')}
        onToggle={() => toggleSection('other')}
        allMatch={otherStats.allMatch}
        totalItems={otherSettings.filter((s) => !s.configNotFound).length}
        matchingItems={otherStats.upToDate}
        resetButton={
          onResetAllOther && !otherStats.allMatch ? (
            <ResetButton onClick={onResetAllOther} label="Reset All Other" size="medium" />
          ) : undefined
        }
      >
        <div className="p-4 space-y-4">
          {/* Other Settings Summary */}
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
            <div className="bg-green-500/10 border border-green-500/30 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-green-400">{otherStats.upToDate}</div>
              <div className="text-xs text-green-300">Up to Date</div>
            </div>
            <div className="bg-orange-500/10 border border-orange-500/30 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-orange-400">{otherStats.outOfDate}</div>
              <div className="text-xs text-orange-300">With Changes</div>
            </div>
            <div className="bg-gray-500/10 border border-gray-500/30 rounded-lg p-3 text-center">
              <div className="text-2xl font-bold text-gray-400">{otherStats.notConfigured}</div>
              <div className="text-xs text-gray-300">Not Configured</div>
            </div>
          </div>

          {/* Other Setting Cards */}
          <div className="space-y-4">
            {otherSettings.map((setting) => {
              const resetHandler =
                setting.settingType === 'circuit_breaker'
                  ? onResetCircuitBreaker
                  : setting.settingType === 'llm_config'
                  ? onResetLLMConfig
                  : setting.settingType === 'capital_allocation'
                  ? onResetCapitalAllocation
                  : undefined;

              return (
                <OtherSettingCard
                  key={setting.settingType}
                  setting={setting}
                  isAdmin={isAdmin}
                  isExpanded={expandedOtherSettings.has(setting.settingType)}
                  onToggleExpand={() => toggleOtherSettingExpanded(setting.settingType)}
                  onReset={resetHandler}
                  onSave={
                    onSaveOtherSetting && editedOtherValues[setting.settingType]
                      ? () =>
                          onSaveOtherSetting(setting.settingType, editedOtherValues[setting.settingType])
                      : undefined
                  }
                  editedValues={editedOtherValues[setting.settingType]}
                  onFieldChange={(path, value) =>
                    handleOtherFieldChange(setting.settingType, path, value)
                  }
                />
              );
            })}
          </div>
        </div>
      </SectionHeader>

      {/* ==================== SECTION 3: READ-ONLY ==================== */}
      <div className="rounded-lg border bg-gray-800 border-gray-700 overflow-hidden">
        <button
          onClick={() => toggleSection('readonly')}
          className="w-full p-4 flex items-center justify-between hover:bg-gray-700/50 transition-colors"
        >
          <div className="flex items-center gap-3">
            <FileText className="w-6 h-6 text-gray-400" />
            <div className="text-left">
              <h3 className="text-lg font-semibold text-white">Read-Only Information</h3>
              <p className="text-sm text-gray-400">Metadata and system information</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <span className="px-3 py-1 text-sm rounded-full font-medium bg-gray-600/30 text-gray-300">
              Info Only
            </span>
            {expandedSections.has('readonly') ? (
              <ChevronUp className="w-5 h-5 text-gray-400" />
            ) : (
              <ChevronDown className="w-5 h-5 text-gray-400" />
            )}
          </div>
        </button>

        {expandedSections.has('readonly') && (
          <div className="border-t border-gray-700/50 p-4 space-y-4">
            {/* Metadata Card */}
            <ReadOnlyCard
              title="Metadata"
              icon={<Info className="w-6 h-6 text-blue-400" />}
              data={metadata}
              isExpanded={expandedReadOnly.has('metadata')}
              onToggleExpand={() => toggleReadOnlyExpanded('metadata')}
            />

            {/* Settings Risk Index Info */}
            <ReadOnlyCard
              title="Settings Risk Index"
              icon={<AlertTriangle className="w-6 h-6 text-yellow-400" />}
              data={{
                high_risk_changes: modeStats.totalChanges > 10 ? 'High' : modeStats.totalChanges > 5 ? 'Medium' : 'Low',
                modes_with_differences: modeStats.outOfDate,
                other_settings_changed: otherStats.outOfDate,
                recommendation:
                  modeStats.totalChanges > 5
                    ? 'Review settings before trading'
                    : 'Settings are within normal parameters',
              }}
              isExpanded={expandedReadOnly.has('risk_index')}
              onToggleExpand={() => toggleReadOnlyExpanded('risk_index')}
            />
          </div>
        )}
      </div>
    </div>
  );
}
