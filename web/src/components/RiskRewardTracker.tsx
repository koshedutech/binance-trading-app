// Risk-Reward Stage Tracker Component
// Displays R:R levels (1:1, 1:2, 1:3, 1:4) with trailing stop progression

import React from 'react';
import {
  Shield,
  Target,
  TrendingUp,
  AlertCircle,
  CheckCircle2,
  Circle,
  ArrowRight,
  Lock,
  Unlock,
} from 'lucide-react';
import type { ExpandedPositionData, RRMilestone } from '../types/positionManagement';
import {
  calculateRRMilestones,
  calculateCurrentRR,
  formatPositionPrice,
} from '../types/positionManagement';

// ==================== Types ====================

interface RiskRewardTrackerProps {
  position: ExpandedPositionData;
  compact?: boolean;
}

// ==================== Subcomponents ====================

interface MilestoneNodeProps {
  milestone: RRMilestone;
  isActive: boolean;  // Current price is at this level
  side: 'LONG' | 'SHORT';
}

function MilestoneNode({ milestone, isActive, side }: MilestoneNodeProps) {
  const { level, label, price, achieved } = milestone;

  // Determine colors based on state
  const getColors = () => {
    if (achieved) {
      return {
        bg: 'bg-green-500/30',
        border: 'border-green-500',
        text: 'text-green-400',
        icon: 'text-green-400',
      };
    }
    if (isActive) {
      return {
        bg: 'bg-yellow-500/30',
        border: 'border-yellow-500',
        text: 'text-yellow-400',
        icon: 'text-yellow-400',
      };
    }
    return {
      bg: 'bg-gray-700/50',
      border: 'border-gray-600',
      text: 'text-gray-500',
      icon: 'text-gray-500',
    };
  };

  const colors = getColors();

  // Get icon based on level
  const getIcon = () => {
    if (achieved) return <CheckCircle2 className={`w-4 h-4 ${colors.icon}`} />;
    if (level === 4) return <Target className={`w-4 h-4 ${colors.icon}`} />;
    return <Circle className={`w-4 h-4 ${colors.icon}`} />;
  };

  // What happens at each level
  const getLevelDescription = () => {
    switch (level) {
      case 1: return 'Entry + 1R';
      case 2: return 'SL → Breakeven';
      case 3: return 'SL → 1:1';
      case 4: return 'Take Profit';
      default: return '';
    }
  };

  return (
    <div className="flex flex-col items-center">
      {/* Node circle */}
      <div
        className={`
          w-10 h-10 rounded-full flex items-center justify-center
          ${colors.bg} border-2 ${colors.border}
          ${isActive ? 'ring-2 ring-offset-2 ring-offset-gray-900 ring-yellow-500/50' : ''}
          transition-all duration-300
        `}
      >
        {getIcon()}
      </div>

      {/* Label */}
      <span className={`mt-1 text-xs font-bold ${colors.text}`}>
        {label}
      </span>

      {/* Price */}
      <span className={`text-[10px] ${colors.text} font-mono`}>
        {formatPositionPrice(price)}
      </span>

      {/* Level description */}
      <span className="text-[9px] text-gray-600 mt-0.5 text-center max-w-[60px]">
        {getLevelDescription()}
      </span>
    </div>
  );
}

// ==================== Connector Line ====================

interface ConnectorLineProps {
  filled: boolean;
  progress?: number;  // 0-100 for partial fill
}

function ConnectorLine({ filled, progress }: ConnectorLineProps) {
  return (
    <div className="flex-1 h-1 mx-1 bg-gray-700 rounded-full overflow-hidden relative">
      <div
        className={`h-full rounded-full transition-all duration-500 ${
          filled ? 'bg-green-500' : 'bg-gray-700'
        }`}
        style={{
          width: progress !== undefined ? `${progress}%` : filled ? '100%' : '0%',
        }}
      />
    </div>
  );
}

// ==================== SL Status Section ====================

interface SLStatusProps {
  position: ExpandedPositionData;
  currentRR: number;
  milestones: RRMilestone[];
}

