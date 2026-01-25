// Pattern Progress Component
// Epic 14: Chain Trading System - Story 14.13: Frontend UI Enhancement
// Displays pattern step visualization (1 -> 2 -> 3) with progress indicators

import React from 'react';
import {
  Check,
  Circle,
  Loader2,
  XCircle,
  ArrowRight,
} from 'lucide-react';
import type {
  PatternStatus,
  CoinMatch,
  PATTERN_STATUS_COLORS,
} from '../../types/entryDecision';

// ==================== Interfaces ====================

interface PatternProgressProps {
  /** Current step (1-based) */
  currentStep: number;
  /** Total steps in pattern */
  totalSteps: number;
  /** Pattern status */
  status: PatternStatus;
  /** Optional progress details (e.g., "3/6 candles") */
  details?: string;
  /** Size variant */
  size?: 'sm' | 'md' | 'lg';
  /** Show step labels */
  showLabels?: boolean;
}

interface PatternProgressFromCoinProps {
  /** Coin match data */
  coin: CoinMatch;
  /** Size variant */
  size?: 'sm' | 'md' | 'lg';
  /** Show step labels */
  showLabels?: boolean;
}

// ==================== Size Configurations ====================

const sizeConfig = {
  sm: {
    circle: 'w-5 h-5',
    icon: 'w-2.5 h-2.5',
    text: 'text-[9px]',
    connector: 'w-3 h-0.5',
    gap: 'gap-0.5',
  },
  md: {
    circle: 'w-6 h-6',
    icon: 'w-3 h-3',
    text: 'text-[10px]',
    connector: 'w-4 h-0.5',
    gap: 'gap-1',
  },
  lg: {
    circle: 'w-8 h-8',
    icon: 'w-4 h-4',
    text: 'text-xs',
    connector: 'w-6 h-0.5',
    gap: 'gap-1.5',
  },
};

// ==================== Step Labels ====================

const STEP_LABELS: Record<number, string> = {
  1: 'Spike',
  2: 'Consolidate',
  3: 'Ready',
};

// ==================== Helper Functions ====================

/**
 * Get step state based on current progress
 */
function getStepState(
  stepNumber: number,
  currentStep: number,
  status: PatternStatus
): 'completed' | 'current' | 'pending' | 'failed' {
  if (status === 'failed' || status === 'expired') {
    if (stepNumber < currentStep) return 'completed';
    if (stepNumber === currentStep) return 'failed';
    return 'pending';
  }

  if (stepNumber < currentStep) return 'completed';
  if (stepNumber === currentStep) return 'current';
  return 'pending';
}

/**
 * Get step styling based on state
 */
function getStepStyles(state: 'completed' | 'current' | 'pending' | 'failed'): {
  bg: string;
  border: string;
  text: string;
} {
  switch (state) {
    case 'completed':
      return {
        bg: 'bg-green-500/20',
        border: 'border-green-500',
        text: 'text-green-400',
      };
    case 'current':
      return {
        bg: 'bg-yellow-500/20',
        border: 'border-yellow-500',
        text: 'text-yellow-400',
      };
    case 'failed':
      return {
        bg: 'bg-red-500/20',
        border: 'border-red-500',
        text: 'text-red-400',
      };
    case 'pending':
    default:
      return {
        bg: 'bg-gray-700/50',
        border: 'border-gray-600',
        text: 'text-gray-500',
      };
  }
}

// ==================== Main Component ====================

