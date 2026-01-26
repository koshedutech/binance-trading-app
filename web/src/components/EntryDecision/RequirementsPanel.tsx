// Strategy Requirements Panel Component
// Epic 14: Chain Trading System - Entry Decision Strategy Requirements Display
// Displays what each strategy requires to trigger an entry signal

import React, { useState } from 'react';
import {
  ChevronDown,
  ChevronRight,
  Activity,
  BarChart2,
  TrendingUp,
  Clock,
  Target,
  AlertTriangle,
  Shield,
  DollarSign,
  Percent,
} from 'lucide-react';

// ==================== Types ====================

interface VolumeImbalanceRequirements {
  volumeSpikeMultiplier: number;
  lookbackPeriod: number;
  minConsolidationCandles: number;
  maxConsolidationCandles: number;
  consolidationRangeTolerance: number;
  breakoutVolumeSurge: number;
  patternExpirationMinutes: number;
}

interface RiskManagementLevels {
  entryDescription: string;
  stopLossDescription: string;
  takeProfitDescription: string;
  riskRewardRatio: number;
  trailingDescription?: string;
}

interface RequirementsPanelProps {
  /** Strategy name */
  strategy: string;
  /** Sub-strategy name */
  subStrategy?: string;
  /** Timeframe */
  timeframe: string;
  /** Whether to show expanded by default */
  defaultExpanded?: boolean;
  /** Custom Volume Imbalance config (optional) */
  volumeImbalanceConfig?: Partial<VolumeImbalanceRequirements>;
}

// ==================== Default Configurations ====================

const DEFAULT_VOLUME_IMBALANCE_CONFIG: VolumeImbalanceRequirements = {
  volumeSpikeMultiplier: 2.0,
  lookbackPeriod: 20,
  minConsolidationCandles: 2,
  maxConsolidationCandles: 6,
  consolidationRangeTolerance: 0.01, // 1%
  breakoutVolumeSurge: 1.5, // 50% above avg
  patternExpirationMinutes: 60,
};

const DEFAULT_RISK_MANAGEMENT: RiskManagementLevels = {
  entryDescription: 'At/above reference candle high',
  stopLossDescription: 'Below consolidation low (-0.1%)',
  takeProfitDescription: 'Entry + (Risk x 4.0 R:R)',
  riskRewardRatio: 4.0,
  trailingDescription: 'Move SL to BE at 2:1, lock at 3:1',
};

// ==================== Step Components ====================

interface StepRequirementProps {
  stepNumber: number;
  title: string;
  requirements: Array<{ label: string; value: string; highlight?: boolean }>;
  icon: React.ReactNode;
  color: string;
}

