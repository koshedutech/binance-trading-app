// Decision Engine Settings Component
// Epic 11: Position Decision Engine - Story 11.24: Decision Engine Settings UI

import { useState, useEffect, useCallback } from 'react';
import {
  ChevronDown,
  ChevronUp,
  CheckCircle2,
  XCircle,
  AlertTriangle,
  Loader2,
  RefreshCw,
  Settings,
  Zap,
  TrendingUp,
  BarChart2,
  Target,
} from 'lucide-react';
import {
  getDecisionEngineComparison,
  resetDecisionEngineSettings,
  resetStrategySettings,
} from '../api/decisionEngine';
import type {
  StrategyName,
  DecisionEngineComparisonResponse,
  StrategyComparison,
  GroupComparison,
  FieldComparison,
} from '../types/decision-engine';
import { STRATEGY_DISPLAY_NAMES, GROUP_DISPLAY_NAMES, type SettingsGroupName } from '../types/decision-engine';

// ==================== INTERFACES ====================

interface DecisionEngineSettingsProps {
  isAdmin?: boolean;
  onResetStrategy?: (strategy: StrategyName) => void;
  onResetAll?: () => void;
}

// ==================== CONSTANTS ====================

const STRATEGY_ICONS: Record<StrategyName, React.ReactNode> = {
  trend_following: <TrendingUp className="w-5 h-5 text-green-400" />,
  mean_reversion: <BarChart2 className="w-5 h-5 text-blue-400" />,
  breakout: <Zap className="w-5 h-5 text-yellow-400" />,
  range_trading: <Target className="w-5 h-5 text-purple-400" />,
};

// ==================== UTILITY FUNCTIONS ====================

const formatValue = (value: unknown): string => {
  if (value === null || value === undefined) return 'N/A';
  if (typeof value === 'boolean') return value ? 'Yes' : 'No';
  if (typeof value === 'number') {
    if (Number.isInteger(value)) return value.toLocaleString();
    return value.toFixed(2).replace(/\.?0+$/, '');
  }
  if (Array.isArray(value)) return value.length > 0 ? value.join(', ') : '[]';
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
};

// ==================== SUB-COMPONENTS ====================

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

// Group Header (within a strategy)
function GroupHeader({
  groupName,
  group,
  isExpanded,
  onToggle,
}: {
  groupName: string;
  group: GroupComparison;
  isExpanded: boolean;
  onToggle: () => void;
}) {
  const displayName = GROUP_DISPLAY_NAMES[groupName as SettingsGroupName] || groupName;

  return (
    <button
      onClick={onToggle}
      className={`w-full p-3 flex items-center justify-between transition-colors ${
        group.all_match ? 'bg-gray-800/30 hover:bg-gray-800/50' : 'bg-orange-900/10 hover:bg-orange-900/20'
      }`}
    >
      <div className="flex items-center gap-2">
        {group.all_match ? (
          <CheckCircle2 className="w-4 h-4 text-green-400" />
        ) : (
          <AlertTriangle className="w-4 h-4 text-orange-400" />
        )}
        <span className="text-sm font-medium text-white">{displayName}</span>
        <span className="text-xs text-gray-500">({group.total_fields} fields)</span>
      </div>
      <div className="flex items-center gap-2">
        {!group.all_match && (
          <span className="text-xs text-orange-400">
            {group.total_fields - group.matching_fields} differences
          </span>
        )}
        {isExpanded ? (
          <ChevronUp className="w-4 h-4 text-gray-400" />
        ) : (
          <ChevronDown className="w-4 h-4 text-gray-400" />
        )}
      </div>
    </button>
  );
}