export default function PatternProgress({
  currentStep,
  totalSteps,
  status,
  details,
  size = 'md',
  showLabels = false,
}: PatternProgressProps) {
  const config = sizeConfig[size];
  const steps = Array.from({ length: totalSteps }, (_, i) => i + 1);

  return (
    <div className="flex flex-col items-start">
      {/* Step indicators */}
      <div className={`flex items-center ${config.gap}`}>
        {steps.map((stepNum, index) => {
          const state = getStepState(stepNum, currentStep, status);
          const styles = getStepStyles(state);

          return (
            <React.Fragment key={stepNum}>
              {/* Step circle */}
              <div
                className={`
                  flex items-center justify-center rounded-full border-2 transition-colors
                  ${config.circle} ${styles.bg} ${styles.border} ${styles.text}
                  ${state === 'current' ? 'animate-pulse' : ''}
                `}
                title={`Step ${stepNum}: ${STEP_LABELS[stepNum] || `Step ${stepNum}`}`}
              >
                {state === 'completed' && (
                  <Check className={config.icon} />
                )}
                {state === 'current' && (
                  <Loader2 className={`${config.icon} animate-spin`} />
                )}
                {state === 'failed' && (
                  <XCircle className={config.icon} />
                )}
                {state === 'pending' && (
                  <span className={`font-medium ${config.text}`}>{stepNum}</span>
                )}
              </div>

              {/* Connector line */}
              {index < steps.length - 1 && (
                <div
                  className={`
                    ${config.connector} mx-0.5 transition-colors
                    ${state === 'completed' ? 'bg-green-500' : 'bg-gray-600'}
                  `}
                />
              )}
            </React.Fragment>
          );
        })}
      </div>

      {/* Labels row */}
      {showLabels && (
        <div className={`flex items-center ${config.gap} mt-1`}>
          {steps.map((stepNum, index) => {
            const state = getStepState(stepNum, currentStep, status);
            const styles = getStepStyles(state);

            return (
              <React.Fragment key={`label-${stepNum}`}>
                <div
                  className={`${config.circle} flex items-center justify-center`}
                  style={{ visibility: 'hidden', height: 0, overflow: 'hidden' }}
                >
                  {/* Spacer to align with circles */}
                </div>
                <span className={`${config.text} ${styles.text} text-center w-12 -ml-3`}>
                  {STEP_LABELS[stepNum] || `Step ${stepNum}`}
                </span>
                {index < steps.length - 1 && (
                  <div className={`${config.connector} mx-0.5`} style={{ visibility: 'hidden' }} />
                )}
              </React.Fragment>
            );
          })}
        </div>
      )}

      {/* Details text */}
      {details && (
        <div className={`${config.text} text-gray-500 mt-1`}>
          {details}
        </div>
      )}
    </div>
  );
}

// ==================== From Coin Match Variant ====================

/**
 * PatternProgress component that accepts a CoinMatch object
 */
export function PatternProgressFromCoin({
  coin,
  size = 'md',
  showLabels = false,
}: PatternProgressFromCoinProps) {
  // If no pattern data, return placeholder
  if (!coin.status || coin.step === undefined) {
    return (
      <div className="text-xs text-gray-500">
        No pattern data
      </div>
    );
  }

  return (
    <PatternProgress
      currentStep={coin.step}
      totalSteps={3} // Default to 3 steps for volume imbalance
      status={coin.status}
      details={coin.details}
      size={size}
      showLabels={showLabels}
    />
  );
}

// ==================== Compact Inline Variant ====================

interface PatternProgressCompactProps {
  /** Current step (1-based) */
  currentStep: number;
  /** Total steps in pattern */
  totalSteps: number;
  /** Pattern status */
  status: PatternStatus;
}

/**
 * Compact inline pattern progress display
 */
export function PatternProgressCompact({
  currentStep,
  totalSteps,
  status,
}: PatternProgressCompactProps) {
  const statusColors: Record<PatternStatus, string> = {
    watching: 'text-gray-400',
    accumulation: 'text-blue-400',
    consolidating: 'text-yellow-400',
    ready: 'text-green-400',
    failed: 'text-red-400',
    expired: 'text-gray-500',
  };

  const statusLabels: Record<PatternStatus, string> = {
    watching: 'Watching',
    accumulation: 'Accumulating',
    consolidating: 'Consolidating',
    ready: 'Ready',
    failed: 'Failed',
    expired: 'Expired',
  };

  return (
    <div className="flex items-center gap-2">
      <span className={`text-xs font-medium ${statusColors[status]}`}>
        Step {currentStep}/{totalSteps}
      </span>
      <ArrowRight className="w-3 h-3 text-gray-600" />
      <span className={`text-xs ${statusColors[status]}`}>
        {statusLabels[status]}
      </span>
    </div>
  );
}
