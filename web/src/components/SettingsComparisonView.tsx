import { useState, useEffect, useMemo, useCallback } from 'react';
import {
  CheckCircle2,
  XCircle,
  ChevronDown,
  ChevronUp,
  ChevronRight,
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
  Globe,
  TrendingUp,
  RotateCcw,
  Zap,
  Power,
  PowerOff,
  Sliders,
  Activity,
  BarChart3,
  Edit3,
} from 'lucide-react';
import {
  loadModeDefaults,
  loadCircuitBreakerDefaults,
  loadLLMConfigDefaults,
  loadCapitalAllocationDefaults,
  loadGlobalTradingDefaults,
  ConfigResetPreview,
  FieldComparison,
} from '../services/futuresApi';
import modeStrategyApi from '../api/modeStrategy';
import type { ModeName, StrategyName, ModeStrategyConfig } from '../types/modeStrategy';
import { STRATEGY_DISPLAY_NAMES } from '../types/modeStrategy';
// Story 11.43-11.46: Import sub-strategies hook and types for Ravindra Volume Imbalance
import { useSubStrategies, useUpdateSubStrategy, useStrategyGroups } from '../hooks/useStrategyHierarchy';
import type { SubStrategySettings, VolumeImbalanceSettings, StrategyGroupSettings } from '../types/strategyHierarchy';

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

