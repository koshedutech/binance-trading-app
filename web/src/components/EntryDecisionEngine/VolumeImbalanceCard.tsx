// Volume Imbalance Card
// Epic 11: Position Decision Engine - Story 11.46: Volume Imbalance UI Components

import React, { useState, useCallback } from 'react';
import {
  TrendingUp,
  TrendingDown,
  Check,
  Circle,
  Clock,
  AlertTriangle,
  Play,
  XCircle,
  ChevronDown,
  ChevronUp,
  Loader2,
  Target,
  Shield,
  BarChart3,
  Zap,
  Brain,
} from 'lucide-react';
import { formatDistanceToNow } from 'date-fns';
import type {
  VolumeImbalancePattern,
  VolumeImbalanceState,
  PatternStep,
  TradeSetup,
  VOLUME_IMBALANCE_STATE_COLORS,
  VOLUME_IMBALANCE_STATE_LABELS,
} from '../../types/strategyHierarchy';

// ==================== Interfaces ====================

interface VolumeImbalanceCardProps {
  pattern: VolumeImbalancePattern;
  onExecute: (patternId: string) => Promise<void>;
  onSkip: (patternId: string, reason?: string) => Promise<void>;
  isLoading?: boolean;
  disabled?: boolean;
}

// ==================== State Badge Component ====================

interface StateBadgeProps {
  state: VolumeImbalanceState;
  progress?: number;
}

function StateBadge({ state, progress }: StateBadgeProps) {
  const stateColors: Record<VolumeImbalanceState, { bg: string; text: string; border: string }> = {
    WATCHING: {
      bg: 'bg-gray-500/20',
      text: 'text-gray-400',
      border: 'border-gray-500/30',
    },
    CONSOLIDATING: {
      bg: 'bg-yellow-500/20',
      text: 'text-yellow-400',
      border: 'border-yellow-500/30',
    },
    READY: {
      bg: 'bg-green-500/20',
      text: 'text-green-400',
      border: 'border-green-500/30',
    },
    EXPIRED: {
      bg: 'bg-red-500/20',
      text: 'text-red-400',
      border: 'border-red-500/30',
    },
  };

  const stateLabels: Record<VolumeImbalanceState, string> = {
    WATCHING: 'Watching',
    CONSOLIDATING: 'Consolidating',
    READY: 'Ready',
    EXPIRED: 'Expired',
  };

  const colors = stateColors[state];

  return (
    <div className={`inline-flex items-center gap-2 px-3 py-1 ${colors.bg} ${colors.text} border ${colors.border} rounded-full`}>
      {state === 'WATCHING' && <Circle className="w-3 h-3" />}
      {state === 'CONSOLIDATING' && <Clock className="w-3 h-3 animate-pulse" />}
      {state === 'READY' && <Zap className="w-3 h-3" />}
      {state === 'EXPIRED' && <AlertTriangle className="w-3 h-3" />}
      <span className="text-xs font-medium">{stateLabels[state]}</span>
      {state === 'CONSOLIDATING' && progress !== undefined && (
        <span className="text-[10px] opacity-75">({progress}%)</span>
      )}
    </div>
  );
}

// ==================== Pattern Steps Component ====================

interface PatternStepsProps {
  steps: PatternStep[];
}

function PatternSteps({ steps }: PatternStepsProps) {
  return (
    <div className="flex items-center gap-1">
      {steps.map((step, index) => (
        <div key={index} className="flex items-center">
          {/* Step indicator */}
          <div
            className={`
              flex items-center justify-center w-6 h-6 rounded-full border-2 transition-colors
              ${step.status === 'completed'
                ? 'bg-green-500/20 border-green-500 text-green-400'
                : step.status === 'in_progress'
                  ? 'bg-yellow-500/20 border-yellow-500 text-yellow-400 animate-pulse'
                  : step.status === 'failed'
                    ? 'bg-red-500/20 border-red-500 text-red-400'
                    : 'bg-gray-700/50 border-gray-600 text-gray-500'
              }
            `}
            title={`${step.name}: ${step.description}`}
          >
            {step.status === 'completed' ? (
              <Check className="w-3 h-3" />
            ) : step.status === 'in_progress' ? (
              <Loader2 className="w-3 h-3 animate-spin" />
            ) : step.status === 'failed' ? (
              <XCircle className="w-3 h-3" />
            ) : (
              <span className="text-[10px] font-medium">{index + 1}</span>
            )}
          </div>

          {/* Connector line */}
          {index < steps.length - 1 && (
            <div
              className={`
                w-4 h-0.5 mx-0.5 transition-colors
                ${step.status === 'completed' ? 'bg-green-500' : 'bg-gray-600'}
              `}
            />
          )}
        </div>
      ))}
    </div>
  );
}

// ==================== Trade Setup Display Component ====================