// Field Row Component
function FieldRow({ field }: { field: FieldComparison }) {
  const pathParts = field.path.split('.');
  const fieldName = pathParts[pathParts.length - 1];
  const displayName = fieldName
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase());

  return (
    <div
      className={`px-4 py-2 flex items-center justify-between ${
        field.match ? 'bg-gray-800/20' : 'bg-orange-900/10'
      }`}
    >
      <div className="flex items-center gap-2">
        {field.match ? (
          <CheckCircle2 className="w-3.5 h-3.5 text-green-400" />
        ) : (
          <XCircle className="w-3.5 h-3.5 text-orange-400" />
        )}
        <span className="text-sm text-gray-300">{displayName}</span>
      </div>
      <div className="flex items-center gap-4 text-sm">
        <div className="text-right">
          <span className="text-gray-500">Current: </span>
          <span className={field.match ? 'text-gray-300' : 'text-orange-400'}>
            {formatValue(field.current)}
          </span>
        </div>
        {!field.match && (
          <div className="text-right">
            <span className="text-gray-500">Default: </span>
            <span className="text-green-400">{formatValue(field.default)}</span>
          </div>
        )}
      </div>
    </div>
  );
}

// Strategy Card Component
function StrategyCard({
  strategyName,
  comparison,
  isExpanded,
  onToggle,
  expandedGroups,
  onToggleGroup,
  onReset,
}: {
  strategyName: StrategyName;
  comparison: StrategyComparison;
  isExpanded: boolean;
  onToggle: () => void;
  expandedGroups: Record<string, boolean>;
  onToggleGroup: (groupName: string) => void;
  onReset: () => void;
}) {
  const displayName = STRATEGY_DISPLAY_NAMES[strategyName];
  const icon = STRATEGY_ICONS[strategyName];
  const allMatch = comparison.total_fields === comparison.matching_fields;

  return (
    <div className="mb-4">
      <SectionHeader
        title={displayName}
        icon={icon}
        isExpanded={isExpanded}
        onToggle={onToggle}
        allMatch={allMatch}
        totalItems={comparison.total_fields}
        matchingItems={comparison.matching_fields}
        resetButton={
          !allMatch && (
            <ResetButton onClick={onReset} label="Reset" size="small" />
          )
        }
      >
        {/* Groups within this strategy */}
        <div className="divide-y divide-gray-700/30">
          {Object.entries(comparison.groups).map(([groupName, group]) => (
            <div key={groupName}>
              <GroupHeader
                groupName={groupName}
                group={group}
                isExpanded={expandedGroups[groupName] || false}
                onToggle={() => onToggleGroup(groupName)}
              />
              {expandedGroups[groupName] && (
                <div className="divide-y divide-gray-700/20">
                  {group.fields.map((field, idx) => (
                    <FieldRow key={`${field.path}-${idx}`} field={field} />
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      </SectionHeader>
    </div>
  );
}

// ==================== MAIN COMPONENT ====================

export default function DecisionEngineSettings({
  isAdmin = false,
  onResetStrategy,
  onResetAll,
}: DecisionEngineSettingsProps) {
  // State
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [comparison, setComparison] = useState<DecisionEngineComparisonResponse | null>(null);
  const [expandedStrategies, setExpandedStrategies] = useState<Record<StrategyName, boolean>>({
    trend_following: false,
    mean_reversion: false,
    breakout: false,
    range_trading: false,
  });
  const [expandedGroups, setExpandedGroups] = useState<Record<string, Record<string, boolean>>>({});
  const [resetting, setResetting] = useState(false);

  // Load comparison data
  const loadComparison = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await getDecisionEngineComparison();
      setComparison(data);
    } catch (err) {
      console.error('Failed to load decision engine comparison:', err);
      setError(err instanceof Error ? err.message : 'Failed to load settings comparison');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadComparison();
  }, [loadComparison]);

  // Toggle strategy expansion
  const toggleStrategy = (strategy: StrategyName) => {
    setExpandedStrategies((prev) => ({
      ...prev,
      [strategy]: !prev[strategy],
    }));
  };

  // Toggle group expansion within a strategy
  const toggleGroup = (strategy: StrategyName, groupName: string) => {
    setExpandedGroups((prev) => ({
      ...prev,
      [strategy]: {
        ...prev[strategy],
        [groupName]: !prev[strategy]?.[groupName],
      },
    }));
  };

  // Reset a specific strategy
  const handleResetStrategy = async (strategy: StrategyName) => {
    try {
      setResetting(true);
      await resetStrategySettings(strategy);
      await loadComparison();
      onResetStrategy?.(strategy);
    } catch (err) {
      console.error('Failed to reset strategy:', err);
      setError(err instanceof Error ? err.message : 'Failed to reset strategy');
    } finally {
      setResetting(false);
    }
  };

  // Reset all strategies
  const handleResetAll = async () => {
    try {
      setResetting(true);
      await resetDecisionEngineSettings();
      await loadComparison();
      onResetAll?.();
    } catch (err) {
      console.error('Failed to reset all strategies:', err);
      setError(err instanceof Error ? err.message : 'Failed to reset all strategies');
    } finally {
      setResetting(false);
    }
  };

  // Loading state
  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="w-8 h-8 animate-spin text-blue-400" />
        <span className="ml-3 text-gray-400">Loading Decision Engine settings...</span>
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className="bg-red-900/20 border border-red-500/30 rounded-lg p-4">
        <div className="flex items-center gap-2 text-red-400">
          <AlertTriangle className="w-5 h-5" />
          <span>Error: {error}</span>
        </div>
        <button
          onClick={loadComparison}
          className="mt-3 px-3 py-1 text-sm bg-red-600 hover:bg-red-700 text-white rounded"
        >
          Retry
        </button>
      </div>
    );
  }

  if (!comparison) return null;

  const allMatch = comparison.all_match;
  const strategies = Object.keys(comparison.strategies) as StrategyName[];

  return (
    <div className="space-y-4">
      {/* Header */}
      <div
        className={`rounded-lg border p-4 ${
          allMatch
            ? 'bg-green-900/20 border-green-500/30'
            : 'bg-orange-900/20 border-orange-500/30'
        }`}
      >
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Settings className="w-6 h-6 text-blue-400" />
            <div>
              <h2 className="text-xl font-bold text-white">Decision Engine Settings</h2>
              <p className="text-sm text-gray-400">
                {comparison.matching_fields}/{comparison.total_fields} fields match defaults across{' '}
                {comparison.total_strategies} strategies
              </p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            {!allMatch && (
              <ResetButton
                onClick={handleResetAll}
                label="Reset All Strategies"
                size="medium"
                disabled={resetting}
              />
            )}
            <span
              className={`px-4 py-2 rounded-full font-medium ${
                allMatch
                  ? 'bg-green-500/30 text-green-300'
                  : 'bg-orange-500/30 text-orange-300'
              }`}
            >
              {allMatch ? 'All Match' : `${comparison.total_differences} Differences`}
            </span>
          </div>
        </div>

        {/* Active Strategy indicator */}
        <div className="mt-3 pt-3 border-t border-gray-700/50">
          <div className="flex items-center gap-2">
            <span className="text-sm text-gray-400">Active Strategy:</span>
            <span className="text-sm font-medium text-white">
              {STRATEGY_DISPLAY_NAMES[comparison.active_strategy.current]}
            </span>
            {!comparison.active_strategy.match && (
              <span className="text-xs text-orange-400">
                (default: {STRATEGY_DISPLAY_NAMES[comparison.active_strategy.default]})
              </span>
            )}
          </div>
        </div>
      </div>

      {/* Strategy Cards */}
      {strategies.map((strategyName) => {
        const strategyComparison = comparison.strategies[strategyName];
        if (!strategyComparison) return null;

        return (
          <StrategyCard
            key={strategyName}
            strategyName={strategyName}
            comparison={strategyComparison}
            isExpanded={expandedStrategies[strategyName] || false}
            onToggle={() => toggleStrategy(strategyName)}
            expandedGroups={expandedGroups[strategyName] || {}}
            onToggleGroup={(groupName) => toggleGroup(strategyName, groupName)}
            onReset={() => handleResetStrategy(strategyName)}
          />
        );
      })}

      {/* Resetting overlay */}
      {resetting && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-gray-800 rounded-lg p-6 flex items-center gap-3">
            <Loader2 className="w-6 h-6 animate-spin text-blue-400" />
            <span className="text-white">Resetting settings...</span>
          </div>
        </div>
      )}
    </div>
  );
}