// Strategy comparison result for nested strategies inside modes
interface StrategyComparisonResult {
  strategy: StrategyName;
  strategyName: string;
  enabled: boolean;
  allMatch: boolean;
  totalFields: number;
  matchingFields: number;
  differentFields: number;
  differences: FieldComparison[];
  allValues: FieldComparison[];  // All fields including matches
  config?: ModeStrategyConfig;
  isLoading?: boolean;
  error?: string;
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
  // NEW: Strategies nested inside modes
  strategies?: StrategyComparisonResult[];
  strategiesAllMatch?: boolean;
  totalStrategyDifferences?: number;
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
  // Strategy resets (NEW for Story 11.34)
  onResetStrategy?: (mode: string, strategy: string) => void;
  onResetAllStrategiesInMode?: (mode: string) => void;
  // Other settings resets
  onResetAllOther?: () => void;
  onResetCircuitBreaker?: () => void;
  onResetLLMConfig?: () => void;
  onResetCapitalAllocation?: () => void;
  onResetGlobalTrading?: () => void;
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

// List of all strategies (Story 11.34)
const ALL_STRATEGIES: StrategyName[] = ['trend_following', 'mean_reversion', 'breakout', 'range_trading'];

// Strategy settings groups - matching the Futures page Mode Strategy Settings UI
// Each group defines which field paths belong to it
interface StrategySettingGroup {
  key: string;
  name: string;
  icon: React.ComponentType<{ className?: string }>;
  iconColor: string;
  fields: string[]; // Field names (not prefixes, since strategy fields are flat)
}

// Story 11.41: Expanded to include all 18 sections
const STRATEGY_SETTING_GROUPS: StrategySettingGroup[] = [
  {
    key: 'position_sizing',
    name: 'Position Sizing',
    icon: Sliders,
    iconColor: 'text-blue-400',
    fields: ['enabled', 'priority', 'leverage', 'max_positions', 'base_size_usd', 'supported_regimes', 'max_size_usd', 'min_position_size_usd', 'safety_margin', 'auto_size_enabled', 'auto_size_min_cover_fee'],
  },
  {
    key: 'timeframe',
    name: 'Timeframe',
    icon: Activity,
    iconColor: 'text-indigo-400',
    fields: ['trend_timeframe', 'entry_timeframe', 'analysis_timeframe'],
  },
  {
    key: 'mtf',
    name: 'Multi-Timeframe (MTF)',
    icon: BarChart3,
    iconColor: 'text-violet-400',
    fields: ['primary_timeframe', 'primary_weight', 'secondary_timeframe', 'secondary_weight', 'tertiary_timeframe', 'tertiary_weight', 'min_consensus', 'min_weighted_strength', 'trend_stability_check'],
  },
  {
    key: 'sltp',
    name: 'Stop Loss / Take Profit',
    icon: Shield,
    iconColor: 'text-red-400',
    fields: ['sl_percent', 'tp1_percent', 'tp1_sell_percent', 'tp2_percent', 'tp2_sell_percent', 'tp3_percent', 'tp3_sell_percent', 'trailing_enabled', 'trailing_activation_pct', 'trailing_stop_pct', 'use_atr_based', 'atr_sl_multiplier', 'atr_tp_multiplier', 'min_sl_distance_pct'],
  },
  {
    key: 'confidence',
    name: 'Confidence Thresholds',
    icon: Target,
    iconColor: 'text-green-400',
    fields: ['min_confidence', 'high_confidence', 'ultra_confidence'],
  },
  {
    key: 'entry_conditions',
    name: 'Entry Conditions',
    icon: TrendingUp,
    iconColor: 'text-purple-400',
    fields: [
      'adx_min', 'adx_max', 'rsi_min', 'rsi_max', 'require_trend_align', 'min_volume_multiplier', 'use_limit_entry', 'limit_order_gap_percent', 'max_limit_gap_percent',
      'rsi_oversold', 'rsi_overbought', 'bollinger_std', 'require_price_at_band',
      'breakout_atr_multiplier', 'volume_spike_multiplier', 'require_consolidation', 'consolidation_bars',
      'range_high_touch', 'range_low_touch', 'range_width_atr', 'min_range_duration_bars',
    ],
  },
  {
    key: 'exit_conditions',
    name: 'Exit Conditions',
    icon: Activity,
    iconColor: 'text-yellow-400',
    fields: ['use_ai_exit', 'exit_at_mean', 'exit_at_range_boundary', 'max_hold_minutes', 'early_warning_enabled', 'exit_on_trend_reversal', 'adx_exit_threshold'],
  },
  {
    key: 'scoring',
    name: 'Scoring Weights',
    icon: BarChart3,
    iconColor: 'text-cyan-400',
    fields: ['technical_weight', 'momentum_weight', 'volume_weight', 'sentiment_weight', 'min_score', 'high_score'],
  },
  {
    key: 'circuit_breaker',
    name: 'Circuit Breaker',
    icon: Shield,
    iconColor: 'text-orange-400',
    fields: ['max_loss_per_hour_usd', 'max_loss_per_day_usd', 'max_consecutive_losses', 'cooldown_minutes', 'max_trades_per_hour', 'max_trades_per_day', 'win_rate_check_after', 'min_win_rate_pct'],
  },
  {
    key: 'hedge',
    name: 'Hedging',
    icon: Shield,
    iconColor: 'text-teal-400',
    fields: ['allow_hedge', 'min_confidence_for_hedge', 'existing_must_be_in_profit_pct', 'max_hedge_size_percent', 'allow_same_mode_hedge', 'max_total_exposure_multiplier'],
  },
  {
    key: 'averaging',
    name: 'Position Averaging',
    icon: TrendingUp,
    iconColor: 'text-emerald-400',
    fields: ['allow_averaging', 'average_up_profit_percent', 'average_down_loss_percent', 'add_size_percent', 'max_averages', 'min_confidence_for_average', 'use_llm_for_averaging', 'staged_entry_enabled', 'staged_entry_levels', 'staged_entry_percent'],
  },
  {
    key: 'stale_release',
    name: 'Stale Position Release',
    icon: Activity,
    iconColor: 'text-amber-400',
    fields: ['max_hold_duration_minutes', 'min_profit_to_keep_pct', 'max_loss_to_force_close_pct', 'stale_zone_lo_pct', 'stale_zone_hi_pct', 'stale_zone_action'],
  },
  {
    key: 'position_optimization',
    name: 'Position Optimization',
    icon: Zap,
    iconColor: 'text-pink-400',
    fields: ['reentry_enabled', 'reentry_after_tp1', 'reentry_min_pullback_pct', 'max_reentries_per_position', 'dynamic_sl_enabled', 'dynamic_sl_at_breakeven_pct', 'profit_protection_enabled', 'profit_protection_trigger_pct', 'profit_protection_lock_pct'],
  },
  {
    key: 'funding_rate',
    name: 'Funding Rate',
    icon: Wallet,
    iconColor: 'text-lime-400',
    fields: ['max_funding_rate_pct', 'exit_before_funding_minutes', 'block_entry_above_rate_pct'],
  },
  {
    key: 'risk',
    name: 'Risk Management',
    icon: Shield,
    iconColor: 'text-rose-400',
    fields: ['risk_level', 'max_drawdown_percent', 'max_daily_loss_percent', 'position_risk_percent'],
  },
  {
    key: 'trend_divergence',
    name: 'Trend Divergence',
    icon: TrendingUp,
    iconColor: 'text-fuchsia-400',
    fields: ['min_divergence_percent', 'block_on_divergence', 'divergence_weight'],
  },
  {
    key: 'dynamic_ai_exit',
    name: 'Dynamic AI Exit',
    icon: Brain,
    iconColor: 'text-sky-400',
    fields: ['min_hold_before_ai_ms', 'ai_check_interval_ms', 'use_llm_for_loss', 'use_llm_for_profit', 'max_hold_time_ms'],
  },
  {
    key: 'early_warning',
    name: 'Early Warning',
    icon: AlertTriangle,
    iconColor: 'text-yellow-500',
    fields: ['start_after_minutes', 'min_loss_percent', 'check_interval_secs', 'close_min_hold_mins'],
  },
];

// Group strategy fields by their settings group
interface GroupedStrategyFields {
  group: StrategySettingGroup;
  fields: FieldComparison[];
  allMatch: boolean;
  totalFields: number;
  matchingFields: number;
  differentFields: number;
}

function groupStrategyFieldsByCategory(allFields: FieldComparison[]): GroupedStrategyFields[] {
  const result: GroupedStrategyFields[] = [];
  const usedPaths = new Set<string>();

  // Groups that have a section prefix in the path (e.g., "mtf.enabled", "hedge.allow_hedge")
  const prefixedGroups = new Set([
    'mtf', 'circuit_breaker', 'hedge', 'averaging', 'stale_release',
    'position_optimization', 'funding_rate', 'risk', 'trend_divergence',
    'dynamic_ai_exit', 'early_warning', 'sltp', 'confidence', 'entry_conditions',
    'exit_conditions', 'scoring', 'timeframe'
  ]);

  for (const group of STRATEGY_SETTING_GROUPS) {
    const groupFields: FieldComparison[] = [];

    for (const field of allFields) {
      if (usedPaths.has(field.path)) continue;

      // Check if field belongs to this group based on path prefix or field name
      let belongsToGroup = false;

      if (prefixedGroups.has(group.key)) {
        // For prefixed groups, match if path starts with the group key
        // e.g., "mtf.enabled" matches group "mtf", "sltp.sl_percent" matches group "sltp"
        const pathPrefix = field.path.split('.')[0];
        if (pathPrefix === group.key) {
          belongsToGroup = true;
        }
      } else {
        // For non-prefixed groups like "position_sizing", match by field name
        // These are top-level fields like "enabled", "leverage", "max_positions"
        const fieldName = field.path.includes('.') ? field.path.split('.').pop() || '' : field.path;
        // Only match if it's a top-level field (no prefix) and the field name is in the group's fields
        if (!prefixedGroups.has(field.path.split('.')[0]) && group.fields.includes(fieldName)) {
          belongsToGroup = true;
        }
      }

      if (belongsToGroup) {
        groupFields.push(field);
        usedPaths.add(field.path);
      }
    }

    if (groupFields.length > 0) {
      const matchingCount = groupFields.filter((f) => f.match).length;
      const differentCount = groupFields.filter((f) => !f.match).length;

      result.push({
        group,
        fields: groupFields,
        allMatch: differentCount === 0,
        totalFields: groupFields.length,
        matchingFields: matchingCount,
        differentFields: differentCount,
      });
    }
  }

  // Handle any remaining fields as "Other"
  const otherFields = allFields.filter((f) => !usedPaths.has(f.path));
  if (otherFields.length > 0) {
    const matchingCount = otherFields.filter((f) => f.match).length;
    const differentCount = otherFields.filter((f) => !f.match).length;

    result.push({
      group: {
        key: 'other',
        name: 'Other Settings',
        icon: Settings,
        iconColor: 'text-gray-400',
        fields: [],
      },
      fields: otherFields,
      allMatch: differentCount === 0,
      totalFields: otherFields.length,
      matchingFields: matchingCount,
      differentFields: differentCount,
    });
  }

  return result;
}

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
  profit_reinvest_risk_level: ['conservative', 'moderate', 'aggressive'],
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

// Timezone presets matching Settings page (combined timezone + offset)
const TIMEZONE_PRESETS = [
  { tz_identifier: 'UTC', display_name: 'Coordinated Universal Time (UTC)', gmt_offset: '+00:00' },
  { tz_identifier: 'Asia/Kolkata', display_name: 'India Standard Time (IST)', gmt_offset: '+05:30' },
  { tz_identifier: 'Asia/Phnom_Penh', display_name: 'Indochina Time (ICT)', gmt_offset: '+07:00' },
];

// Detect field type from value and field name
function getFieldType(fieldName: string, value: any): 'boolean' | 'dropdown' | 'timezone' | 'number' | 'text' {
  // Boolean fields
  if (typeof value === 'boolean') return 'boolean';

  // Special timezone field (combined dropdown)
  if (fieldName === 'timezone') return 'timezone';

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

  // Timezone - Combined dropdown matching Settings page
  if (fieldType === 'timezone') {
    return (
      <div className="flex items-center gap-2">
        <select
          value={String(value)}
          onChange={(e) => onChange(e.target.value)}
          className="px-2 py-1 bg-gray-800 border border-gray-600 rounded text-white text-xs font-mono focus:border-blue-500 focus:outline-none min-w-[280px]"
        >
          {TIMEZONE_PRESETS.map((preset) => (
            <option key={preset.tz_identifier} value={preset.tz_identifier}>
              {preset.display_name} ({preset.gmt_offset})
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

// ==================== STRATEGY EDITABLE INPUT COMPONENTS ====================

// Strategy Number Input - Similar to StrategySettingsForm but compact
function StrategyNumberInput({
  label,
  value,
  defaultValue,
  onChange,
  min = 0,
  max = 100,
  step = 1,
  unit = '',
  description,
  isEdited = false,
}: {
  label: string;
  value: number | null | undefined;
  defaultValue: number | null | undefined;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  unit?: string;
  description?: string;
  isEdited?: boolean;
}) {
  const displayValue = value ?? defaultValue ?? 0;

  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between">
        <label className="text-xs font-medium text-gray-300">{label}</label>
        <div className="flex items-center gap-1">
          <input
            type="number"
            value={displayValue}
            onChange={(e) => onChange(Number(e.target.value))}
            min={min}
            max={max}
            step={step}
            className={`w-20 px-2 py-1 bg-gray-700 border rounded text-white text-right text-xs focus:outline-none focus:ring-2 focus:ring-purple-500 ${
              isEdited ? 'border-orange-500' : 'border-gray-600'
            }`}
          />
          {unit && <span className="text-xs text-gray-500 w-8">{unit}</span>}
          {isEdited && <span className="text-orange-400 text-xs">*</span>}
        </div>
      </div>
      {description && (
        <p className="text-xs text-gray-500">{description}</p>
      )}
    </div>
  );
}

// Strategy Toggle Input - Similar to StrategySettingsForm but compact
function StrategyToggleInput({
  label,
  checked,
  defaultChecked,
  onChange,
  description,
  isEdited = false,
}: {
  label: string;
  checked: boolean | null | undefined;
  defaultChecked: boolean | null | undefined;
  onChange: (checked: boolean) => void;
  description?: string;
  isEdited?: boolean;
}) {
  const displayValue = checked ?? defaultChecked ?? false;

  return (
    <div className="flex items-start justify-between py-1">
      <div className="flex-1">
        <label className="text-xs font-medium text-gray-300">{label}</label>
        {description && (
          <p className="text-xs text-gray-500 mt-0.5">{description}</p>
        )}
      </div>
      <div className="flex items-center gap-1">
        <button
          type="button"
          onClick={() => onChange(!displayValue)}
          className={`
            relative inline-flex h-5 w-9 items-center rounded-full transition-colors cursor-pointer
            ${displayValue ? 'bg-purple-600' : 'bg-gray-600'}
            ${isEdited ? 'ring-2 ring-orange-500' : ''}
          `}
        >
          <span
            className={`
              inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform
              ${displayValue ? 'translate-x-5' : 'translate-x-0.5'}
            `}
          />
        </button>
        {isEdited && <span className="text-orange-400 text-xs">*</span>}
      </div>
    </div>
  );
}

// Strategy Slider Input - Similar to StrategySettingsForm but compact
function StrategySliderInput({
  label,
  value,
  defaultValue,
  onChange,
  min,
  max,
  step = 1,
  unit = '',
  description,
  isEdited = false,
}: {
  label: string;
  value: number | null | undefined;
  defaultValue: number | null | undefined;
  onChange: (value: number) => void;
  min: number;
  max: number;
  step?: number;
  unit?: string;
  description?: string;
  isEdited?: boolean;
}) {
  const displayValue = value ?? defaultValue ?? min;

  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between">
        <label className="text-xs font-medium text-gray-300">{label}</label>
        <span className={`text-xs font-mono ${isEdited ? 'text-orange-400' : 'text-purple-400'}`}>
          {displayValue}{unit} {isEdited && '*'}
        </span>
      </div>
      {description && (
        <p className="text-xs text-gray-500">{description}</p>
      )}
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={displayValue}
        onChange={(e) => onChange(Number(e.target.value))}
        className={`w-full h-1.5 bg-gray-700 rounded-lg appearance-none cursor-pointer accent-purple-500 ${
          isEdited ? 'ring-1 ring-orange-500' : ''
        }`}
      />
      <div className="flex justify-between text-xs text-gray-600">
        <span>{min}{unit}</span>
        <span>{max}{unit}</span>
      </div>
    </div>
  );
}

// Strategy Select Input - Similar to StrategySettingsForm but compact
function StrategySelectInput({
  label,
  value,
  defaultValue,
  onChange,
  options,
  description,
  isEdited = false,
}: {
  label: string;
  value: string | null | undefined;
  defaultValue: string | null | undefined;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
  description?: string;
  isEdited?: boolean;
}) {
  const displayValue = value ?? defaultValue ?? '';

  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between">
        <label className="text-xs font-medium text-gray-300">{label}</label>
        <div className="flex items-center gap-1">
          <select
            value={displayValue}
            onChange={(e) => onChange(e.target.value)}
            className={`w-24 px-2 py-1 bg-gray-700 border rounded text-white text-xs focus:outline-none focus:ring-2 focus:ring-purple-500 ${
              isEdited ? 'border-orange-500' : 'border-gray-600'
            }`}
          >
            {options.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
          {isEdited && <span className="text-orange-400 text-xs">*</span>}
        </div>
      </div>
      {description && (
        <p className="text-xs text-gray-500">{description}</p>
      )}
    </div>
  );
}

// Strategy field type detector for editable inputs
function getStrategyFieldInputType(path: string, value: any): 'toggle' | 'slider' | 'number' | 'select' | 'text' {
  const fieldName = path.split('.').pop() || path;

  // Boolean fields
  if (typeof value === 'boolean') return 'toggle';
  if (
    fieldName.startsWith('enabled') ||
    fieldName.startsWith('use_') ||
    fieldName.startsWith('allow_') ||
    fieldName.endsWith('_enabled') ||
    fieldName.endsWith('_check') ||
    fieldName === 'block_on_divergence' ||
    fieldName === 'trailing_enabled' ||
    fieldName === 'staged_entry_enabled' ||
    fieldName === 'trend_stability_check' ||
    fieldName === 'require_trend_align' ||
    fieldName === 'require_price_at_band' ||
    fieldName === 'require_consolidation' ||
    fieldName === 'range_high_touch' ||
    fieldName === 'range_low_touch' ||
    fieldName === 'use_ai_exit' ||
    fieldName === 'exit_at_mean' ||
    fieldName === 'exit_at_range_boundary' ||
    fieldName === 'early_warning_enabled' ||
    fieldName === 'exit_on_trend_reversal'
  ) {
    return 'toggle';
  }

  // Percentage fields (use slider)
  if (
    fieldName.endsWith('_percent') ||
    fieldName.endsWith('_pct') ||
    fieldName.endsWith('_weight') ||
    fieldName === 'min_confidence' ||
    fieldName === 'high_confidence' ||
    fieldName === 'ultra_confidence' ||
    fieldName === 'min_win_rate_pct' ||
    fieldName === 'sl_percent' ||
    fieldName === 'tp1_percent' ||
    fieldName === 'tp2_percent' ||
    fieldName === 'tp3_percent'
  ) {
    return 'slider';
  }

  // Timeframe fields (use select)
  if (
    fieldName.endsWith('_timeframe') ||
    fieldName === 'trend_timeframe' ||
    fieldName === 'entry_timeframe' ||
    fieldName === 'analysis_timeframe' ||
    fieldName === 'primary_timeframe' ||
    fieldName === 'secondary_timeframe' ||
    fieldName === 'tertiary_timeframe'
  ) {
    return 'select';
  }

  // Risk level (use select)
  if (fieldName === 'risk_level' || fieldName === 'stale_zone_action') {
    return 'select';
  }

  // Number fields
  if (typeof value === 'number') return 'number';

  return 'text';
}

// Get appropriate input props for a strategy field
function getStrategyFieldConfig(path: string, value: any): {
  min?: number;
  max?: number;
  step?: number;
  unit?: string;
  options?: { value: string; label: string }[];
} {
  const fieldName = path.split('.').pop() || path;

  // Timeframe options
  if (fieldName.endsWith('_timeframe')) {
    return {
      options: [
        { value: '1m', label: '1m' },
        { value: '3m', label: '3m' },
        { value: '5m', label: '5m' },
        { value: '15m', label: '15m' },
        { value: '30m', label: '30m' },
        { value: '1h', label: '1h' },
        { value: '2h', label: '2h' },
        { value: '4h', label: '4h' },
        { value: '1d', label: '1d' },
      ],
    };
  }

  // Risk level options
  if (fieldName === 'risk_level') {
    return {
      options: [
        { value: 'low', label: 'Low' },
        { value: 'medium', label: 'Medium' },
        { value: 'high', label: 'High' },
        { value: 'aggressive', label: 'Aggressive' },
      ],
    };
  }

  // Stale zone action options
  if (fieldName === 'stale_zone_action') {
    return {
      options: [
        { value: 'hold', label: 'Hold' },
        { value: 'reduce', label: 'Reduce' },
        { value: 'close', label: 'Close' },
        { value: 'llm_decide', label: 'LLM Decide' },
      ],
    };
  }

  // Percentage fields
  if (
    fieldName === 'min_confidence' ||
    fieldName === 'high_confidence' ||
    fieldName === 'ultra_confidence'
  ) {
    return { min: 30, max: 100, step: 5, unit: '%' };
  }

  if (
    fieldName.endsWith('_weight') ||
    fieldName === 'primary_weight' ||
    fieldName === 'secondary_weight' ||
    fieldName === 'tertiary_weight'
  ) {
    return { min: 0, max: 100, step: 5, unit: '%' };
  }

  if (
    fieldName === 'sl_percent' ||
    fieldName === 'tp1_percent' ||
    fieldName === 'tp2_percent' ||
    fieldName === 'tp3_percent' ||
    fieldName.endsWith('_sell_percent')
  ) {
    return { min: 0, max: 100, step: 0.5, unit: '%' };
  }

  if (fieldName.endsWith('_percent') || fieldName.endsWith('_pct')) {
    return { min: 0, max: 100, step: 0.5, unit: '%' };
  }

  // Leverage
  if (fieldName === 'leverage') {
    return { min: 1, max: 125, step: 1, unit: 'x' };
  }

  // USD amounts
  if (fieldName.endsWith('_usd')) {
    return { min: 10, max: 50000, step: 10, unit: 'USD' };
  }

  // Minutes
  if (fieldName.endsWith('_minutes') || fieldName.endsWith('_mins')) {
    return { min: 1, max: 10080, step: 1, unit: 'min' };
  }

  // Seconds
  if (fieldName.endsWith('_secs') || fieldName.endsWith('_seconds')) {
    return { min: 1, max: 3600, step: 1, unit: 'sec' };
  }

  // Milliseconds (convert to seconds for display)
  if (fieldName.endsWith('_ms')) {
    return { min: 0, max: 3600, step: 1, unit: 'sec' };
  }

  // Max positions
  if (fieldName === 'max_positions') {
    return { min: 1, max: 50, step: 1 };
  }

  // Priority
  if (fieldName === 'priority') {
    return { min: 1, max: 10, step: 1 };
  }

  // Default number config
  return { min: 0, max: 1000, step: 1 };
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
          {fields.map((field) => {
            const isEdited = editedValues && editedValues[field.path] !== undefined;
            const displayValue = isEdited ? editedValues[field.path] : field.current;
            const fieldName = field.path.split('.').pop() || '';
            const notConfigured = isFieldNotConfigured(field);

            return (
              <tr
                key={field.path}
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
  // Strategy props (Story 11.34)
  selectedStrategy,
  onSelectStrategy,
  onResetStrategy,
  onResetAllStrategies,
  resettingStrategies,
  // Strategy editing props
  editedStrategyValues,
  onStrategyFieldChange,
  onSaveStrategy,
  isSavingStrategy,
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
  // Strategy props (Story 11.34)
  selectedStrategy?: StrategyName | null;
  onSelectStrategy?: (strategy: StrategyName) => void;
  onResetStrategy?: (strategy: StrategyName) => void;
  onResetAllStrategies?: () => void;
  resettingStrategies?: Set<string>;
  // Strategy editing props
  editedStrategyValues?: Record<string, any>;
  onStrategyFieldChange?: (path: string, value: any) => void;
  onSaveStrategy?: () => void;
  isSavingStrategy?: boolean;
}) {
  const hasEdits = editedValues && Object.keys(editedValues).length > 0;
  const hasStrategies = comparison.strategies && comparison.strategies.length > 0;

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
          ) : (
            <div className="p-4 space-y-4">
              {/* === STRATEGIES SECTION (Story 11.34) === */}
              {hasStrategies && (
                <div className="border border-purple-500/30 rounded-lg overflow-hidden bg-purple-900/10">
                  {/* Strategies Header */}
                  <div className="px-4 py-3 border-b border-purple-500/20 flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Settings className="w-5 h-5 text-purple-400" />
                      <h5 className="font-medium text-purple-300">Strategies</h5>
                      <span className="text-xs text-gray-500">
                        ({comparison.strategies?.filter(s => s.enabled).length || 0}/{ALL_STRATEGIES.length} enabled)
                      </span>
                    </div>
                    {onResetAllStrategies && !comparison.strategiesAllMatch && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          onResetAllStrategies();
                        }}
                        className="px-2 py-1 text-xs bg-blue-600 hover:bg-blue-700 text-white rounded transition-colors flex items-center gap-1"
                      >
                        <RefreshCw className="w-3 h-3" />
                        Reset All Strategies
                      </button>
                    )}
                  </div>

                  {/* Strategy Cards Grid */}
                  <div className="p-3">
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
                      {comparison.strategies?.map((strategy) => (
                        <StrategyCard
                          key={strategy.strategy}
                          strategy={strategy}
                          isSelected={selectedStrategy === strategy.strategy}
                          onSelect={() => onSelectStrategy?.(strategy.strategy)}
                          onReset={onResetStrategy ? () => onResetStrategy(strategy.strategy) : undefined}
                          isResetting={resettingStrategies?.has(strategy.strategy)}
                        />
                      ))}
                    </div>

                    {/* Selected Strategy Settings Panel */}
                    {selectedStrategy && comparison.strategies && (
                      <div className="mt-3 border-t border-purple-500/20 pt-3">
                        <div className="text-sm text-purple-300 mb-2 flex items-center gap-2">
                          <Target className="w-4 h-4" />
                          Selected: {STRATEGY_DISPLAY_NAMES[selectedStrategy]} - Settings
                        </div>
                        {comparison.strategies
                          .filter(s => s.strategy === selectedStrategy)
                          .map(strategy => (
                            <StrategySettingsPanel
                              key={strategy.strategy}
                              strategy={strategy}
                              mode={comparison.mode}
                              isAdmin={isAdmin}
                              editedValues={editedStrategyValues}
                              onFieldChange={onStrategyFieldChange}
                              onSave={onSaveStrategy}
                              isSaving={isSavingStrategy}
                            />
                          ))}
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* No data message */}
              {!hasStrategies && (
                <div className="p-6 text-center">
                  <Info className="w-12 h-12 text-purple-400 mx-auto mb-3" />
                  <p className="text-gray-400">No settings data available.</p>
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// Strategy Card Component (Story 11.34)
// Displays a strategy card inside a mode, with ON/OFF badge, settings count, and reset button
function StrategyCard({
  strategy,
  isSelected,
  onSelect,
  onReset,
  isResetting,
}: {
  strategy: StrategyComparisonResult;
  isSelected: boolean;
  onSelect: () => void;
  onReset?: () => void;
  isResetting?: boolean;
}) {
  const getStrategyIcon = (name: StrategyName) => {
    switch (name) {
      case 'trend_following':
        return <TrendingUp className="w-4 h-4" />;
      case 'mean_reversion':
        return <RotateCcw className="w-4 h-4" />;
      case 'breakout':
        return <Zap className="w-4 h-4" />;
      case 'range_trading':
        return <Target className="w-4 h-4" />;
    }
  };

  return (
    <div
      onClick={onSelect}
      className={`
        relative p-3 rounded-lg border-2 cursor-pointer transition-all
        ${isSelected
          ? 'border-purple-500 bg-purple-500/10'
          : strategy.allMatch
          ? 'border-green-500/30 bg-green-900/10 hover:border-green-500/50'
          : 'border-orange-500/30 bg-orange-900/10 hover:border-orange-500/50'}
      `}
    >
      {/* Strategy Header */}
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <span className={isSelected ? 'text-purple-400' : strategy.allMatch ? 'text-green-400' : 'text-orange-400'}>
            {getStrategyIcon(strategy.strategy)}
          </span>
          <span className={`text-sm font-medium ${isSelected ? 'text-purple-300' : 'text-gray-200'}`}>
            {strategy.strategyName}
          </span>
        </div>
        {/* ON/OFF Badge */}
        <span
          className={`
            flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium
            ${strategy.enabled
              ? 'bg-green-500/20 text-green-400'
              : 'bg-gray-600/20 text-gray-500'}
          `}
        >
          {strategy.enabled ? (
            <>
              <Power className="w-3 h-3" />
              ON
            </>
          ) : (
            <>
              <PowerOff className="w-3 h-3" />
              OFF
            </>
          )}
        </span>
      </div>

      {/* Settings Match Count */}
      <div className="text-xs text-gray-400 mb-2">
        {strategy.isLoading ? (
          <span className="flex items-center gap-1">
            <Loader2 className="w-3 h-3 animate-spin" />
            Loading...
          </span>
        ) : strategy.error ? (
          <span className="text-red-400">{strategy.error}</span>
        ) : strategy.allMatch ? (
          <span className="text-green-400">{strategy.matchingFields}/{strategy.totalFields} match</span>
        ) : (
          <span className="text-orange-400">{strategy.differentFields} differences</span>
        )}
      </div>

      {/* Reset Button */}
      {onReset && !strategy.allMatch && !strategy.isLoading && (
        <button
          onClick={(e) => {
            e.stopPropagation();
            onReset();
          }}
          disabled={isResetting}
          className="w-full mt-1 px-2 py-1 text-xs bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white rounded transition-colors flex items-center justify-center gap-1"
        >
          {isResetting ? (
            <>
              <Loader2 className="w-3 h-3 animate-spin" />
              Resetting...
            </>
          ) : (
            <>
              <RefreshCw className="w-3 h-3" />
              Reset
            </>
          )}
        </button>
      )}
    </div>
  );
}

// Collapsible Strategy Section Component - for grouped settings with editable UI
function CollapsibleStrategySection({
  groupData,
  expanded,
  onToggle,
  editMode = false,
  editedValues = {},
  onFieldChange,
}: {
  groupData: GroupedStrategyFields;
  expanded: boolean;
  onToggle: () => void;
  editMode?: boolean;
  editedValues?: Record<string, any>;
  onFieldChange?: (path: string, value: any) => void;
}) {
  const IconComponent = groupData.group.icon;

  // Check if any field in this group has been edited
  const hasEdits = groupData.fields.some((field) => editedValues[field.path] !== undefined);

  // Render editable field input based on field type
  const renderEditableField = (field: FieldComparison) => {
    const fieldName = field.path.split('.').pop() || field.path;
    const inputType = getStrategyFieldInputType(field.path, field.current ?? field.default);
    const config = getStrategyFieldConfig(field.path, field.current ?? field.default);
    const currentValue = editedValues[field.path] ?? field.current;
    const isEdited = editedValues[field.path] !== undefined;

    switch (inputType) {
      case 'toggle':
        return (
          <StrategyToggleInput
            label={fieldName.replace(/_/g, ' ')}
            checked={currentValue as boolean | null | undefined}
            defaultChecked={field.default as boolean | null | undefined}
            onChange={(value) => onFieldChange?.(field.path, value)}
            isEdited={isEdited}
          />
        );

      case 'slider':
        return (
          <StrategySliderInput
            label={fieldName.replace(/_/g, ' ')}
            value={currentValue as number | null | undefined}
            defaultValue={field.default as number | null | undefined}
            onChange={(value) => onFieldChange?.(field.path, value)}
            min={config.min ?? 0}
            max={config.max ?? 100}
            step={config.step ?? 1}
            unit={config.unit ?? ''}
            isEdited={isEdited}
          />
        );

      case 'select':
        return (
          <StrategySelectInput
            label={fieldName.replace(/_/g, ' ')}
            value={currentValue as string | null | undefined}
            defaultValue={field.default as string | null | undefined}
            onChange={(value) => onFieldChange?.(field.path, value)}
            options={config.options ?? []}
            isEdited={isEdited}
          />
        );

      case 'number':
      default:
        return (
          <StrategyNumberInput
            label={fieldName.replace(/_/g, ' ')}
            value={currentValue as number | null | undefined}
            defaultValue={field.default as number | null | undefined}
            onChange={(value) => onFieldChange?.(field.path, value)}
            min={config.min ?? 0}
            max={config.max ?? 1000}
            step={config.step ?? 1}
            unit={config.unit ?? ''}
            isEdited={isEdited}
          />
        );
    }
  };

  return (
    <div className="border border-gray-700 rounded-lg overflow-hidden">
      {/* Section Header */}
      <button
        type="button"
        onClick={onToggle}
        className={`w-full flex items-center justify-between px-3 py-2 transition-colors ${
          hasEdits
            ? 'bg-orange-900/30 hover:bg-orange-900/40'
            : groupData.allMatch
            ? 'bg-green-900/20 hover:bg-green-900/30'
            : 'bg-orange-900/20 hover:bg-orange-900/30'
        }`}
      >
        <div className="flex items-center gap-2">
          <span className={groupData.group.iconColor}>
            <IconComponent className="w-4 h-4" />
          </span>
          <span className="font-medium text-gray-200 text-sm">{groupData.group.name}</span>
          <span className="text-xs text-gray-500">
            ({groupData.matchingFields}/{groupData.totalFields})
          </span>
          {editMode && (
            <span className="text-xs text-purple-400 ml-1">
              <Edit3 className="w-3 h-3 inline" />
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          {hasEdits ? (
            <span className="text-xs px-2 py-0.5 bg-orange-500/30 text-orange-300 rounded font-medium">
              Modified
            </span>
          ) : groupData.allMatch ? (
            <span className="text-xs px-2 py-0.5 bg-green-500/20 text-green-400 rounded">
              Match
            </span>
          ) : (
            <span className="text-xs px-2 py-0.5 bg-orange-500/20 text-orange-400 rounded">
              {groupData.differentFields} diff
            </span>
          )}
          {expanded ? (
            <ChevronDown className="w-4 h-4 text-gray-400" />
          ) : (
            <ChevronRight className="w-4 h-4 text-gray-400" />
          )}
        </div>
      </button>

      {/* Section Content - Editable or Read-only */}
      {expanded && (
        <div className="bg-gray-900/30 border-t border-gray-700 p-3 space-y-2">
          {editMode && onFieldChange ? (
            // Editable Mode - Show proper UI controls
            <div className="grid gap-3">
              {groupData.fields.map((field) => (
                <div key={field.path} className="relative">
                  {renderEditableField(field)}
                  {/* Show default value hint when different */}
                  {!field.match && editedValues[field.path] === undefined && (
                    <div className="mt-0.5 text-xs text-blue-400/70">
                      Default: {formatValue(field.default)}
                    </div>
                  )}
                </div>
              ))}
            </div>
          ) : (
            // Read-only Mode - Show comparison table
            <table className="w-full text-xs">
              <thead className="bg-gray-800/50">
                <tr className="text-gray-400 border-b border-gray-700/50">
                  <th className="text-left p-2 pl-3 font-medium">Setting</th>
                  <th className="text-left p-2 font-medium">Current</th>
                  <th className="text-left p-2 font-medium">Default</th>
                  <th className="text-left p-2 pr-3 font-medium">Status</th>
                </tr>
              </thead>
              <tbody>
                {groupData.fields.map((field) => (
                  <tr
                    key={field.path}
                    className={`border-b border-gray-700/30 last:border-0 ${
                      field.match ? 'bg-green-900/5' : 'bg-orange-900/10'
                    }`}
                  >
                    <td className="p-2 pl-3 font-mono text-gray-300">
                      {field.path.split('.').pop() || field.path}
                    </td>
                    <td className={`p-2 font-mono ${field.match ? 'text-green-400' : 'text-orange-400'}`}>
                      {formatValue(field.current)}
                    </td>
                    <td className="p-2 font-mono text-blue-400">
                      {formatValue(field.default)}
                    </td>
                    <td className="p-2 pr-3">
                      {field.match ? (
                        <span className="text-green-400 flex items-center gap-1">
                          <CheckCircle2 className="w-3 h-3" />
                        </span>
                      ) : (
                        <span className="text-orange-400 flex items-center gap-1">
                          <XCircle className="w-3 h-3" />
                        </span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  );
}

// Strategy Settings Expansion Panel (Story 11.34)
// Shows ALL settings with grouped collapsible sections when strategy is selected
// Enhanced with editable UI controls
function StrategySettingsPanel({
  strategy,
  mode,
  isAdmin,
  editedValues = {},
  onFieldChange,
  onSave,
  isSaving = false,
}: {
  strategy: StrategyComparisonResult;
  mode: string;
  isAdmin: boolean;
  editedValues?: Record<string, any>;
  onFieldChange?: (path: string, value: any) => void;
  onSave?: () => void;
  isSaving?: boolean;
}) {
  // Track which sections are expanded (default: Position Settings expanded)
  const [expandedSections, setExpandedSections] = useState<Set<string>>(new Set(['position_sizing']));
  // Toggle between edit and view mode
  const [editMode, setEditMode] = useState(false);

  const toggleSection = (key: string) => {
    setExpandedSections(prev => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  // Check if there are unsaved changes
  const hasEdits = Object.keys(editedValues).length > 0;

  if (strategy.isLoading) {
    return (
      <div className="p-4 text-center">
        <Loader2 className="w-6 h-6 animate-spin text-purple-400 mx-auto mb-2" />
        <p className="text-sm text-gray-400">Loading strategy settings...</p>
      </div>
    );
  }

  if (strategy.error) {
    return (
      <div className="p-4 text-center">
        <AlertTriangle className="w-6 h-6 text-red-400 mx-auto mb-2" />
        <p className="text-sm text-red-400">{strategy.error}</p>
      </div>
    );
  }

  // Use allValues if available, otherwise fall back to differences
  const fieldsToShow = strategy.allValues.length > 0 ? strategy.allValues : strategy.differences;

  if (fieldsToShow.length === 0) {
    return (
      <div className="p-4 text-center">
        <Info className="w-6 h-6 text-gray-400 mx-auto mb-2" />
        <p className="text-sm text-gray-400">No settings to display</p>
      </div>
    );
  }

  // Group fields by category
  const groupedFields = groupStrategyFieldsByCategory(fieldsToShow);

  return (
    <div className="p-3">
      {/* Header with Edit Mode Toggle and Save Button */}
      <div className="flex items-center justify-between mb-3 pb-2 border-b border-gray-700/50">
        <div className="text-xs text-gray-400 flex items-center gap-2">
          <span>
            {strategy.allMatch
              ? `All ${strategy.totalFields} settings match defaults`
              : `${strategy.matchingFields}/${strategy.totalFields} match (${strategy.differentFields} diff)`}
          </span>
          <span className={strategy.allMatch ? 'text-green-400' : 'text-orange-400'}>
            {strategy.allMatch ? (
              <CheckCircle2 className="w-3.5 h-3.5 inline" />
            ) : (
              <XCircle className="w-3.5 h-3.5 inline" />
            )}
          </span>
        </div>
        <div className="flex items-center gap-2">
          {/* Edit Mode Toggle */}
          <button
            onClick={() => setEditMode(!editMode)}
            className={`flex items-center gap-1 px-2 py-1 rounded text-xs transition-colors ${
              editMode
                ? 'bg-purple-600 text-white'
                : 'bg-gray-700 text-gray-300 hover:bg-gray-600'
            }`}
          >
            <Edit3 className="w-3 h-3" />
            {editMode ? 'Editing' : 'Edit'}
          </button>

          {/* Save Button - Only show when in edit mode and has changes */}
          {editMode && hasEdits && onSave && (
            <button
              onClick={onSave}
              disabled={isSaving}
              className="flex items-center gap-1 px-3 py-1 bg-green-600 hover:bg-green-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white rounded text-xs transition-colors"
            >
              {isSaving ? (
                <>
                  <Loader2 className="w-3 h-3 animate-spin" />
                  Saving...
                </>
              ) : (
                <>
                  <Save className="w-3 h-3" />
                  Save ({Object.keys(editedValues).length})
                </>
              )}
            </button>
          )}
        </div>
      </div>

      {/* Unsaved Changes Warning */}
      {hasEdits && (
        <div className="mb-3 p-2 bg-orange-500/10 border border-orange-500/30 rounded text-xs text-orange-400 flex items-center gap-2">
          <AlertTriangle className="w-3.5 h-3.5" />
          <span>{Object.keys(editedValues).length} unsaved changes</span>
        </div>
      )}

      {/* Grouped Collapsible Sections */}
      <div className="space-y-2 max-h-[500px] overflow-y-auto">
        {/* Story 11.43-11.46: Sub-Strategies Section for Breakout Strategy - BEFORE other settings */}
        {strategy.strategy === 'breakout' && (
          <SubStrategiesSection
            mode={mode}
            expandedSections={expandedSections}
            onToggleSection={toggleSection}
            editMode={editMode}
            onFieldChange={onFieldChange}
            onRefresh={() => window.location.reload()}
          />
        )}

        {groupedFields.map((groupData) => (
          <CollapsibleStrategySection
            key={groupData.group.key}
            groupData={groupData}
            expanded={expandedSections.has(groupData.group.key)}
            onToggle={() => toggleSection(groupData.group.key)}
            editMode={editMode}
            editedValues={editedValues}
            onFieldChange={onFieldChange}
          />
        ))}
      </div>
    </div>
  );
}

// Story 11.43-11.46: Sub-Strategies Section Component
// Displays sub-strategies (e.g., Ravindra Volume Imbalance) inside the Breakout strategy
// Enhanced to follow CollapsibleStrategySection pattern with field comparisons and editing support
function SubStrategiesSection({
  mode,
  expandedSections,
  onToggleSection,
  editMode = false,
  onFieldChange,
  onRefresh: parentRefresh,
}: {
  mode: string;
  expandedSections: Set<string>;
  onToggleSection: (key: string) => void;
  editMode?: boolean;
  onFieldChange?: (path: string, value: any) => void;
  onRefresh?: () => void;
}) {
  const { subStrategies, isLoading, error, refresh } = useSubStrategies(mode, 'breakout');
  // Fetch strategy groups to get base_settings (including timeframe)
  const { groups: strategyGroups, isLoading: groupsLoading } = useStrategyGroups(mode);

  // Find the breakout group to get base_settings
  const breakoutGroup = strategyGroups.find(g => g.id === 'breakout');
  const baseSettings = breakoutGroup?.base_settings;

  // Default base settings for comparison
  const defaultBaseSettings = {
    timeframe: '3m',
    position_size_percent: 2.0,
    max_leverage: 10,
    max_positions: 1,
    min_volume_usdt: 1000000,
  };

  if (isLoading || groupsLoading) {
    return (
      <div className="space-y-4">
        <div className="border border-purple-500/30 rounded-lg overflow-hidden bg-purple-900/10">
          <div className="flex items-center gap-2 px-4 py-3 bg-purple-900/20 border-b border-purple-500/30">
            <Loader2 className="w-5 h-5 text-purple-400 animate-spin" />
            <span className="text-sm font-semibold text-purple-300">Loading sub-strategies...</span>
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="space-y-4">
        <div className="border border-red-500/30 rounded-lg overflow-hidden bg-red-900/10">
          <div className="flex items-center gap-2 px-4 py-3 bg-red-900/20">
            <AlertTriangle className="w-5 h-5 text-red-400" />
            <span className="text-sm text-red-400">Failed to load sub-strategies: {error}</span>
          </div>
        </div>
      </div>
    );
  }

  if (subStrategies.length === 0) {
    return null; // No sub-strategies to show
  }

  return (
    <div className="space-y-4">
      {/* Strategy Hierarchy Base Settings - Shows the 3m timeframe for Volume Imbalance */}
      {baseSettings && (
        <div className="border border-cyan-500/30 rounded-lg overflow-hidden bg-cyan-900/10">
          <div className="flex items-center gap-2 px-4 py-3 bg-cyan-900/20 border-b border-cyan-500/30">
            <Clock className="w-5 h-5 text-cyan-400" />
            <span className="text-sm font-semibold text-cyan-300">Volume Imbalance Strategy Settings</span>
            <span className="px-2 py-0.5 text-xs bg-cyan-500/20 text-cyan-300 rounded">
              Breakout Group
            </span>
          </div>
          <div className="p-3 space-y-2">
            {/* Timeframe - Most important for Volume Imbalance */}
            <div className="flex items-center justify-between py-1.5 border-b border-cyan-500/10">
              <span className="text-sm text-gray-300">Entry/Analysis Timeframe</span>
              <div className="flex items-center gap-2">
                <span className={`px-2 py-0.5 text-xs rounded ${
                  baseSettings.timeframe === defaultBaseSettings.timeframe
                    ? 'bg-green-500/20 text-green-400'
                    : 'bg-yellow-500/20 text-yellow-400'
                }`}>
                  {baseSettings.timeframe}
                </span>
                {baseSettings.timeframe !== defaultBaseSettings.timeframe && (
                  <span className="text-xs text-gray-500">
                    (default: {defaultBaseSettings.timeframe})
                  </span>
                )}
              </div>
            </div>
            {/* Position Size */}
            <div className="flex items-center justify-between py-1.5 border-b border-cyan-500/10">
              <span className="text-sm text-gray-300">Position Size</span>
              <span className="text-sm text-gray-400">{baseSettings.position_size_percent}%</span>
            </div>
            {/* Max Leverage */}
            <div className="flex items-center justify-between py-1.5 border-b border-cyan-500/10">
              <span className="text-sm text-gray-300">Max Leverage</span>
              <span className="text-sm text-gray-400">{baseSettings.max_leverage}x</span>
            </div>
            {/* Max Positions */}
            <div className="flex items-center justify-between py-1.5 border-b border-cyan-500/10">
              <span className="text-sm text-gray-300">Max Positions</span>
              <span className="text-sm text-gray-400">{baseSettings.max_positions}</span>
            </div>
            {/* Min Volume */}
            <div className="flex items-center justify-between py-1.5">
              <span className="text-sm text-gray-300">Min Volume</span>
              <span className="text-sm text-gray-400">{(baseSettings.min_volume_usdt / 1000000).toFixed(1)}M USDT</span>
            </div>
          </div>
        </div>
      )}

      {/* Sub-Strategies Section with clear header - matches Futures page UI */}
      <div className="border border-purple-500/30 rounded-lg overflow-hidden bg-purple-900/10">
        <div className="flex items-center gap-2 px-4 py-3 bg-purple-900/20 border-b border-purple-500/30">
          <Zap className="w-5 h-5 text-purple-400" />
          <span className="text-sm font-semibold text-purple-300">Sub-Strategies</span>
          <span className="px-2 py-0.5 text-xs bg-purple-500/20 text-purple-300 rounded">
            {subStrategies.length}
          </span>
          {editMode && (
            <span className="ml-2 text-xs text-purple-400">
              <Edit3 className="w-3 h-3 inline" /> Editing
            </span>
          )}
        </div>
        <div className="p-3 space-y-2">
          {subStrategies.map((subStrategy) => (
            <SubStrategyCollapsibleSection
              key={subStrategy.sub_strategy}
              subStrategy={subStrategy}
              mode={mode}
              expanded={expandedSections.has(`sub_strategy_${subStrategy.sub_strategy}`)}
              onToggle={() => onToggleSection(`sub_strategy_${subStrategy.sub_strategy}`)}
              onRefresh={refresh}
              editMode={editMode}
              onFieldChange={onFieldChange}
            />
          ))}
        </div>
      </div>

      {/* Divider between Sub-Strategies and Main Strategy Settings - matches Futures page UI */}
      <div className="flex items-center gap-3 py-2">
        <div className="flex-1 border-t border-gray-600"></div>
        <span className="text-xs text-gray-500 font-medium uppercase tracking-wider">Breakout Main Settings</span>
        <div className="flex-1 border-t border-gray-600"></div>
      </div>
    </div>
  );
}

// Story 11.43-11.46: Sub-Strategy Collapsible Section Component
// Follows the same pattern as CollapsibleStrategySection with field comparisons and editing support
function SubStrategyCollapsibleSection({
  subStrategy,
  mode,
  expanded,
  onToggle,
  onRefresh,
  editMode = false,
  onFieldChange,
}: {
  subStrategy: SubStrategySettings;
  mode: string;
  expanded: boolean;
  onToggle: () => void;
  onRefresh?: () => Promise<void>;
  editMode?: boolean;
  onFieldChange?: (path: string, value: any) => void;
}) {
  // Type guard for Volume Imbalance settings
  // Use sub_strategy field for identification (not id which is the DB UUID)
  const subStrategyName = subStrategy.sub_strategy;
  const isVolumeImbalance = subStrategyName === 'ravindra_volume_imbalance';
  const settings = subStrategy.settings as VolumeImbalanceSettings | undefined;
  const [isResetting, setIsResetting] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [resetError, setResetError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);

  // Track local edits for sub-strategy fields
  const [localEditedValues, setLocalEditedValues] = useState<Record<string, any>>({});

  // Use the update hook for API calls
  // IMPORTANT: Pass sub_strategy name (not id) to the hook - API expects strategy name
  const { mutate: updateSubStrategy, isLoading: isUpdating } = useUpdateSubStrategy(
    mode,
    'breakout',
    subStrategyName
  );

  // Get display name from sub_strategy field (not id)
  const displayName = subStrategyName === 'ravindra_volume_imbalance'
    ? 'Ravindra Volume Imbalance'
    : subStrategyName.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase());

  // Default settings for reset - backtested values (Dec 2025 - Jan 2026: 51 trades, 47.1% WR, +1147% return)
  const defaultSettings: VolumeImbalanceSettings = {
    enabled: true,
    risk_reward: { risk: 1, reward: 4, min_ratio: 3 },
    llm_validation_enabled: false,
    trailing_stop: {
      enabled: true,
      activation_profit_pct: 2.0,  // At 2:1 R:R
      initial_trail_pct: 0.0,      // Move to breakeven
      milestones: [
        { trigger_profit_pct: 2.0, trail_distance_pct: 0.0, label: 'BE' },   // At 2:1, move SL to entry (breakeven)
        { trigger_profit_pct: 3.0, trail_distance_pct: 1.0, label: '+1R' },  // At 3:1, lock in 1:1 profit
      ],
    },
    pattern_detection: {
      // Legacy fields (for backwards compatibility)
      min_volume_ratio: 3.0,
      consolidation_time_mins: 15,
      breakout_confirmation_candles: 1,
      max_pattern_age_mins: 60,
      require_htf_confirmation: false,
      htf_timeframe: '15m',
      // New backtested parameters (3m timeframe)
      reference_lookback_candles: 5,
      min_consolidation_candles: 1,
      max_consolidation_candles: 999,
      volume_spike_threshold: 3.0,
      breakout_volume_surge: 1.0,
      consolidation_range_tolerance: 0.01,
      entry_volume_vs_reference: 1.0,
      max_sl_percent: 1.5,
    },
    max_concurrent_patterns: 5,
    priority: 1,
    budget_allocation: {
      assigned_budget_usd: 100,
      max_concurrent_trades: 1,
      position_sizing: 'all_in',
      use_incremental_equity: true,
    },
  };

  // Handle local field change
  const handleLocalFieldChange = (path: string, value: any) => {
    setLocalEditedValues(prev => ({ ...prev, [path]: value }));
    // Also notify parent if provided
    onFieldChange?.(`sub_strategy.${subStrategyName}.${path}`, value);
  };

  // Handle save edits
  const handleSave = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (isSaving || isUpdating || Object.keys(localEditedValues).length === 0) return;

    setIsSaving(true);
    setSaveError(null);

    try {
      // Build the settings object from edits
      const updatedSettings: Partial<VolumeImbalanceSettings> = {
        ...(settings || defaultSettings),
      };

      // Apply edits to the settings object
      for (const [path, value] of Object.entries(localEditedValues)) {
        if (path === 'enabled') {
          // enabled is at the subStrategy level, not inside settings
          continue;
        }
        const parts = path.split('.');
        if (parts.length === 1) {
          (updatedSettings as any)[parts[0]] = value;
        } else if (parts.length === 2) {
          if (!(updatedSettings as any)[parts[0]]) {
            (updatedSettings as any)[parts[0]] = {};
          }
          (updatedSettings as any)[parts[0]][parts[1]] = value;
        }
      }

      // Send update to API
      await updateSubStrategy({
        enabled: localEditedValues['enabled'] !== undefined ? localEditedValues['enabled'] : subStrategy.enabled,
        settings: updatedSettings,
      });

      // Clear local edits and refresh
      setLocalEditedValues({});
      await onRefresh?.();
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : 'Failed to save sub-strategy');
    } finally {
      setIsSaving(false);
    }
  };

  // Handle reset to defaults
  const handleReset = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (isResetting || isUpdating) return;

    setIsResetting(true);
    setResetError(null);

    try {
      // Reset all settings to defaults - send the complete settings object
      await updateSubStrategy({
        enabled: defaultSettings.enabled,
        priority: defaultSettings.priority,
        settings: defaultSettings,
      });
      // Clear local edits and refresh data after reset
      setLocalEditedValues({});
      await onRefresh?.();
    } catch (err) {
      setResetError(err instanceof Error ? err.message : 'Failed to reset sub-strategy');
    } finally {
      setIsResetting(false);
    }
  };

  // Create field comparison list
  const fieldComparisons: { path: string; current: any; default: any; match: boolean; inputType: 'toggle' | 'number' | 'slider' }[] = [];

  if (settings) {
    // Basic fields
    fieldComparisons.push({
      path: 'enabled',
      current: subStrategy.enabled,
      default: defaultSettings.enabled,
      match: subStrategy.enabled === defaultSettings.enabled,
      inputType: 'toggle',
    });
    fieldComparisons.push({
      path: 'priority',
      current: settings.priority,
      default: defaultSettings.priority,
      match: settings.priority === defaultSettings.priority,
      inputType: 'number',
    });
    fieldComparisons.push({
      path: 'llm_validation_enabled',
      current: settings.llm_validation_enabled,
      default: defaultSettings.llm_validation_enabled,
      match: settings.llm_validation_enabled === defaultSettings.llm_validation_enabled,
      inputType: 'toggle',
    });
    fieldComparisons.push({
      path: 'max_concurrent_patterns',
      current: settings.max_concurrent_patterns,
      default: defaultSettings.max_concurrent_patterns,
      match: settings.max_concurrent_patterns === defaultSettings.max_concurrent_patterns,
      inputType: 'number',
    });

    // Risk/Reward fields
    if (settings.risk_reward) {
      fieldComparisons.push({
        path: 'risk_reward.risk',
        current: settings.risk_reward.risk,
        default: defaultSettings.risk_reward.risk,
        match: settings.risk_reward.risk === defaultSettings.risk_reward.risk,
        inputType: 'number',
      });
      fieldComparisons.push({
        path: 'risk_reward.reward',
        current: settings.risk_reward.reward,
        default: defaultSettings.risk_reward.reward,
        match: settings.risk_reward.reward === defaultSettings.risk_reward.reward,
        inputType: 'number',
      });
      fieldComparisons.push({
        path: 'risk_reward.min_ratio',
        current: settings.risk_reward.min_ratio,
        default: defaultSettings.risk_reward.min_ratio,
        match: settings.risk_reward.min_ratio === defaultSettings.risk_reward.min_ratio,
        inputType: 'number',
      });
    }

    // Pattern Detection fields - Updated with backtested parameters (Dec 2025 - Jan 2026)
    if (settings.pattern_detection) {
      // New backtested parameters (3m timeframe)
      fieldComparisons.push({
        path: 'pattern_detection.reference_lookback_candles',
        current: settings.pattern_detection.reference_lookback_candles ?? 5,
        default: defaultSettings.pattern_detection.reference_lookback_candles,
        match: (settings.pattern_detection.reference_lookback_candles ?? 5) === defaultSettings.pattern_detection.reference_lookback_candles,
        inputType: 'number',
      });
      fieldComparisons.push({
        path: 'pattern_detection.volume_spike_threshold',
        current: settings.pattern_detection.volume_spike_threshold ?? settings.pattern_detection.min_volume_ratio ?? 3.0,
        default: defaultSettings.pattern_detection.volume_spike_threshold,
        match: (settings.pattern_detection.volume_spike_threshold ?? settings.pattern_detection.min_volume_ratio ?? 3.0) === defaultSettings.pattern_detection.volume_spike_threshold,
        inputType: 'slider',
      });
      fieldComparisons.push({
        path: 'pattern_detection.min_consolidation_candles',
        current: settings.pattern_detection.min_consolidation_candles ?? 1,
        default: defaultSettings.pattern_detection.min_consolidation_candles,
        match: (settings.pattern_detection.min_consolidation_candles ?? 1) === defaultSettings.pattern_detection.min_consolidation_candles,
        inputType: 'number',
      });
      fieldComparisons.push({
        path: 'pattern_detection.max_consolidation_candles',
        current: settings.pattern_detection.max_consolidation_candles ?? 999,
        default: defaultSettings.pattern_detection.max_consolidation_candles,
        match: (settings.pattern_detection.max_consolidation_candles ?? 999) === defaultSettings.pattern_detection.max_consolidation_candles,
        inputType: 'number',
      });
      fieldComparisons.push({
        path: 'pattern_detection.consolidation_range_tolerance',
        current: settings.pattern_detection.consolidation_range_tolerance ?? 0.01,
        default: defaultSettings.pattern_detection.consolidation_range_tolerance,
        match: (settings.pattern_detection.consolidation_range_tolerance ?? 0.01) === defaultSettings.pattern_detection.consolidation_range_tolerance,
        inputType: 'slider',
      });
      fieldComparisons.push({
        path: 'pattern_detection.breakout_volume_surge',
        current: settings.pattern_detection.breakout_volume_surge ?? 1.0,
        default: defaultSettings.pattern_detection.breakout_volume_surge,
        match: (settings.pattern_detection.breakout_volume_surge ?? 1.0) === defaultSettings.pattern_detection.breakout_volume_surge,
        inputType: 'slider',
      });
      fieldComparisons.push({
        path: 'pattern_detection.entry_volume_vs_reference',
        current: settings.pattern_detection.entry_volume_vs_reference ?? 1.0,
        default: defaultSettings.pattern_detection.entry_volume_vs_reference,
        match: (settings.pattern_detection.entry_volume_vs_reference ?? 1.0) === defaultSettings.pattern_detection.entry_volume_vs_reference,
        inputType: 'slider',
      });
      fieldComparisons.push({
        path: 'pattern_detection.max_sl_percent',
        current: settings.pattern_detection.max_sl_percent ?? 1.5,
        default: defaultSettings.pattern_detection.max_sl_percent,
        match: (settings.pattern_detection.max_sl_percent ?? 1.5) === defaultSettings.pattern_detection.max_sl_percent,
        inputType: 'slider',
      });
      fieldComparisons.push({
        path: 'pattern_detection.require_htf_confirmation',
        current: settings.pattern_detection.require_htf_confirmation ?? false,
        default: defaultSettings.pattern_detection.require_htf_confirmation,
        match: (settings.pattern_detection.require_htf_confirmation ?? false) === defaultSettings.pattern_detection.require_htf_confirmation,
        inputType: 'toggle',
      });
      // Additional pattern detection fields
      fieldComparisons.push({
        path: 'pattern_detection.breakout_confirmation_candles',
        current: settings.pattern_detection.breakout_confirmation_candles ?? 1,
        default: defaultSettings.pattern_detection.breakout_confirmation_candles ?? 1,
        match: (settings.pattern_detection.breakout_confirmation_candles ?? 1) === (defaultSettings.pattern_detection.breakout_confirmation_candles ?? 1),
        inputType: 'number',
      });
      fieldComparisons.push({
        path: 'pattern_detection.max_pattern_age_mins',
        current: settings.pattern_detection.max_pattern_age_mins ?? 60,
        default: defaultSettings.pattern_detection.max_pattern_age_mins ?? 60,
        match: (settings.pattern_detection.max_pattern_age_mins ?? 60) === (defaultSettings.pattern_detection.max_pattern_age_mins ?? 60),
        inputType: 'number',
      });
      fieldComparisons.push({
        path: 'pattern_detection.htf_timeframe',
        current: settings.pattern_detection.htf_timeframe ?? '15m',
        default: defaultSettings.pattern_detection.htf_timeframe ?? '15m',
        match: (settings.pattern_detection.htf_timeframe ?? '15m') === (defaultSettings.pattern_detection.htf_timeframe ?? '15m'),
        inputType: 'number', // Will display as text
      });
    }

    // Budget Allocation fields - Per-strategy capital management
    if (settings.budget_allocation) {
      fieldComparisons.push({
        path: 'budget_allocation.assigned_budget_usd',
        current: settings.budget_allocation.assigned_budget_usd ?? 100,
        default: defaultSettings.budget_allocation?.assigned_budget_usd ?? 100,
        match: (settings.budget_allocation.assigned_budget_usd ?? 100) === (defaultSettings.budget_allocation?.assigned_budget_usd ?? 100),
        inputType: 'number',
      });
      fieldComparisons.push({
        path: 'budget_allocation.max_concurrent_trades',
        current: settings.budget_allocation.max_concurrent_trades ?? 1,
        default: defaultSettings.budget_allocation?.max_concurrent_trades ?? 1,
        match: (settings.budget_allocation.max_concurrent_trades ?? 1) === (defaultSettings.budget_allocation?.max_concurrent_trades ?? 1),
        inputType: 'number',
      });
      fieldComparisons.push({
        path: 'budget_allocation.use_incremental_equity',
        current: settings.budget_allocation.use_incremental_equity ?? true,
        default: defaultSettings.budget_allocation?.use_incremental_equity ?? true,
        match: (settings.budget_allocation.use_incremental_equity ?? true) === (defaultSettings.budget_allocation?.use_incremental_equity ?? true),
        inputType: 'toggle',
      });
      fieldComparisons.push({
        path: 'budget_allocation.position_sizing',
        current: settings.budget_allocation.position_sizing ?? 'all_in',
        default: defaultSettings.budget_allocation?.position_sizing ?? 'all_in',
        match: (settings.budget_allocation.position_sizing ?? 'all_in') === (defaultSettings.budget_allocation?.position_sizing ?? 'all_in'),
        inputType: 'number', // Will display as text
      });
    } else {
      // Show budget allocation fields even if not set (using defaults)
      fieldComparisons.push({
        path: 'budget_allocation.assigned_budget_usd',
        current: 100,
        default: defaultSettings.budget_allocation?.assigned_budget_usd ?? 100,
        match: true,
        inputType: 'number',
      });
      fieldComparisons.push({
        path: 'budget_allocation.max_concurrent_trades',
        current: 1,
        default: defaultSettings.budget_allocation?.max_concurrent_trades ?? 1,
        match: true,
        inputType: 'number',
      });
      fieldComparisons.push({
        path: 'budget_allocation.use_incremental_equity',
        current: true,
        default: defaultSettings.budget_allocation?.use_incremental_equity ?? true,
        match: true,
        inputType: 'toggle',
      });
      fieldComparisons.push({
        path: 'budget_allocation.position_sizing',
        current: 'all_in',
        default: defaultSettings.budget_allocation?.position_sizing ?? 'all_in',
        match: true,
        inputType: 'number', // Will display as text
      });
    }

    // Trailing Stop fields
    if (settings.trailing_stop) {
      fieldComparisons.push({
        path: 'trailing_stop.enabled',
        current: settings.trailing_stop.enabled,
        default: defaultSettings.trailing_stop.enabled,
        match: settings.trailing_stop.enabled === defaultSettings.trailing_stop.enabled,
        inputType: 'toggle',
      });
      fieldComparisons.push({
        path: 'trailing_stop.activation_profit_pct',
        current: settings.trailing_stop.activation_profit_pct,
        default: defaultSettings.trailing_stop.activation_profit_pct,
        match: settings.trailing_stop.activation_profit_pct === defaultSettings.trailing_stop.activation_profit_pct,
        inputType: 'slider',
      });
      fieldComparisons.push({
        path: 'trailing_stop.initial_trail_pct',
        current: settings.trailing_stop.initial_trail_pct,
        default: defaultSettings.trailing_stop.initial_trail_pct,
        match: settings.trailing_stop.initial_trail_pct === defaultSettings.trailing_stop.initial_trail_pct,
        inputType: 'slider',
      });
      // Trailing Stop Milestones - each milestone has trigger_profit_pct, trail_distance_pct, label
      const currentMilestones = settings.trailing_stop.milestones || [];
      const defaultMilestones = defaultSettings.trailing_stop.milestones || [];
      const maxMilestones = Math.max(currentMilestones.length, defaultMilestones.length);
      for (let i = 0; i < maxMilestones; i++) {
        const currentMilestone = currentMilestones[i];
        const defaultMilestone = defaultMilestones[i];
        const milestoneLabel = currentMilestone?.label || defaultMilestone?.label || `Milestone ${i + 1}`;

        fieldComparisons.push({
          path: `trailing_stop.milestones[${i}].trigger_profit_pct`,
          current: currentMilestone?.trigger_profit_pct ?? 'N/A',
          default: defaultMilestone?.trigger_profit_pct ?? 'N/A',
          match: currentMilestone?.trigger_profit_pct === defaultMilestone?.trigger_profit_pct,
          inputType: 'number',
        });
        fieldComparisons.push({
          path: `trailing_stop.milestones[${i}].trail_distance_pct`,
          current: currentMilestone?.trail_distance_pct ?? 'N/A',
          default: defaultMilestone?.trail_distance_pct ?? 'N/A',
          match: currentMilestone?.trail_distance_pct === defaultMilestone?.trail_distance_pct,
          inputType: 'number',
        });
        fieldComparisons.push({
          path: `trailing_stop.milestones[${i}].label`,
          current: currentMilestone?.label ?? 'N/A',
          default: defaultMilestone?.label ?? 'N/A',
          match: currentMilestone?.label === defaultMilestone?.label,
          inputType: 'number', // Will display as text
        });
      }
    }
  }

  const matchingFields = fieldComparisons.filter(f => f.match).length;
  const totalFields = fieldComparisons.length;
  const differentFields = totalFields - matchingFields;
  const allMatch = differentFields === 0;
  const hasLocalEdits = Object.keys(localEditedValues).length > 0;

  // Render editable field input based on field type
  const renderEditableField = (field: { path: string; current: any; default: any; match: boolean; inputType: 'toggle' | 'number' | 'slider' }) => {
    const fieldName = field.path.split('.').pop() || field.path;
    const currentValue = localEditedValues[field.path] ?? field.current;
    const isEdited = localEditedValues[field.path] !== undefined;

    switch (field.inputType) {
      case 'toggle':
        return (
          <StrategyToggleInput
            label={fieldName.replace(/_/g, ' ')}
            checked={currentValue as boolean | null | undefined}
            defaultChecked={field.default as boolean | null | undefined}
            onChange={(value) => handleLocalFieldChange(field.path, value)}
            isEdited={isEdited}
          />
        );

      case 'slider':
        return (
          <StrategySliderInput
            label={fieldName.replace(/_/g, ' ')}
            value={currentValue as number | null | undefined}
            defaultValue={field.default as number | null | undefined}
            onChange={(value) => handleLocalFieldChange(field.path, value)}
            min={0}
            max={field.path.includes('pct') ? 10 : 100}
            step={field.path.includes('pct') ? 0.1 : 0.5}
            unit={field.path.includes('pct') ? '%' : ''}
            isEdited={isEdited}
          />
        );

      case 'number':
      default:
        return (
          <StrategyNumberInput
            label={fieldName.replace(/_/g, ' ')}
            value={currentValue as number | null | undefined}
            defaultValue={field.default as number | null | undefined}
            onChange={(value) => handleLocalFieldChange(field.path, value)}
            min={0}
            max={field.path.includes('mins') ? 120 : 100}
            step={1}
            unit={field.path.includes('mins') ? 'min' : ''}
            isEdited={isEdited}
          />
        );
    }
  };

  return (
    <div className="border border-gray-700 rounded-lg overflow-hidden">
      {/* Section Header - follows CollapsibleStrategySection pattern */}
      <button
        type="button"
        onClick={onToggle}
        className={`w-full flex items-center justify-between px-3 py-2 transition-colors ${
          hasLocalEdits
            ? 'bg-orange-900/30 hover:bg-orange-900/40'
            : allMatch
            ? 'bg-green-900/20 hover:bg-green-900/30'
            : 'bg-orange-900/20 hover:bg-orange-900/30'
        }`}
      >
        <div className="flex items-center gap-2">
          <span className="text-purple-400">
            <Zap className="w-4 h-4" />
          </span>
          <span className="font-medium text-gray-200 text-sm">{displayName}</span>
          <span className="text-xs text-gray-500">
            ({matchingFields}/{totalFields})
          </span>
          {/* Enable/Disable Badge */}
          <span className={`px-2 py-0.5 text-xs rounded ${
            subStrategy.enabled
              ? 'bg-green-500/20 text-green-400'
              : 'bg-gray-600/20 text-gray-500'
          }`}>
            {subStrategy.enabled ? 'ON' : 'OFF'}
          </span>
          {editMode && (
            <span className="text-xs text-purple-400 ml-1">
              <Edit3 className="w-3 h-3 inline" />
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          {/* Save Button - only show when in edit mode and has local edits */}
          {editMode && hasLocalEdits && (
            <div
              onClick={handleSave}
              className={`
                flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium transition-colors cursor-pointer
                bg-green-600/20 text-green-400 hover:bg-green-600/30
                ${isSaving ? 'opacity-50 cursor-wait' : ''}
              `}
              title="Save changes"
            >
              {isSaving ? (
                <Loader2 className="w-3 h-3 animate-spin" />
              ) : (
                <Save className="w-3 h-3" />
              )}
              Save
            </div>
          )}
          {/* Reset Button - only show when there are differences */}
          {!allMatch && (
            <div
              onClick={handleReset}
              className={`
                flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium transition-colors cursor-pointer
                bg-blue-600/20 text-blue-400 hover:bg-blue-600/30
                ${isResetting ? 'opacity-50 cursor-wait' : ''}
              `}
              title="Reset to defaults"
            >
              {isResetting ? (
                <Loader2 className="w-3 h-3 animate-spin" />
              ) : (
                <RefreshCw className="w-3 h-3" />
              )}
              Reset
            </div>
          )}
          {hasLocalEdits ? (
            <span className="text-xs px-2 py-0.5 bg-orange-500/30 text-orange-300 rounded font-medium">
              Modified
            </span>
          ) : allMatch ? (
            <span className="text-xs px-2 py-0.5 bg-green-500/20 text-green-400 rounded">
              Match
            </span>
          ) : (
            <span className="text-xs px-2 py-0.5 bg-orange-500/20 text-orange-400 rounded">
              {differentFields} diff
            </span>
          )}
          {expanded ? (
            <ChevronDown className="w-4 h-4 text-gray-400" />
          ) : (
            <ChevronRight className="w-4 h-4 text-gray-400" />
          )}
        </div>
      </button>

      {/* Error Messages */}
      {(resetError || saveError) && (
        <div className="px-3 py-2 bg-red-900/20 border-t border-red-500/30 flex items-center gap-2">
          <AlertTriangle className="w-3 h-3 text-red-400" />
          <span className="text-xs text-red-400">{resetError || saveError}</span>
        </div>
      )}

      {/* Section Content - Editable or Read-only */}
      {expanded && (
        <div className="bg-gray-900/30 border-t border-gray-700 p-3">
          {isVolumeImbalance && (
            <div className="mb-3 text-xs text-gray-400 italic">
              3-step pattern: Accumulation → Consolidation → Breakout
            </div>
          )}

          {editMode ? (
            // Editable Mode - Show proper UI controls
            <div className="grid gap-3">
              {fieldComparisons.map((field) => (
                <div key={field.path} className="relative">
                  {renderEditableField(field)}
                  {/* Show default value hint when different */}
                  {!field.match && localEditedValues[field.path] === undefined && (
                    <div className="mt-0.5 text-xs text-blue-400/70">
                      Default: {formatValue(field.default)}
                    </div>
                  )}
                </div>
              ))}
            </div>
          ) : (
            // Read-only Mode - Show comparison table
            <table className="w-full text-xs">
              <thead className="bg-gray-800/50">
                <tr className="text-gray-400 border-b border-gray-700/50">
                  <th className="text-left p-2 pl-3 font-medium">Setting</th>
                  <th className="text-left p-2 font-medium">Current</th>
                  <th className="text-left p-2 font-medium">Default</th>
                  <th className="text-left p-2 pr-3 font-medium">Status</th>
                </tr>
              </thead>
              <tbody>
                {fieldComparisons.map((field) => (
                  <tr
                    key={field.path}
                    className={`border-b border-gray-700/30 last:border-0 ${
                      field.match ? 'bg-green-900/5' : 'bg-orange-900/10'
                    }`}
                  >
                    <td className="p-2 pl-3 font-mono text-gray-300">
                      {field.path.split('.').pop() || field.path}
                    </td>
                    <td className={`p-2 font-mono ${field.match ? 'text-green-400' : 'text-orange-400'}`}>
                      {formatValue(field.current)}
                    </td>
                    <td className="p-2 font-mono text-blue-400">
                      {formatValue(field.default)}
                    </td>
                    <td className="p-2 pr-3">
                      {field.match ? (
                        <span className="text-green-400 flex items-center gap-1">
                          <CheckCircle2 className="w-3 h-3" />
                        </span>
                      ) : (
                        <span className="text-orange-400 flex items-center gap-1">
                          <XCircle className="w-3 h-3" />
                        </span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
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
  // Strategy resets (Story 11.34)
  onResetStrategy,
  onResetAllStrategiesInMode,
  // Other settings resets
  onResetAllOther,
  onResetCircuitBreaker,
  onResetLLMConfig,
  onResetCapitalAllocation,
  onResetGlobalTrading,
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

  // Strategy states (Story 11.34)
  const [selectedStrategies, setSelectedStrategies] = useState<Record<string, StrategyName | null>>({});
  const [resettingStrategies, setResettingStrategies] = useState<Set<string>>(new Set());

  // Edited strategy values state - keyed by "mode:strategy"
  const [editedStrategyValues, setEditedStrategyValues] = useState<Record<string, Record<string, any>>>({});
  const [savingStrategies, setSavingStrategies] = useState<Set<string>>(new Set());

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

  // Load strategy comparisons for a single mode (Story 11.34)
  const loadStrategyComparisons = useCallback(async (mode: string): Promise<StrategyComparisonResult[]> => {
    const results: StrategyComparisonResult[] = [];

    for (const strategy of ALL_STRATEGIES) {
      try {
        // Get strategy comparison from API
        const comparison = await modeStrategyApi.compareModeStrategy(mode as ModeName, strategy);

        // Map API response to internal format
        const mapField = (d: { path: string; current: unknown; default: unknown; match?: boolean }) => ({
          path: d.path,
          current: d.current,
          default: d.default,
          match: d.match ?? false,
        });

        results.push({
          strategy,
          strategyName: STRATEGY_DISPLAY_NAMES[strategy] || strategy,
          enabled: comparison.enabled ?? false,
          allMatch: comparison.all_match ?? true,
          totalFields: comparison.total_fields ?? 0,
          matchingFields: comparison.matching_fields ?? comparison.total_fields ?? 0,
          differentFields: comparison.differences?.length ?? 0,
          differences: comparison.differences?.map(mapField) || [],
          allValues: comparison.all_values?.map(mapField) || [],
        });
      } catch (err: any) {
        console.error(`[SettingsComparison] Error loading strategy ${strategy} for mode ${mode}:`, err);
        // Add placeholder for failed strategies
        results.push({
          strategy,
          strategyName: STRATEGY_DISPLAY_NAMES[strategy] || strategy,
          enabled: false,
          allMatch: true,
          totalFields: 0,
          matchingFields: 0,
          differentFields: 0,
          differences: [],
          allValues: [],
          error: err?.response?.status === 404 ? 'Not configured' : 'Failed to load',
        });
      }
    }

    return results;
  }, []);

  const loadModeComparisons = useCallback(async () => {
    const results: ModeComparisonResult[] = [];

    for (const mode of modes) {
      try {
        // Load strategy comparisons for this mode - now the primary data source
        const strategies = await loadStrategyComparisons(mode);
        const strategiesAllMatch = strategies.every(s => s.allMatch && !s.error);
        const totalStrategyDifferences = strategies.reduce((sum, s) => sum + s.differentFields, 0);
        const totalStrategyFields = strategies.reduce((sum, s) => sum + s.totalFields, 0);

        results.push({
          mode,
          modeName: MODE_DISPLAY_NAMES[mode] || mode,
          allMatch: strategiesAllMatch,
          totalChanges: totalStrategyDifferences,
          totalFields: totalStrategyFields,
          groups: [], // No longer using mode-level groups
          // Strategies are now the primary data source
          strategies,
          strategiesAllMatch,
          totalStrategyDifferences,
        });
      } catch (err: any) {
        console.error(`[SettingsComparison] Error loading mode ${mode}:`, err);
        // Handle all errors gracefully - mark as "not configured" rather than failing entirely
        results.push({
          mode,
          modeName: MODE_DISPLAY_NAMES[mode] || mode,
          allMatch: false,
          totalChanges: -1,
          totalFields: 0,
          groups: [],
          configNotFound: true,
        });
      }
    }

    return results;
  }, [modes, loadStrategyComparisons]);

  const loadOtherSettings = useCallback(async () => {
    const results: OtherSettingComparison[] = [];

    // Circuit Breaker
    try {
      const preview = (await loadCircuitBreakerDefaults(true)) as ConfigResetPreview;
      const allFields =
        preview.all_values ||
        (preview.differences || []).map((d) => ({
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
      console.error('[SettingsComparison] Error loading circuit breaker defaults:', err);
      // Handle 404 as "not configured", other errors as partial failure
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

    // LLM Config
    try {
      const preview = (await loadLLMConfigDefaults(true)) as ConfigResetPreview;
      const allFields =
        preview.all_values ||
        (preview.differences || []).map((d) => ({
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
      console.error('[SettingsComparison] Error loading LLM config defaults:', err);
      // Handle 404 as "not configured", other errors as partial failure
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

    // Capital Allocation
    try {
      const preview = (await loadCapitalAllocationDefaults(true)) as ConfigResetPreview;
      const allFields =
        preview.all_values ||
        (preview.differences || []).map((d) => ({
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
      console.error('[SettingsComparison] Error loading capital allocation defaults:', err);
      // Handle 404 as "not configured", other errors as partial failure
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

    // Global Trading (includes timezone)
    try {
      const preview = (await loadGlobalTradingDefaults(true)) as ConfigResetPreview;
      const allFields =
        preview.all_values ||
        (preview.differences || []).map((d) => ({
          path: d.path,
          current: d.current,
          default: d.default,
          match: false,
          risk_level: d.risk_level,
        }));

      results.push({
        settingType: 'global_trading',
        settingName: 'Global Trading & Timezone',
        icon: <Globe className="w-6 h-6 text-blue-400" />,
        allMatch: preview.all_match,
        totalChanges: preview.total_changes,
        totalFields: allFields.length,
        fields: allFields,
        isAdmin: preview.is_admin,
        rawData: preview,
      });
    } catch (err: any) {
      console.error('[SettingsComparison] Error loading global trading defaults:', err);
      console.error('[SettingsComparison] Error details:', err?.response?.data || err?.message);
      results.push({
        settingType: 'global_trading',
        settingName: 'Global Trading & Timezone',
        icon: <Globe className="w-6 h-6 text-blue-400" />,
        allMatch: false,
        totalChanges: -1,
        totalFields: 0,
        fields: [],
        configNotFound: true,
      });
    }

    return results;
  }, []);

  const loadAllComparisons = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      const [modeResults, otherResults] = await Promise.all([
        loadModeComparisons(),
        loadOtherSettings(),
      ]);

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
    } catch (err: any) {
      console.error('[SettingsComparison] Failed to load comparisons:', err);
      setError(err?.response?.data?.error || err?.message || 'Failed to load settings comparison');
    } finally {
      setLoading(false);
    }
  }, [loadModeComparisons, loadOtherSettings]);

  useEffect(() => {
    loadAllComparisons();
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

    // Special handling for timezone - auto-update timezone_offset based on selection
    // path format: "global_trading.timezone" or just "timezone"
    const fieldName = path.split('.').pop() || path;
    if (settingType === 'global_trading' && fieldName === 'timezone') {
      // Find the matching preset to get the offset
      const preset = TIMEZONE_PRESETS.find(p => p.tz_identifier === value);
      const newOffset = preset?.gmt_offset || '+00:00';

      // Build the offset path to match the field path format
      const offsetPath = path.replace('timezone', 'timezone_offset');

      setEditedOtherValues((prev) => ({
        ...prev,
        [settingType]: {
          ...(prev[settingType] || {}),
          [path]: value,
          [offsetPath]: newOffset,
        },
      }));
      return;
    }

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

  // ==================== STRATEGY HANDLERS (Story 11.34) ====================

  // Select a strategy within a mode
  const handleSelectStrategy = useCallback((mode: string, strategy: StrategyName) => {
    setSelectedStrategies(prev => ({
      ...prev,
      [mode]: prev[mode] === strategy ? null : strategy,
    }));
  }, []);

  // Reset a single strategy
  const handleResetStrategy = useCallback(async (mode: string, strategy: StrategyName) => {
    const key = `${mode}:${strategy}`;
    setResettingStrategies(prev => new Set(prev).add(key));

    try {
      if (onResetStrategy) {
        await onResetStrategy(mode, strategy);
      } else {
        // Use API directly if no handler provided
        await modeStrategyApi.resetModeStrategy(mode as ModeName, strategy);
      }
      // Reload comparisons after reset
      loadAllComparisons();
    } catch (err: any) {
      console.error(`Failed to reset strategy ${strategy} for mode ${mode}:`, err);
      // Show error to user via the component's error state
      setError(`Failed to reset ${STRATEGY_DISPLAY_NAMES[strategy]} strategy: ${err?.response?.data?.error || err?.message || 'Unknown error'}`);
    } finally {
      setResettingStrategies(prev => {
        const next = new Set(prev);
        next.delete(key);
        return next;
      });
    }
  }, [onResetStrategy, loadAllComparisons]);

  // Reset all strategies in a mode
  const handleResetAllStrategiesInMode = useCallback(async (mode: string) => {
    // Mark all strategies as resetting
    const keys = ALL_STRATEGIES.map(s => `${mode}:${s}`);
    setResettingStrategies(prev => {
      const next = new Set(prev);
      keys.forEach(k => next.add(k));
      return next;
    });

    try {
      if (onResetAllStrategiesInMode) {
        await onResetAllStrategiesInMode(mode);
      } else {
        // Use API directly if no handler provided
        await modeStrategyApi.resetAllModeStrategies(mode as ModeName);
      }
      // Reload comparisons after reset
      loadAllComparisons();
    } catch (err: any) {
      console.error(`Failed to reset all strategies for mode ${mode}:`, err);
      // Show error to user via the component's error state
      setError(`Failed to reset all strategies for ${MODE_DISPLAY_NAMES[mode] || mode} mode: ${err?.response?.data?.error || err?.message || 'Unknown error'}`);
    } finally {
      setResettingStrategies(prev => {
        const next = new Set(prev);
        keys.forEach(k => next.delete(k));
        return next;
      });
    }
  }, [onResetAllStrategiesInMode, loadAllComparisons]);

  // Get resetting state for a specific mode
  const getModeResettingStrategies = useCallback((mode: string): Set<string> => {
    const modeStrategies = new Set<string>();
    resettingStrategies.forEach(key => {
      if (key.startsWith(`${mode}:`)) {
        modeStrategies.add(key.split(':')[1]);
      }
    });
    return modeStrategies;
  }, [resettingStrategies]);

  // ==================== STRATEGY EDITING HANDLERS ====================

  // Handle strategy field change
  const handleStrategyFieldChange = useCallback((mode: string, strategy: StrategyName, path: string, value: any) => {
    const key = `${mode}:${strategy}`;
    setEditedStrategyValues(prev => ({
      ...prev,
      [key]: {
        ...(prev[key] || {}),
        [path]: value,
      },
    }));
  }, []);

  // Save strategy settings
  const handleSaveStrategy = useCallback(async (mode: string, strategy: StrategyName) => {
    const key = `${mode}:${strategy}`;
    const changes = editedStrategyValues[key];
    if (!changes || Object.keys(changes).length === 0) return;

    setSavingStrategies(prev => new Set(prev).add(key));

    try {
      // Build the update payload from edited values
      // Group changes by their section prefix
      const updatePayload: Record<string, any> = {};

      Object.entries(changes).forEach(([path, value]) => {
        // Handle nested paths like "sltp.sl_percent" or "confidence.min_confidence"
        const parts = path.split('.');
        if (parts.length === 2) {
          // Nested field like "sltp.sl_percent"
          const [section, field] = parts;
          if (!updatePayload[section]) {
            updatePayload[section] = {};
          }
          updatePayload[section][field] = value;
        } else {
          // Top-level field like "enabled", "leverage", "priority"
          updatePayload[path] = value;
        }
      });

      // Call the API to save the strategy
      await modeStrategyApi.updateModeStrategy(mode as ModeName, strategy, updatePayload);

      // Clear the edited values for this strategy
      setEditedStrategyValues(prev => {
        const next = { ...prev };
        delete next[key];
        return next;
      });

      // Reload comparisons to reflect changes
      loadAllComparisons();
    } catch (err: any) {
      console.error(`Failed to save strategy ${strategy} for mode ${mode}:`, err);
      setError(`Failed to save ${STRATEGY_DISPLAY_NAMES[strategy]} strategy: ${err?.response?.data?.error || err?.message || 'Unknown error'}`);
    } finally {
      setSavingStrategies(prev => {
        const next = new Set(prev);
        next.delete(key);
        return next;
      });
    }
  }, [editedStrategyValues, loadAllComparisons]);

  // Get edited values for a specific mode+strategy
  const getStrategyEditedValues = useCallback((mode: string, strategy: StrategyName): Record<string, any> => {
    const key = `${mode}:${strategy}`;
    return editedStrategyValues[key] || {};
  }, [editedStrategyValues]);

  // Check if a strategy is being saved
  const isStrategySaving = useCallback((mode: string, strategy: StrategyName): boolean => {
    const key = `${mode}:${strategy}`;
    return savingStrategies.has(key);
  }, [savingStrategies]);

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
            {modeComparisons.map((comparison) => {
              const selectedStrategy = selectedStrategies[comparison.mode];
              return (
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
                  // Strategy props (Story 11.34)
                  selectedStrategy={selectedStrategy || null}
                  onSelectStrategy={(strategy) => handleSelectStrategy(comparison.mode, strategy)}
                  onResetStrategy={(strategy) => handleResetStrategy(comparison.mode, strategy)}
                  onResetAllStrategies={() => handleResetAllStrategiesInMode(comparison.mode)}
                  resettingStrategies={getModeResettingStrategies(comparison.mode)}
                  // Strategy editing props
                  editedStrategyValues={selectedStrategy ? getStrategyEditedValues(comparison.mode, selectedStrategy) : undefined}
                  onStrategyFieldChange={selectedStrategy ? (path, value) => handleStrategyFieldChange(comparison.mode, selectedStrategy, path, value) : undefined}
                  onSaveStrategy={selectedStrategy ? () => handleSaveStrategy(comparison.mode, selectedStrategy) : undefined}
                  isSavingStrategy={selectedStrategy ? isStrategySaving(comparison.mode, selectedStrategy) : false}
                />
              );
            })}
          </div>
        </div>
      </SectionHeader>

      {/* ==================== SECTION 2: OTHER SETTINGS ==================== */}
      <SectionHeader
        title="Other Settings"
        icon={<Settings className="w-6 h-6 text-purple-400" />}
        isExpanded={expandedSections.has('other')}
        onToggle={() => toggleSection('other')}
        allMatch={otherStats.allMatch && otherStats.notConfigured === 0}
        totalItems={otherSettings.length}
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
                  : setting.settingType === 'global_trading'
                  ? onResetGlobalTrading
                  : undefined;

              // Show all global_trading fields including timezone (same pattern as other fields)
              const displaySetting = setting;

              return (
                <OtherSettingCard
                  key={setting.settingType}
                  setting={displaySetting}
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