function SLStatus({ position, currentRR, milestones }: SLStatusProps) {
  const trailingStatus = position.trailing_stop_status;

  // Determine current SL location description
  const getSLDescription = () => {
    if (trailingStatus?.moved_to_1r) {
      return { text: 'At 1:1 Level', color: 'text-green-400', locked: '+1R locked' };
    }
    if (trailingStatus?.moved_to_breakeven) {
      return { text: 'At Breakeven', color: 'text-yellow-400', locked: 'Zero risk' };
    }
    return { text: 'At Initial', color: 'text-red-400', locked: 'Full risk' };
  };

  const slDesc = getSLDescription();

  // Determine next move
  const getNextMove = () => {
    if (trailingStatus?.moved_to_1r) {
      // Already at 1R, next is TP at 1:4
      const tp = milestones.find(m => m.level === 4);
      return tp ? { price: tp.price, label: 'Take Profit' } : null;
    }
    if (trailingStatus?.moved_to_breakeven) {
      // At breakeven, next move at 1:3
      const level3 = milestones.find(m => m.level === 3);
      return level3 ? { price: level3.price, label: 'SL → 1:1' } : null;
    }
    // Initial SL, next move at 1:2
    const level2 = milestones.find(m => m.level === 2);
    return level2 ? { price: level2.price, label: 'SL → Breakeven' } : null;
  };

  const nextMove = getNextMove();
  const currentSL = trailingStatus?.current_sl || position.stop_loss;
  const isSynced = trailingStatus?.sl_order_synced ?? false;
  const hasOrderId = trailingStatus?.sl_order_id && trailingStatus.sl_order_id > 0;

  return (
    <div className="mt-4 p-3 bg-gray-800/50 rounded-lg border border-gray-700">
      <div className="flex items-center justify-between mb-2">
        <h5 className="text-xs font-medium text-gray-400 flex items-center gap-1">
          <Shield className="w-3 h-3" />
          Trailing Stop Status
        </h5>
        {/* Binance sync indicator */}
        <div className={`flex items-center gap-1 text-[10px] ${hasOrderId ? 'text-green-400' : 'text-yellow-500'}`}>
          {hasOrderId ? (
            <>
              <CheckCircle2 className="w-3 h-3" />
              Binance SL Active
            </>
          ) : (
            <>
              <AlertCircle className="w-3 h-3" />
              No Binance SL
            </>
          )}
        </div>
      </div>

      <div className="grid grid-cols-3 gap-3">
        {/* Current SL */}
        <div>
          <span className="text-[10px] text-gray-500 block">Current SL</span>
          <span className="text-sm font-mono text-red-400">
            {currentSL ? formatPositionPrice(currentSL) : 'N/A'}
          </span>
          <span className={`text-[10px] ${slDesc.color} block`}>
            {slDesc.text}
          </span>
        </div>

        {/* Locked Profit */}
        <div>
          <span className="text-[10px] text-gray-500 block">Locked</span>
          <span className={`text-sm font-medium ${slDesc.color}`}>
            {slDesc.locked}
          </span>
          <span className="text-[10px] text-gray-500 block flex items-center gap-0.5">
            {trailingStatus?.moved_to_breakeven ? (
              <Lock className="w-2.5 h-2.5" />
            ) : (
              <Unlock className="w-2.5 h-2.5" />
            )}
            {trailingStatus?.moved_to_breakeven ? 'Protected' : 'At Risk'}
          </span>
        </div>

        {/* Next Move */}
        <div>
          <span className="text-[10px] text-gray-500 block">Next Move</span>
          {nextMove ? (
            <>
              <span className="text-sm font-mono text-blue-400">
                {formatPositionPrice(nextMove.price)}
              </span>
              <span className="text-[10px] text-blue-400 block flex items-center gap-0.5">
                <ArrowRight className="w-2.5 h-2.5" />
                {nextMove.label}
              </span>
            </>
          ) : (
            <span className="text-sm text-gray-500">-</span>
          )}
        </div>
      </div>

      {/* Current R:R Display */}
      <div className="mt-3 pt-2 border-t border-gray-700 flex items-center justify-between">
        <span className="text-[10px] text-gray-500">Current R:R Ratio</span>
        <span className={`text-sm font-bold ${
          currentRR >= 4 ? 'text-green-400' :
          currentRR >= 2 ? 'text-yellow-400' :
          currentRR >= 0 ? 'text-gray-400' : 'text-red-400'
        }`}>
          1:{currentRR.toFixed(2)}
        </span>
      </div>
    </div>
  );
}

// ==================== Main Component ====================

