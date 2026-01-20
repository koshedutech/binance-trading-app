// Epic 10: Position Stage Management - Story 10.1: Position Card Expanded UI
// Expandable position card with full position management display

import React, { useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import {
  ChevronDown,
  ChevronRight,
  TrendingUp,
  TrendingDown,
  AlertTriangle,
  Shield,
  Target,
  Activity,
  Clock,
  Zap,
  BarChart2,
  Brain,
  Settings,
  RefreshCw,
  Eye,
} from 'lucide-react';
import { ExitDecisionMonitor } from './ExitDecisionMonitor';
import type {
  ExpandedPositionData,
  PositionStage,
  PriceLevel,
  DecisionMode,
} from '../types/positionManagement';
import {
  STAGE_DISPLAY,
  getStageIndex,
  calculatePriceLevels,
  formatPositionPrice,
  formatPositionPercent,
} from '../types/positionManagement';

// ==================== Stage Icon Component ====================

interface StageIconProps {
  stage: PositionStage;
  size?: 'sm' | 'md' | 'lg';
  showLabel?: boolean;
}

function StageIcon({ stage, size = 'md', showLabel = false }: StageIconProps) {
  const config = STAGE_DISPLAY[stage];
  const sizeClasses = {
    sm: 'w-4 h-4',
    md: 'w-5 h-5',
    lg: 'w-6 h-6',
  };

  const IconComponent = {
    AlertTriangle,
    Shield,
    Target,
    TrendingUp,
  }[config.icon] || Activity;

  return (
    <div className={`flex items-center gap-2 ${config.color}`}>
      <IconComponent className={sizeClasses[size]} />
      {showLabel && <span className="text-sm font-medium">{config.name}</span>}
    </div>
  );
}

// ==================== Stage Badge Component ====================

interface StageBadgeProps {
  stage: PositionStage;
  compact?: boolean;
}

function StageBadge({ stage, compact = false }: StageBadgeProps) {
  const config = STAGE_DISPLAY[stage];

  if (compact) {
    return (
      <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${config.bgColor} ${config.color} border ${config.borderColor}`}>
        <StageIcon stage={stage} size="sm" />
        {config.name}
      </span>
    );
  }

  return (
    <div className={`flex items-center gap-2 px-3 py-1.5 rounded-lg ${config.bgColor} border ${config.borderColor}`}>
      <StageIcon stage={stage} size="md" />
      <div>
        <div className={`text-sm font-medium ${config.color}`}>{config.name}</div>
        <div className="text-xs text-gray-500">{config.description}</div>
      </div>
    </div>
  );
}

// ==================== Stage Timeline Component ====================

interface StageTimelineProps {
  currentStage: PositionStage;
  stageEntryTime?: number;
}

function StageTimeline({ currentStage, stageEntryTime }: StageTimelineProps) {
  const stages: PositionStage[] = ['RISK_ZONE', 'BREAKEVEN', 'TP1', 'EFFICIENCY'];
  const currentIndex = getStageIndex(currentStage);

  return (
    <div className="flex items-center justify-between">
      {stages.map((stage, index) => {
        const config = STAGE_DISPLAY[stage];
        const isActive = index === currentIndex;
        const isCompleted = index < currentIndex;
        const isPending = index > currentIndex;

        return (
          <React.Fragment key={stage}>
            {/* Stage node */}
            <div className="flex flex-col items-center">
              <div
                className={`
                  w-10 h-10 rounded-full flex items-center justify-center transition-all
                  ${isActive ? `${config.bgColor} ${config.borderColor} border-2 ring-2 ring-offset-2 ring-offset-gray-900 ring-${config.color.replace('text-', '')}` : ''}
                  ${isCompleted ? 'bg-green-500/30 border-green-500/50 border' : ''}
                  ${isPending ? 'bg-gray-700/50 border-gray-600 border' : ''}
                `}
              >
                <StageIcon stage={stage} size="sm" />
              </div>
              <span className={`mt-2 text-xs font-medium ${isActive ? config.color : isCompleted ? 'text-green-400' : 'text-gray-500'}`}>
                {config.name}
              </span>
              {isActive && stageEntryTime && (
                <span className="text-xs text-gray-500">
                  {formatDistanceToNow(stageEntryTime, { addSuffix: true })}
                </span>
              )}
            </div>

            {/* Connector line */}
            {index < stages.length - 1 && (
              <div
                className={`flex-1 h-0.5 mx-2 ${
                  index < currentIndex ? 'bg-green-500' : 'bg-gray-700'
                }`}
              />
            )}
          </React.Fragment>
        );
      })}
    </div>
  );
}

// ==================== Price Levels Visualization ====================

interface PriceLevelsDisplayProps {
  position: ExpandedPositionData;
}

function PriceLevelsDisplay({ position }: PriceLevelsDisplayProps) {
  const levels = calculatePriceLevels(position);
  const isLong = position.side === 'LONG';

  // Calculate price range for positioning
  const prices = levels.map(l => l.price);
  const minPrice = Math.min(...prices) * 0.995;
  const maxPrice = Math.max(...prices) * 1.005;
  const range = maxPrice - minPrice;

  const getPosition = (price: number) => {
    return ((price - minPrice) / range) * 100;
  };

  return (
    <div className="relative h-32 bg-gray-800 rounded-lg p-4">
      {/* Price scale */}
      <div className="absolute left-0 top-0 bottom-0 w-16 flex flex-col justify-between text-xs text-gray-500 py-2">
        <span>{formatPositionPrice(maxPrice)}</span>
        <span>{formatPositionPrice(minPrice)}</span>
      </div>

      {/* Price levels */}
      <div className="relative ml-16 h-full">
        {levels.map((level, idx) => {
          const position_y = 100 - getPosition(level.price);

          return (
            <div
              key={`${level.type}-${idx}`}
              className="absolute left-0 right-0 flex items-center"
              style={{ top: `${position_y}%` }}
            >
              <div className={`w-full h-px ${level.type === 'current' ? 'bg-white' : 'bg-gray-600'}`} />
              <span className={`absolute right-0 px-2 py-0.5 rounded text-xs ${level.color} bg-gray-900`}>
                {level.label}: {formatPositionPrice(level.price)}
              </span>
            </div>
          );
        })}

        {/* Current price indicator */}
        <div
          className="absolute left-0 flex items-center"
          style={{ top: `${100 - getPosition(position.current_price)}%` }}
        >
          <div className="w-2 h-2 bg-white rounded-full animate-pulse" />
        </div>
      </div>
    </div>
  );
}

// ==================== Efficiency Metrics Display ====================

interface EfficiencyMetricsDisplayProps {
  metrics: ExpandedPositionData['efficiency'];
}

function EfficiencyMetricsDisplay({ metrics }: EfficiencyMetricsDisplayProps) {
  if (!metrics) return null;

  const efficiencyColor = metrics.efficiency_percent >= 80 ? 'text-green-400' :
    metrics.efficiency_percent >= 50 ? 'text-yellow-400' : 'text-red-400';

  return (
    <div className="bg-purple-900/20 border border-purple-500/30 rounded-lg p-4">
      <h4 className="text-sm font-medium text-purple-400 mb-3 flex items-center gap-2">
        <Activity className="w-4 h-4" />
        Efficiency Metrics
      </h4>

      <div className="grid grid-cols-2 gap-4">
        <div>
          <span className="text-xs text-gray-500">Peak Profit</span>
          <div className="text-lg font-semibold text-green-400">
            {formatPositionPercent(metrics.peak_profit)}
          </div>
        </div>
        <div>
          <span className="text-xs text-gray-500">Current Profit</span>
          <div className={`text-lg font-semibold ${metrics.current_profit >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {formatPositionPercent(metrics.current_profit)}
          </div>
        </div>
        <div>
          <span className="text-xs text-gray-500">Efficiency</span>
          <div className={`text-lg font-semibold ${efficiencyColor}`}>
            {metrics.efficiency_percent.toFixed(1)}%
          </div>
        </div>
        <div>
          <span className="text-xs text-gray-500">Threshold</span>
          <div className="text-lg font-semibold text-gray-300">
            {metrics.threshold_percent.toFixed(1)}%
          </div>
        </div>
      </div>

      {/* Efficiency progress bar */}
      <div className="mt-3">
        <div className="flex justify-between text-xs text-gray-500 mb-1">
          <span>Efficiency vs Threshold</span>
          <span>{metrics.efficiency_percent.toFixed(1)}% / {metrics.threshold_percent.toFixed(1)}%</span>
        </div>
        <div className="h-2 bg-gray-700 rounded-full overflow-hidden">
          <div
            className={`h-full transition-all ${
              metrics.efficiency_percent >= metrics.threshold_percent ? 'bg-green-500' : 'bg-yellow-500'
            }`}
            style={{ width: `${Math.min(100, (metrics.efficiency_percent / metrics.threshold_percent) * 100)}%` }}
          />
        </div>
      </div>
    </div>
  );
}

// ==================== Classic Indicator Scores ====================

interface ClassicIndicatorDisplayProps {
  scores: ExpandedPositionData['classic_scores'];
}

function ClassicIndicatorDisplay({ scores }: ClassicIndicatorDisplayProps) {
  if (!scores) return null;

  const adxStatus = scores.adx >= scores.adx_threshold;
  const rsiColor = scores.rsi_state === 'overbought' ? 'text-red-400' :
    scores.rsi_state === 'oversold' ? 'text-green-400' : 'text-gray-400';

  return (
    <div className="bg-gray-800/50 rounded-lg p-4">
      <h4 className="text-sm font-medium text-gray-400 mb-3 flex items-center gap-2">
        <BarChart2 className="w-4 h-4 text-blue-400" />
        Classic Indicators
      </h4>

      <div className="space-y-3">
        {/* ADX */}
        <div>
          <div className="flex justify-between text-xs mb-1">
            <span className="text-gray-500">ADX</span>
            <span className={adxStatus ? 'text-green-400' : 'text-yellow-400'}>
              {scores.adx.toFixed(1)} / {scores.adx_threshold}
            </span>
          </div>
          <div className="h-2 bg-gray-700 rounded-full overflow-hidden">
            <div
              className={`h-full ${adxStatus ? 'bg-green-500' : 'bg-yellow-500'}`}
              style={{ width: `${Math.min(100, (scores.adx / 50) * 100)}%` }}
            />
          </div>
        </div>

        {/* RSI */}
        <div>
          <div className="flex justify-between text-xs mb-1">
            <span className="text-gray-500">RSI</span>
            <span className={rsiColor}>
              {scores.rsi.toFixed(1)} ({scores.rsi_state})
            </span>
          </div>
          <div className="h-2 bg-gray-700 rounded-full overflow-hidden">
            <div
              className={`h-full ${rsiColor.replace('text-', 'bg-').replace('-400', '-500')}`}
              style={{ width: `${scores.rsi}%` }}
            />
          </div>
        </div>

        {/* Reversal Signals */}
        <div>
          <div className="flex justify-between text-xs mb-1">
            <span className="text-gray-500">Reversal Signals</span>
            <span className={scores.reversal_signals >= scores.reversal_required ? 'text-red-400' : 'text-gray-400'}>
              {scores.reversal_signals} / {scores.reversal_required}
            </span>
          </div>
          <div className="flex gap-1">
            {Array.from({ length: scores.reversal_required }).map((_, i) => (
              <div
                key={i}
                className={`h-2 flex-1 rounded ${
                  i < scores.reversal_signals ? 'bg-red-500' : 'bg-gray-700'
                }`}
              />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

// ==================== New Engine Indicator Scores ====================

interface NewEngineIndicatorDisplayProps {
  scores: ExpandedPositionData['new_engine_scores'];
}

function NewEngineIndicatorDisplay({ scores }: NewEngineIndicatorDisplayProps) {
  if (!scores) return null;

  const scoreItems = [
    { label: 'Technical', value: scores.technical, max: 40, color: 'bg-blue-500' },
    { label: 'Context', value: scores.context, max: 30, color: 'bg-green-500' },
    { label: 'LLM', value: scores.llm, max: 20, color: 'bg-purple-500' },
    { label: 'History', value: scores.history, max: 10, color: 'bg-yellow-500' },
  ];

  const finalColor = scores.final >= 75 ? 'text-green-400' :
    scores.final >= 50 ? 'text-yellow-400' : 'text-red-400';

  return (
    <div className="bg-gray-800/50 rounded-lg p-4">
      <h4 className="text-sm font-medium text-gray-400 mb-3 flex items-center gap-2">
        <Brain className="w-4 h-4 text-purple-400" />
        New Engine Scores
      </h4>

      {/* Final score */}
      <div className="flex items-center justify-between mb-4">
        <span className="text-gray-400">Final Score</span>
        <div className={`text-2xl font-bold ${finalColor}`}>
          {scores.final}/100
        </div>
      </div>

      {/* Score breakdown */}
      <div className="space-y-2">
        {scoreItems.map(item => (
          <div key={item.label}>
            <div className="flex justify-between text-xs mb-1">
              <span className="text-gray-500">{item.label}</span>
              <span className="text-gray-400">{item.value}/{item.max}</span>
            </div>
            <div className="h-1.5 bg-gray-700 rounded-full overflow-hidden">
              <div
                className={`h-full ${item.color}`}
                style={{ width: `${(item.value / item.max) * 100}%` }}
              />
            </div>
          </div>
        ))}
      </div>

      {/* Regime & Strategy */}
      <div className="mt-4 pt-3 border-t border-gray-700 flex justify-between text-xs">
        <div>
          <span className="text-gray-500">Regime: </span>
          <span className="text-gray-300">{scores.regime}</span>
        </div>
        <div>
          <span className="text-gray-500">Strategy: </span>
          <span className="text-gray-300">{scores.strategy}</span>
        </div>
      </div>
    </div>
  );
}

// ==================== Decision Mode Badge ====================

interface DecisionModeBadgeProps {
  mode: DecisionMode;
}

function DecisionModeBadge({ mode }: DecisionModeBadgeProps) {
  const isNewEngine = mode === 'new_engine';

  return (
    <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${
      isNewEngine
        ? 'bg-purple-500/20 text-purple-400 border border-purple-500/30'
        : 'bg-blue-500/20 text-blue-400 border border-blue-500/30'
    }`}>
      {isNewEngine ? <Brain className="w-3 h-3" /> : <BarChart2 className="w-3 h-3" />}
      {isNewEngine ? 'New Engine' : 'Classic'}
    </span>
  );
}

// ==================== Main Expanded Position Card ====================

interface PositionCardExpandedProps {
  position: ExpandedPositionData;
  defaultExpanded?: boolean;
  onClose?: (symbol: string) => void;
  onRefresh?: (symbol: string) => void;
  onCollapse?: (symbol: string) => void;
}

export default function PositionCardExpanded({
  position,
  defaultExpanded = false,
  onClose,
  onRefresh,
  onCollapse,
}: PositionCardExpandedProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const [exitMonitorExpanded, setExitMonitorExpanded] = useState(false);

  // Handle toggle with parent notification
  const handleToggle = () => {
    const newState = !expanded;
    setExpanded(newState);
    // Notify parent when collapsing
    if (!newState && onCollapse) {
      onCollapse(position.symbol);
    }
  };

  const isLong = position.side === 'LONG';
  const SideIcon = isLong ? TrendingUp : TrendingDown;
  const sideColor = isLong ? 'text-green-400' : 'text-red-400';
  const pnlColor = position.unrealized_pnl >= 0 ? 'text-green-400' : 'text-red-400';

  const lastUpdatedText = position.last_updated > 0
    ? formatDistanceToNow(position.last_updated, { addSuffix: true })
    : 'Unknown';

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden transition-all duration-200 hover:border-gray-600">
      {/* Header - Always visible (Collapsed view) */}
      <button
        type="button"
        className="w-full p-4 flex items-center justify-between cursor-pointer hover:bg-gray-800/80 transition-colors text-left"
        onClick={handleToggle}
        aria-expanded={expanded}
        aria-controls={`position-expanded-${position.symbol}`}
      >
        <div className="flex items-center gap-3">
          {/* Expand/collapse chevron */}
          {expanded ? (
            <ChevronDown className="w-4 h-4 text-purple-400" />
          ) : (
            <ChevronRight className="w-4 h-4 text-gray-500" />
          )}

          {/* Symbol and side */}
          <div className="flex items-center gap-2">
            <SideIcon className={`w-5 h-5 ${sideColor}`} />
            <span className="text-lg font-semibold text-gray-200">{position.symbol}</span>
            <span className={`text-xs ${sideColor}`}>{position.side}</span>
            <span className="text-xs text-yellow-500">{position.leverage}x</span>
          </div>

          {/* Stage badge */}
          <StageBadge stage={position.stage} compact />

          {/* Decision mode */}
          <DecisionModeBadge mode={position.decision_mode} />
        </div>

        <div className="flex items-center gap-4">
          {/* P&L */}
          <div className="text-right">
            <div className={`text-sm font-semibold ${pnlColor}`}>
              {position.unrealized_pnl >= 0 ? '+' : ''}{position.unrealized_pnl.toFixed(2)} USDT
            </div>
            <div className={`text-xs ${pnlColor}`}>
              {formatPositionPercent(position.roe)} ROE
            </div>
          </div>

          {/* Entry price */}
          <div className="text-right">
            <div className="text-xs text-gray-500">Entry</div>
            <div className="text-sm font-mono text-gray-200">{formatPositionPrice(position.entry_price)}</div>
          </div>

          {/* SL indicator */}
          {position.stop_loss ? (
            <div className="text-right">
              <div className="text-xs text-gray-500">SL</div>
              <div className="text-sm font-mono text-red-400">{formatPositionPrice(position.stop_loss)}</div>
            </div>
          ) : (
            <div className="flex items-center gap-1 text-yellow-500">
              <AlertTriangle className="w-4 h-4" />
              <span className="text-xs">No SL</span>
            </div>
          )}

          {/* Last updated */}
          <div className="flex items-center gap-1 text-gray-500 text-xs">
            <Clock className="w-3 h-3" />
            <span>{lastUpdatedText}</span>
          </div>
        </div>
      </button>

      {/* Expanded content */}
      {expanded && (
        <div id={`position-expanded-${position.symbol}`} className="border-t border-gray-700 p-4 space-y-4">
          {/* Stage Timeline */}
          <div>
            <h4 className="text-sm font-medium text-gray-400 mb-3">Position Stage</h4>
            <StageTimeline
              currentStage={position.stage}
              stageEntryTime={position.stage_entry_time}
            />
          </div>

          {/* Price Levels Visualization */}
          <div>
            <h4 className="text-sm font-medium text-gray-400 mb-3 flex items-center gap-2">
              <Zap className="w-4 h-4 text-yellow-400" />
              Price Levels
            </h4>
            <PriceLevelsDisplay position={position} />
          </div>

          {/* Two-column layout for metrics */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {/* Efficiency Metrics (when in EFFICIENCY stage) */}
            {position.stage === 'EFFICIENCY' && position.efficiency && (
              <EfficiencyMetricsDisplay metrics={position.efficiency} />
            )}

            {/* Indicator scores based on decision mode */}
            {position.decision_mode === 'classic' && position.classic_scores && (
              <ClassicIndicatorDisplay scores={position.classic_scores} />
            )}
            {position.decision_mode === 'new_engine' && position.new_engine_scores && (
              <NewEngineIndicatorDisplay scores={position.new_engine_scores} />
            )}
          </div>

          {/* Story 10.3: Exit Decision Monitor - Collapsible Section */}
          <div className="border border-gray-700 rounded-lg overflow-hidden">
            <button
              type="button"
              onClick={() => setExitMonitorExpanded(!exitMonitorExpanded)}
              className="w-full px-4 py-3 flex items-center justify-between bg-gray-800/50 hover:bg-gray-700/50 transition-colors"
            >
              <div className="flex items-center gap-2">
                <Eye className="w-4 h-4 text-purple-400" />
                <span className="text-sm font-medium text-gray-300">Exit Decision Monitor</span>
                <span className="text-xs text-gray-500">(Real-time exit signals)</span>
              </div>
              {exitMonitorExpanded ? (
                <ChevronDown className="w-4 h-4 text-gray-400" />
              ) : (
                <ChevronRight className="w-4 h-4 text-gray-400" />
              )}
            </button>
            {exitMonitorExpanded && (
              <div className="p-4 border-t border-gray-700">
                <ExitDecisionMonitor
                  symbol={position.symbol}
                  isExpanded={exitMonitorExpanded}
                  onRefresh={() => onRefresh?.(position.symbol)}
                />
              </div>
            )}
          </div>

          {/* Actions */}
          <div className="pt-3 border-t border-gray-700 flex items-center justify-between">
            <div className="text-xs text-gray-500">
              Entered {formatDistanceToNow(position.entry_time, { addSuffix: true })}
            </div>
            <div className="flex items-center gap-2">
              {onRefresh && (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    onRefresh(position.symbol);
                  }}
                  className="flex items-center gap-1 px-3 py-1.5 text-sm text-gray-400 hover:text-gray-200 hover:bg-gray-700 rounded transition-colors"
                >
                  <RefreshCw className="w-4 h-4" />
                  Refresh
                </button>
              )}
              {onClose && (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    onClose(position.symbol);
                  }}
                  className="flex items-center gap-1 px-3 py-1.5 text-sm bg-red-600 hover:bg-red-500 text-white rounded transition-colors"
                >
                  Close Position
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ==================== Exports ====================

export {
  StageIcon,
  StageBadge,
  StageTimeline,
  PriceLevelsDisplay,
  EfficiencyMetricsDisplay,
  ClassicIndicatorDisplay,
  NewEngineIndicatorDisplay,
  DecisionModeBadge,
};

// ==================== Skeleton Component ====================

export function PositionCardExpandedSkeleton() {
  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700 p-4 animate-pulse">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-4 h-4 bg-gray-700 rounded" />
          <div className="w-24 h-6 bg-gray-700 rounded" />
          <div className="w-20 h-5 bg-gray-700 rounded-full" />
          <div className="w-16 h-5 bg-gray-700 rounded-full" />
        </div>
        <div className="flex items-center gap-4">
          <div className="w-20 h-8 bg-gray-700 rounded" />
          <div className="w-16 h-8 bg-gray-700 rounded" />
          <div className="w-16 h-8 bg-gray-700 rounded" />
        </div>
      </div>
    </div>
  );
}