interface TradeSetupDisplayProps {
  setup: TradeSetup;
  currentPrice: number;
}

function TradeSetupDisplay({ setup, currentPrice }: TradeSetupDisplayProps) {
  const isLong = setup.side === 'LONG';
  const entryDistance = ((currentPrice - setup.entry_price) / setup.entry_price) * 100;
  const slDistance = Math.abs((setup.stop_loss - setup.entry_price) / setup.entry_price) * 100;
  const tpDistance = Math.abs((setup.take_profit - setup.entry_price) / setup.entry_price) * 100;

  return (
    <div className="space-y-3">
      {/* Side indicator */}
      <div className="flex items-center justify-between">
        <div className={`flex items-center gap-2 ${isLong ? 'text-green-400' : 'text-red-400'}`}>
          {isLong ? (
            <TrendingUp className="w-5 h-5" />
          ) : (
            <TrendingDown className="w-5 h-5" />
          )}
          <span className="text-lg font-bold">{setup.side}</span>
        </div>
        <div className="text-right">
          <div className="text-xs text-gray-500">R:R Ratio</div>
          <div className="text-lg font-bold text-purple-400">
            1:{setup.risk_reward_ratio.toFixed(1)}
          </div>
        </div>
      </div>

      {/* Price levels */}
      <div className="grid grid-cols-3 gap-2">
        {/* Entry */}
        <div className="p-2 bg-blue-500/10 border border-blue-500/30 rounded-lg">
          <div className="text-[10px] text-blue-400 uppercase">Entry</div>
          <div className="text-sm font-medium text-white">
            ${setup.entry_price.toFixed(4)}
          </div>
          <div className={`text-[10px] ${entryDistance >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {entryDistance >= 0 ? '+' : ''}{entryDistance.toFixed(2)}%
          </div>
        </div>

        {/* Stop Loss */}
        <div className="p-2 bg-red-500/10 border border-red-500/30 rounded-lg">
          <div className="text-[10px] text-red-400 uppercase">Stop Loss</div>
          <div className="text-sm font-medium text-white">
            ${setup.stop_loss.toFixed(4)}
          </div>
          <div className="text-[10px] text-red-400">
            -{slDistance.toFixed(2)}%
          </div>
        </div>

        {/* Take Profit */}
        <div className="p-2 bg-green-500/10 border border-green-500/30 rounded-lg">
          <div className="text-[10px] text-green-400 uppercase">Take Profit</div>
          <div className="text-sm font-medium text-white">
            ${setup.take_profit.toFixed(4)}
          </div>
          <div className="text-[10px] text-green-400">
            +{tpDistance.toFixed(2)}%
          </div>
        </div>
      </div>

      {/* Position details */}
      {(setup.position_size_usdt || setup.leverage) && (
        <div className="flex items-center gap-4 text-xs text-gray-400">
          {setup.position_size_usdt && (
            <span>Size: ${setup.position_size_usdt.toFixed(2)}</span>
          )}
          {setup.leverage && (
            <span>Leverage: {setup.leverage}x</span>
          )}
        </div>
      )}
    </div>
  );
}

// ==================== Trailing Stop Plan Component ====================

interface TrailingStopPlanProps {
  isExpanded: boolean;
}

function TrailingStopPlan({ isExpanded }: TrailingStopPlanProps) {
  // Default milestones - in production these would come from settings
  const milestones = [
    { trigger: 2, trail: 0.4, label: 'TP1' },
    { trigger: 3, trail: 0.3, label: 'TP2' },
    { trigger: 4, trail: 0.2, label: 'Final' },
  ];

  if (!isExpanded) return null;

  return (
    <div className="mt-3 p-3 bg-gray-700/30 rounded-lg">
      <div className="flex items-center gap-2 mb-2">
        <Shield className="w-4 h-4 text-purple-400" />
        <span className="text-xs font-medium text-gray-300">Trailing Stop Plan</span>
      </div>
      <div className="space-y-1">
        {milestones.map((m, i) => (
          <div key={i} className="flex items-center justify-between text-xs">
            <div className="flex items-center gap-2">
              <span className="w-12 text-gray-500">{m.label}</span>
              <span className="text-gray-400">at +{m.trigger}% profit</span>
            </div>
            <span className="text-purple-400">trail {m.trail}%</span>
          </div>
        ))}
      </div>
    </div>
  );
}

// ==================== Main Component ====================

export default function VolumeImbalanceCard({
  pattern,
  onExecute,
  onSkip,
  isLoading = false,
  disabled = false,
}: VolumeImbalanceCardProps) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [isExecuting, setIsExecuting] = useState(false);
  const [isSkipping, setIsSkipping] = useState(false);
  const [skipReason, setSkipReason] = useState('');
  const [showSkipDialog, setShowSkipDialog] = useState(false);

  const isActionable = pattern.state === 'READY' && pattern.trade_setup;
  const isDisabled = disabled || isLoading || isExecuting || isSkipping;

  // Handle execute
  const handleExecute = useCallback(async () => {
    setIsExecuting(true);
    try {
      await onExecute(pattern.id);
    } catch (error) {
      console.error('Failed to execute trade:', error);
      // Error will bubble up to parent for handling
      throw error;
    } finally {
      setIsExecuting(false);
    }
  }, [pattern.id, onExecute]);

  // Handle skip confirmation
  const handleConfirmSkip = useCallback(async () => {
    setIsSkipping(true);
    try {
      await onSkip(pattern.id, skipReason || undefined);
      setShowSkipDialog(false);
      setSkipReason('');
    } catch (error) {
      console.error('Failed to skip pattern:', error);
      // Error will bubble up to parent for handling
      throw error;
    } finally {
      setIsSkipping(false);
    }
  }, [pattern.id, skipReason, onSkip]);

  // Time remaining
  const timeRemaining = pattern.expires_at - Date.now();
  const isExpiring = timeRemaining > 0 && timeRemaining < 5 * 60 * 1000; // Less than 5 minutes

  return (
    <div
      className={`
        bg-gray-800 rounded-lg border overflow-hidden transition-all
        ${pattern.state === 'READY'
          ? 'border-green-500/50 shadow-lg shadow-green-500/10'
          : pattern.state === 'CONSOLIDATING'
            ? 'border-yellow-500/30'
            : 'border-gray-700'
        }
      `}
    >
      {/* Header */}
      <div className="flex items-start justify-between p-4">
        <div className="flex-1">
          {/* Symbol and State */}
          <div className="flex items-center gap-3 mb-2">
            <span className="text-lg font-bold text-white">{pattern.symbol}</span>
            <StateBadge state={pattern.state} progress={pattern.consolidation_progress} />
            {pattern.llm_validated && (
              <div className="flex items-center gap-1 px-2 py-0.5 bg-purple-500/20 text-purple-400 border border-purple-500/30 rounded-full">
                <Brain className="w-3 h-3" />
                <span className="text-[10px]">AI OK</span>
              </div>
            )}
          </div>

          {/* Pattern Progress */}
          <div className="flex items-center gap-4 mb-2">
            <PatternSteps steps={pattern.steps} />
            <span className="text-xs text-gray-500">
              {pattern.progress_percent}% complete
            </span>
          </div>

          {/* Meta info */}
          <div className="flex items-center gap-4 text-xs text-gray-500">
            <span className="flex items-center gap-1">
              <BarChart3 className="w-3 h-3" />
              Vol: {pattern.volume_ratio.toFixed(1)}x
            </span>
            <span className="flex items-center gap-1">
              <Clock className="w-3 h-3" />
              {formatDistanceToNow(new Date(pattern.detected_at), { addSuffix: true })}
            </span>
            <span>{pattern.timeframe}</span>
            {isExpiring && (
              <span className="text-yellow-400 animate-pulse">
                Expires {formatDistanceToNow(new Date(pattern.expires_at), { addSuffix: true })}
              </span>
            )}
          </div>
        </div>

        {/* Expand button */}
        <button
          onClick={() => setIsExpanded(!isExpanded)}
          className="p-2 text-gray-400 hover:text-white hover:bg-gray-700 rounded transition-colors"
        >
          {isExpanded ? (
            <ChevronUp className="w-5 h-5" />
          ) : (
            <ChevronDown className="w-5 h-5" />
          )}
        </button>
      </div>

      {/* Trade Setup (when READY) */}
      {isActionable && pattern.trade_setup && (
        <div className="px-4 pb-4">
          <TradeSetupDisplay
            setup={pattern.trade_setup}
            currentPrice={pattern.current_price}
          />
          <TrailingStopPlan isExpanded={isExpanded} />
        </div>
      )}

      {/* Expanded details */}
      {isExpanded && (
        <div className="px-4 pb-4 space-y-3 border-t border-gray-700 pt-3">
          {/* Step details */}
          <div>
            <h4 className="text-xs font-medium text-gray-400 mb-2">Pattern Steps</h4>
            <div className="space-y-2">
              {pattern.steps.map((step, index) => (
                <div
                  key={index}
                  className={`
                    flex items-center justify-between p-2 rounded-lg
                    ${step.status === 'completed'
                      ? 'bg-green-500/10'
                      : step.status === 'in_progress'
                        ? 'bg-yellow-500/10'
                        : 'bg-gray-700/30'
                    }
                  `}
                >
                  <div className="flex items-center gap-2">
                    {step.status === 'completed' ? (
                      <Check className="w-4 h-4 text-green-400" />
                    ) : step.status === 'in_progress' ? (
                      <Loader2 className="w-4 h-4 text-yellow-400 animate-spin" />
                    ) : (
                      <Circle className="w-4 h-4 text-gray-500" />
                    )}
                    <div>
                      <span className="text-sm text-gray-200">{step.name}</span>
                      <p className="text-xs text-gray-500">{step.description}</p>
                    </div>
                  </div>
                  {step.completed_at && (
                    <span className="text-xs text-gray-500">
                      {formatDistanceToNow(new Date(step.completed_at), { addSuffix: true })}
                    </span>
                  )}
                </div>
              ))}
            </div>
          </div>

          {/* LLM Validation reason */}
          {pattern.llm_validation_reason && (
            <div className="p-3 bg-purple-500/10 border border-purple-500/30 rounded-lg">
              <div className="flex items-center gap-2 mb-1">
                <Brain className="w-4 h-4 text-purple-400" />
                <span className="text-xs font-medium text-purple-400">AI Assessment</span>
              </div>
              <p className="text-xs text-gray-300">{pattern.llm_validation_reason}</p>
            </div>
          )}

          {/* HTF Confirmation */}
          {pattern.htf_confirmed !== undefined && (
            <div className={`
              flex items-center gap-2 text-xs
              ${pattern.htf_confirmed ? 'text-green-400' : 'text-yellow-400'}
            `}>
              {pattern.htf_confirmed ? (
                <Check className="w-4 h-4" />
              ) : (
                <Clock className="w-4 h-4" />
              )}
              HTF Confirmation: {pattern.htf_confirmed ? 'Confirmed' : 'Pending'}
            </div>
          )}
        </div>
      )}

      {/* Action Buttons (when READY) */}
      {isActionable && (
        <div className="flex items-center justify-end gap-2 px-4 py-3 bg-gray-700/30 border-t border-gray-700">
          {/* Skip button */}
          {showSkipDialog ? (
            <div className="flex items-center gap-2 flex-1">
              <input
                type="text"
                value={skipReason}
                onChange={(e) => setSkipReason(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') handleConfirmSkip();
                  if (e.key === 'Escape') setShowSkipDialog(false);
                }}
                placeholder="Reason (optional)"
                aria-label="Skip reason"
                className="flex-1 px-3 py-2 text-sm bg-gray-700 border border-gray-600 rounded focus:outline-none focus:border-purple-500"
              />
              <button
                onClick={handleConfirmSkip}
                disabled={isDisabled}
                className="flex items-center gap-1 px-3 py-2 text-sm bg-red-600 hover:bg-red-500 text-white rounded transition-colors disabled:opacity-50"
              >
                {isSkipping ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : (
                  <XCircle className="w-4 h-4" />
                )}
                Confirm
              </button>
              <button
                onClick={() => {
                  setShowSkipDialog(false);
                  setSkipReason('');
                }}
                disabled={isDisabled}
                className="px-3 py-2 text-sm text-gray-400 hover:text-white transition-colors"
              >
                Cancel
              </button>
            </div>
          ) : (
            <>
              <button
                onClick={() => setShowSkipDialog(true)}
                disabled={isDisabled}
                className="flex items-center gap-2 px-4 py-2 text-sm text-gray-300 hover:text-white hover:bg-gray-600 rounded transition-colors disabled:opacity-50"
              >
                <XCircle className="w-4 h-4" />
                Skip
              </button>

              {/* Execute button */}
              <button
                onClick={handleExecute}
                disabled={isDisabled}
                className="flex items-center gap-2 px-6 py-2 text-sm bg-green-600 hover:bg-green-500 text-white rounded font-medium transition-colors disabled:opacity-50"
              >
                {isExecuting ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : (
                  <Play className="w-4 h-4" />
                )}
                Execute Trade
              </button>
            </>
          )}
        </div>
      )}

      {/* Non-actionable states info */}
      {pattern.state === 'EXPIRED' && (
        <div className="px-4 py-3 bg-red-500/10 border-t border-red-500/30">
          <div className="flex items-center gap-2 text-sm text-red-400">
            <AlertTriangle className="w-4 h-4" />
            Pattern expired - no longer valid for execution
          </div>
        </div>
      )}

      {pattern.state === 'CONSOLIDATING' && (
        <div className="px-4 py-3 bg-yellow-500/10 border-t border-yellow-500/30">
          <div className="flex items-center gap-2 text-sm text-yellow-400">
            <Clock className="w-4 h-4" />
            Waiting for consolidation to complete ({pattern.consolidation_progress || 0}% done)
          </div>
        </div>
      )}

      {pattern.state === 'WATCHING' && (
        <div className="px-4 py-3 bg-gray-700/30 border-t border-gray-600">
          <div className="flex items-center gap-2 text-sm text-gray-400">
            <Circle className="w-4 h-4" />
            Monitoring volume imbalance pattern
          </div>
        </div>
      )}
    </div>
  );
}