export default function RiskRewardTracker({ position, compact = false }: RiskRewardTrackerProps) {
  // Get trailing stop status or use defaults
  const trailingStatus = position.trailing_stop_status;

  // Calculate milestones
  const initialSL = trailingStatus?.initial_sl || position.stop_loss || 0;
  const milestones = calculateRRMilestones(
    position.entry_price,
    initialSL,
    position.side,
    position.current_price,
    trailingStatus?.moved_to_breakeven,
    trailingStatus?.moved_to_1r
  );

  // Calculate current R:R from live price (always recompute for real-time updates).
  // The backend's trailingStatus.current_rr is a stale snapshot from the API fetch
  // and doesn't update with every price tick. Use calculateCurrentRR with the live
  // current_price to ensure the display updates in real-time on every WebSocket tick.
  const currentRR = calculateCurrentRR(
    position.entry_price,
    initialSL,
    position.current_price,
    position.side
  );

  // Determine which milestones are active (current price is between them)
  const getActiveIndex = () => {
    for (let i = milestones.length - 1; i >= 0; i--) {
      if (milestones[i].achieved) return i + 1;
    }
    return 0;
  };

  const activeIndex = getActiveIndex();

  // Calculate progress to next milestone
  const getProgressToNext = () => {
    if (activeIndex >= milestones.length) return 100;
    if (activeIndex === 0) {
      // Between entry and 1:1
      const entryToFirst = milestones[0].price - position.entry_price;
      const currentProgress = position.current_price - position.entry_price;
      if (position.side === 'SHORT') {
        return Math.max(0, Math.min(100, ((position.entry_price - position.current_price) / (position.entry_price - milestones[0].price)) * 100));
      }
      return Math.max(0, Math.min(100, (currentProgress / entryToFirst) * 100));
    }
    // Between two milestones
    const prevMilestone = milestones[activeIndex - 1];
    const nextMilestone = milestones[activeIndex];
    if (!nextMilestone) return 100;

    const range = nextMilestone.price - prevMilestone.price;
    const progress = position.current_price - prevMilestone.price;
    if (position.side === 'SHORT') {
      return Math.max(0, Math.min(100, ((prevMilestone.price - position.current_price) / (prevMilestone.price - nextMilestone.price)) * 100));
    }
    return Math.max(0, Math.min(100, (progress / range) * 100));
  };

  // Calculate ultimate loss amount
  const riskAmount = trailingStatus?.risk_amount || Math.abs(position.entry_price - initialSL) * position.quantity;

  // Render compact version
  if (compact) {
    return (
      <div className="flex items-center gap-2 text-xs">
        <span className="text-gray-500">R:R:</span>
        <span className={`font-bold ${currentRR >= 2 ? 'text-green-400' : currentRR >= 0 ? 'text-yellow-400' : 'text-red-400'}`}>
          1:{currentRR.toFixed(1)}
        </span>
        {milestones.map((m, idx) => (
          <span
            key={m.level}
            className={`w-2 h-2 rounded-full ${m.achieved ? 'bg-green-500' : 'bg-gray-600'}`}
            title={`${m.label}: ${formatPositionPrice(m.price)}`}
          />
        ))}
      </div>
    );
  }

  return (
    <div className="bg-gray-800/30 rounded-lg p-4 border border-gray-700">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <h4 className="text-sm font-medium text-gray-300 flex items-center gap-2">
          <TrendingUp className="w-4 h-4 text-blue-400" />
          Risk-Reward Stages
        </h4>
        <div className="text-right">
          <span className="text-[10px] text-gray-500 block">Ultimate Loss</span>
          <span className="text-sm font-mono text-red-400">
            -${riskAmount.toFixed(2)}
          </span>
        </div>
      </div>

      {/* Loss | Entry | Profit sections */}
      <div className="flex items-center mb-2 px-2">
        <div className="flex-shrink-0 w-16 text-center">
          <span className="text-[10px] text-red-400">LOSS</span>
        </div>
        <div className="flex-1" />
        <div className="flex-shrink-0 w-16 text-center">
          <span className="text-[10px] text-blue-400">ENTRY</span>
        </div>
        <div className="flex-1" />
        <div className="flex-shrink-0 w-16 text-center">
          <span className="text-[10px] text-green-400">PROFIT</span>
        </div>
      </div>

      {/* R:R Level Timeline */}
      <div className="flex items-start justify-between px-2">
        {/* SL indicator on left */}
        <div className="flex flex-col items-center flex-shrink-0">
          <div className="w-10 h-10 rounded-full flex items-center justify-center bg-red-500/20 border-2 border-red-500">
            <AlertCircle className="w-4 h-4 text-red-400" />
          </div>
          <span className="mt-1 text-xs font-bold text-red-400">SL</span>
          <span className="text-[10px] text-red-400 font-mono">
            {formatPositionPrice(initialSL)}
          </span>
          <span className="text-[9px] text-gray-600">-${riskAmount.toFixed(0)}</span>
        </div>

        {/* Connector to 1:1 */}
        <ConnectorLine filled={currentRR >= 1} progress={currentRR < 1 ? Math.max(0, (currentRR / 1) * 100) : undefined} />

        {/* R:R Milestones */}
        {milestones.map((milestone, index) => (
          <React.Fragment key={milestone.level}>
            <MilestoneNode
              milestone={milestone}
              isActive={index === activeIndex - 1 || (currentRR >= milestone.level - 1 && currentRR < milestone.level)}
              side={position.side}
            />
            {index < milestones.length - 1 && (
              <ConnectorLine
                filled={milestones[index + 1].achieved}
                progress={
                  !milestones[index + 1].achieved && milestone.achieved
                    ? getProgressToNext()
                    : undefined
                }
              />
            )}
          </React.Fragment>
        ))}
      </div>

      {/* Current price indicator */}
      <div className="mt-3 flex items-center justify-center gap-2 text-xs">
        <div className="w-2 h-2 bg-white rounded-full animate-pulse" />
        <span className="text-gray-400">Current:</span>
        <span className="font-mono text-white">{formatPositionPrice(position.current_price)}</span>
      </div>

      {/* SL Status Section */}
      <SLStatus position={position} currentRR={currentRR} milestones={milestones} />
    </div>
  );
}

// ==================== Exports ====================

export { MilestoneNode, ConnectorLine, SLStatus };