function StepRequirement({ stepNumber, title, requirements, icon, color }: StepRequirementProps) {
  return (
    <div className="p-3 bg-gray-800/30 rounded-lg border border-gray-700/50">
      <div className="flex items-center gap-2 mb-2">
        <span className={`flex items-center justify-center w-6 h-6 rounded-full text-xs font-bold ${color}`}>
          {stepNumber}
        </span>
        <span className={`${color.replace('bg-', 'text-').replace('/20', '-400')}`}>
          {icon}
        </span>
        <span className="font-medium text-white text-sm">{title}</span>
      </div>
      <ul className="space-y-1.5 pl-8">
        {requirements.map((req, idx) => (
          <li key={idx} className="flex items-start gap-2 text-xs">
            <span className="text-gray-500 mt-0.5">•</span>
            <span className={req.highlight ? 'text-yellow-400' : 'text-gray-400'}>
              {req.label}: <span className="text-gray-300 font-mono">{req.value}</span>
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

// ==================== Risk Management Component ====================

interface RiskManagementSectionProps {
  levels: RiskManagementLevels;
}

function RiskManagementSection({ levels }: RiskManagementSectionProps) {
  return (
    <div className="p-3 bg-gray-800/30 rounded-lg border border-green-500/20">
      <div className="flex items-center gap-2 mb-3">
        <Shield className="w-4 h-4 text-green-400" />
        <span className="font-medium text-green-400 text-sm">RISK MANAGEMENT</span>
      </div>
      <div className="space-y-2">
        {/* Entry */}
        <div className="flex items-center gap-2 text-xs">
          <TrendingUp className="w-3 h-3 text-blue-400" />
          <span className="text-gray-500 w-16">Entry:</span>
          <span className="text-gray-300">{levels.entryDescription}</span>
        </div>

        {/* Stop Loss */}
        <div className="flex items-center gap-2 text-xs">
          <AlertTriangle className="w-3 h-3 text-red-400" />
          <span className="text-gray-500 w-16">SL:</span>
          <span className="text-gray-300">{levels.stopLossDescription}</span>
        </div>

        {/* Take Profit */}
        <div className="flex items-center gap-2 text-xs">
          <Target className="w-3 h-3 text-green-400" />
          <span className="text-gray-500 w-16">TP:</span>
          <span className="text-gray-300">{levels.takeProfitDescription}</span>
        </div>

        {/* Risk/Reward Ratio */}
        <div className="flex items-center gap-2 text-xs mt-2 pt-2 border-t border-gray-700/50">
          <DollarSign className="w-3 h-3 text-yellow-400" />
          <span className="text-gray-500 w-16">R:R:</span>
          <span className="text-yellow-400 font-medium">1:{levels.riskRewardRatio}</span>
        </div>

        {/* Trailing */}
        {levels.trailingDescription && (
          <div className="flex items-center gap-2 text-xs">
            <Percent className="w-3 h-3 text-purple-400" />
            <span className="text-gray-500 w-16">Trailing:</span>
            <span className="text-gray-300">{levels.trailingDescription}</span>
          </div>
        )}
      </div>
    </div>
  );
}

// ==================== Volume Imbalance Requirements ====================

interface VolumeImbalanceRequirementsPanelProps {
  config: VolumeImbalanceRequirements;
  timeframe: string;
}

function VolumeImbalanceRequirementsPanel({ config, timeframe }: VolumeImbalanceRequirementsPanelProps) {
  return (
    <div className="space-y-3">
      {/* Step 1: Volume Spike */}
      <StepRequirement
        stepNumber={1}
        title="VOLUME SPIKE"
        icon={<Activity className="w-4 h-4" />}
        color="bg-blue-500/20"
        requirements={[
          {
            label: 'Volume threshold',
            value: `>= ${config.volumeSpikeMultiplier}x average`,
            highlight: true,
          },
          {
            label: 'Lookback period',
            value: `${config.lookbackPeriod} candles`,
          },
          {
            label: 'Preference',
            value: 'Bullish candle (close > open)',
          },
        ]}
      />

      {/* Step 2: Consolidation */}
      <StepRequirement
        stepNumber={2}
        title="CONSOLIDATION"
        icon={<BarChart2 className="w-4 h-4" />}
        color="bg-yellow-500/20"
        requirements={[
          {
            label: 'Duration',
            value: `${config.minConsolidationCandles}-${config.maxConsolidationCandles} candles`,
          },
          {
            label: 'Price range',
            value: `Stay within +/- ${(config.consolidationRangeTolerance * 100).toFixed(0)}% of reference`,
          },
          {
            label: 'Volume trend',
            value: 'Must be declining (negative slope)',
            highlight: true,
          },
        ]}
      />

      {/* Step 3: Breakout */}
      <StepRequirement
        stepNumber={3}
        title="BREAKOUT"
        icon={<TrendingUp className="w-4 h-4" />}
        color="bg-green-500/20"
        requirements={[
          {
            label: 'Volume surge',
            value: `>= ${((config.breakoutVolumeSurge - 1) * 100).toFixed(0)}% above consolidation avg`,
            highlight: true,
          },
          {
            label: 'Price action',
            value: 'Breaks above reference high',
          },
          {
            label: 'Confirmation',
            value: 'Close confirms above reference',
          },
        ]}
      />

      {/* Risk Management */}
      <RiskManagementSection levels={DEFAULT_RISK_MANAGEMENT} />

      {/* Pattern Expiration Notice */}
      <div className="flex items-center gap-2 text-xs text-gray-500 p-2 bg-gray-800/20 rounded">
        <Clock className="w-3 h-3" />
        <span>
          Pattern expires after {config.patternExpirationMinutes} minutes if not completed
        </span>
      </div>
    </div>
  );
}

// ==================== Score-Based Requirements (Trend Following) ====================

interface ScoreBasedRequirementsPanelProps {
  threshold?: number;
  timeframe: string;
}

function ScoreBasedRequirementsPanel({ threshold = 55, timeframe }: ScoreBasedRequirementsPanelProps) {
  const components = [
    { name: 'Trend Alignment', weight: 25, description: 'Price above/below moving averages' },
    { name: 'Volume Confirmation', weight: 20, description: 'Volume supports trend direction' },
    { name: 'Momentum', weight: 20, description: 'RSI/MACD alignment with trend' },
    { name: 'Support/Resistance', weight: 15, description: 'Price near key levels' },
    { name: 'Market Context', weight: 10, description: 'Overall market conditions' },
    { name: 'Volatility', weight: 10, description: 'ATR within acceptable range' },
  ];

  return (
    <div className="space-y-3">
      {/* Score Components */}
      <div className="p-3 bg-gray-800/30 rounded-lg border border-gray-700/50">
        <div className="flex items-center gap-2 mb-3">
          <BarChart2 className="w-4 h-4 text-purple-400" />
          <span className="font-medium text-purple-400 text-sm">SCORE COMPONENTS</span>
        </div>
        <div className="space-y-2">
          {components.map((comp, idx) => (
            <div key={idx} className="flex items-center justify-between text-xs">
              <span className="text-gray-400">{comp.name}</span>
              <span className="text-gray-500">{comp.description}</span>
              <span className="text-purple-400 font-mono w-12 text-right">{comp.weight}%</span>
            </div>
          ))}
        </div>
      </div>

      {/* Threshold */}
      <div className="p-3 bg-gray-800/30 rounded-lg border border-yellow-500/20">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Target className="w-4 h-4 text-yellow-400" />
            <span className="font-medium text-yellow-400 text-sm">ENTRY THRESHOLD</span>
          </div>
          <span className="text-xl font-bold text-yellow-400">{threshold}/100</span>
        </div>
        <p className="text-xs text-gray-500 mt-2">
          Coin is ready when total score meets or exceeds this threshold
        </p>
      </div>

      {/* Risk Management */}
      <RiskManagementSection
        levels={{
          entryDescription: 'At current price when score >= threshold',
          stopLossDescription: 'ATR-based stop (2x ATR below entry)',
          takeProfitDescription: 'ATR-based target (3x ATR above entry)',
          riskRewardRatio: 1.5,
          trailingDescription: 'Move SL to BE at 1:1, trail at 1.5:1',
        }}
      />
    </div>
  );
}

// ==================== Main Component ====================

export default function RequirementsPanel({
  strategy,
  subStrategy,
  timeframe,
  defaultExpanded = false,
  volumeImbalanceConfig,
}: RequirementsPanelProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  // Merge custom config with defaults
  const viConfig = {
    ...DEFAULT_VOLUME_IMBALANCE_CONFIG,
    ...volumeImbalanceConfig,
  };

  // Determine strategy type
  const isVolumeImbalance = strategy === 'volume_imbalance' ||
    subStrategy?.includes('volume_imbalance') ||
    subStrategy?.includes('ravindra');

  const formatStrategyTitle = () => {
    if (subStrategy) {
      return subStrategy.split('_').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
    }
    return strategy.split('_').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
  };

  return (
    <div className="bg-gray-900/50 rounded-lg border border-gray-700/50 overflow-hidden">
      {/* Header */}
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between p-3 hover:bg-gray-800/30 transition-colors"
      >
        <div className="flex items-center gap-2">
          {expanded ? (
            <ChevronDown className="w-4 h-4 text-gray-400" />
          ) : (
            <ChevronRight className="w-4 h-4 text-gray-400" />
          )}
          <span className="text-sm font-medium text-white">
            {formatStrategyTitle()} Requirements
          </span>
          <span className="px-1.5 py-0.5 bg-gray-700/50 text-gray-400 rounded text-xs">
            {timeframe}
          </span>
        </div>
        <span className="text-xs text-gray-500">
          {expanded ? 'Hide' : 'Show'} requirements
        </span>
      </button>

      {/* Content */}
      {expanded && (
        <div className="p-3 border-t border-gray-700/50">
          {isVolumeImbalance ? (
            <VolumeImbalanceRequirementsPanel config={viConfig} timeframe={timeframe} />
          ) : (
            <ScoreBasedRequirementsPanel timeframe={timeframe} />
          )}
        </div>
      )}
    </div>
  );
}

// ==================== Compact Requirements Badge ====================

interface RequirementsBadgeProps {
  strategy: string;
  stepsCompleted?: number;
  totalSteps?: number;
  score?: number;
  threshold?: number;
}

export function RequirementsBadge({
  strategy,
  stepsCompleted,
  totalSteps,
  score,
  threshold,
}: RequirementsBadgeProps) {
  const isPattern = totalSteps !== undefined && stepsCompleted !== undefined;

  if (isPattern) {
    return (
      <div className="flex items-center gap-1 text-xs">
        <span className="text-gray-500">Progress:</span>
        <span className="font-mono text-blue-400">
          {stepsCompleted}/{totalSteps}
        </span>
        {stepsCompleted === totalSteps && (
          <span className="ml-1 px-1.5 py-0.5 bg-green-500/20 text-green-400 rounded-full text-[10px]">
            READY
          </span>
        )}
      </div>
    );
  }

  if (score !== undefined && threshold !== undefined) {
    const isReady = score >= threshold;
    return (
      <div className="flex items-center gap-1 text-xs">
        <span className="text-gray-500">Score:</span>
        <span className={`font-mono ${isReady ? 'text-green-400' : 'text-yellow-400'}`}>
          {score}/{threshold}
        </span>
        {isReady && (
          <span className="ml-1 px-1.5 py-0.5 bg-green-500/20 text-green-400 rounded-full text-[10px]">
            READY
          </span>
        )}
      </div>
    );
  }

  return null;
}
