// Trade Cycle Position Card - Collapsible position details with analytics
// Moved from FuturesPositionsTable and ChainCard into Trade Cycle section

import { useState, useMemo } from 'react';
import {
  ChevronDown,
  ChevronRight,
  TrendingUp,
  TrendingDown,
  Activity,
  Edit2,
  X,
} from 'lucide-react';
import {
  StageBadge,
  StageTimeline,
} from '../PositionCardExpanded';
import RiskRewardTracker from '../RiskRewardTracker';
import type { ExpandedPositionData, PositionStage } from '../../types/positionManagement';
import type { OrderChain } from './types';
import { futuresApi } from '../../services/futuresApi';
import { useFuturesStore } from '../../store/futuresStore';

interface PositionCardProps {
  chain: OrderChain;
  livePrice: number;
  onChainRefresh: () => void;
}

// Convert frontend OrderChain to ExpandedPositionData for RiskRewardTracker
function buildExpandedPositionData(
  chain: OrderChain,
  livePrice: number,
): ExpandedPositionData {
  const ps = chain.positionState;
  const analytics = chain.positionAnalytics;
  const isLong = chain.positionSide === 'LONG';

  const entryPrice = ps?.entryPrice || 0;
  const entryQty = ps?.entryQuantity || 0;
  const remainingQty = ps?.remainingQuantity || 0;
  const currentPrice = livePrice > 0 ? livePrice : (analytics?.current_price ?? entryPrice);

  const priceChange = isLong
    ? currentPrice - entryPrice
    : entryPrice - currentPrice;
  const unrealizedPnl = priceChange * remainingQty;
  const entryValue = entryPrice * entryQty;
  const unrealizedPnlPercent = entryValue > 0 ? (unrealizedPnl / entryValue) * 100 : 0;

  const slPrice = chain.slOrder?.stopPrice || chain.slOrder?.price || analytics?.stop_loss;

  return {
    symbol: chain.symbol,
    side: isLong ? 'LONG' : 'SHORT',
    entry_price: entryPrice,
    current_price: currentPrice,
    quantity: entryQty,
    leverage: 1,
    unrealized_pnl: unrealizedPnl,
    unrealized_pnl_percent: unrealizedPnlPercent,
    roe: analytics?.roe ?? unrealizedPnlPercent,
    stage: (analytics?.stage ?? 'RISK_ZONE') as PositionStage,
    stage_entry_time: analytics?.stage_entry_time ?? (ps?.entryFilledAt ? new Date(ps.entryFilledAt).getTime() : Date.now()),
    stage_progress_percent: 0,
    stop_loss: slPrice,
    take_profit: chain.tpOrders[0]?.price,
    breakeven_price: analytics?.breakeven_price,
    tp1_price: analytics?.tp1_price ?? chain.tpOrders[0]?.price,
    tp2_price: analytics?.tp2_price ?? chain.tpOrders[1]?.price,
    tp3_price: analytics?.tp3_price ?? chain.tpOrders[2]?.price,
    decision_mode: analytics?.decision_mode ?? 'classic',
    efficiency: analytics?.efficiency,
    classic_scores: analytics?.classic_scores,
    new_engine_scores: analytics?.new_engine_scores,
    entry_time: ps?.entryFilledAt ? new Date(ps.entryFilledAt).getTime() : Date.now(),
    last_updated: ps?.updatedAt ? new Date(ps.updatedAt).getTime() : Date.now(),
    trailing_stop_status: chain.trailingStopStatus ? {
      current_rr: chain.trailingStopStatus.current_rr,
      current_sl: chain.trailingStopStatus.current_stop_loss,
      initial_sl: slPrice || (chain.trailingStopStatus.entry_price - chain.trailingStopStatus.risk_amount),
      risk_amount: chain.trailingStopStatus.risk_amount,
      highest_price: chain.trailingStopStatus.highest_price,
      moved_to_breakeven: chain.trailingStopStatus.at_breakeven,
      moved_to_1r: chain.trailingStopStatus.at_1r,
      milestones: [],
      sl_order_synced: true,
    } : undefined,
  };
}

export default function PositionCard({ chain, livePrice, onChainRefresh }: PositionCardProps) {
  const [expanded, setExpanded] = useState(false);
  const [editingTPSL, setEditingTPSL] = useState(false);
  const [tpValue, setTpValue] = useState('');
  const [slValue, setSlValue] = useState('');
  const [savingTPSL, setSavingTPSL] = useState(false);
  const { closePosition, fetchPositions } = useFuturesStore();

  const ps = chain.positionState;
  const analytics = chain.positionAnalytics;
  const isLong = chain.positionSide === 'LONG';

  const entryPrice = ps?.entryPrice ||
    (chain.entryOrder?.avgPrice && chain.entryOrder.avgPrice > 0 ? chain.entryOrder.avgPrice : chain.entryOrder?.price) || 0;
  const remainingQty = ps?.remainingQuantity || (chain.entryOrder?.executedQty || 0);
  const entryQty = ps?.entryQuantity || (chain.entryOrder?.executedQty || 0);
  const currentPrice = livePrice > 0 ? livePrice : (analytics?.current_price ?? entryPrice);

  const slPrice = chain.slOrder?.stopPrice || chain.slOrder?.price || analytics?.stop_loss || 0;
  const tp1Price = chain.tpOrders?.[0]?.stopPrice || chain.tpOrders?.[0]?.price || analytics?.tp1_price || 0;

  // PnL calculation
  const unrealizedPnl = isLong
    ? (currentPrice - entryPrice) * remainingQty
    : (entryPrice - currentPrice) * remainingQty;
  const entryValue = entryPrice * entryQty;
  const pnlPercent = entryValue > 0 ? (unrealizedPnl / entryValue) * 100 : 0;

  // R:R calculation
  const riskDistance = isLong ? entryPrice - slPrice : slPrice - entryPrice;
  const rewardDistance = isLong ? tp1Price - entryPrice : entryPrice - tp1Price;
  const currentRR = riskDistance > 0
    ? (isLong ? (currentPrice - entryPrice) / riskDistance : (entryPrice - currentPrice) / riskDistance)
    : 0;

  // Build expanded position data for RiskRewardTracker
  const expandedPositionData = useMemo(
    () => buildExpandedPositionData(chain, currentPrice),
    [chain, currentPrice]
  );

  const hasAnalytics = !!analytics;
  const hasTrailingStop = !!chain.trailingStopStatus;

  // Stage config for inline badge
  const stage = analytics?.stage ||
    (isLong ? (currentPrice < entryPrice ? 'RISK_ZONE' : 'BREAKEVEN') : (currentPrice > entryPrice ? 'RISK_ZONE' : 'BREAKEVEN'));

  // Handle close position
  const handleClosePosition = async () => {
    if (window.confirm(`Are you sure you want to close your ${chain.symbol} position?`)) {
      await closePosition(chain.symbol);
      setTimeout(() => {
        onChainRefresh();
        fetchPositions();
      }, 500);
    }
  };

  // Handle save TP/SL
  const handleSaveTPSL = async () => {
    setSavingTPSL(true);
    try {
      await futuresApi.setPositionTPSL(
        chain.symbol,
        chain.positionSide || 'BOTH',
        tpValue ? parseFloat(tpValue) : undefined,
        slValue ? parseFloat(slValue) : undefined,
      );
      setEditingTPSL(false);
      onChainRefresh();
    } catch (err) {
      console.error('Error saving TP/SL:', err);
      alert('Failed to save TP/SL orders');
    } finally {
      setSavingTPSL(false);
    }
  };

  // Start editing
  const startEditTPSL = () => {
    setTpValue(tp1Price > 0 ? tp1Price.toString() : '');
    setSlValue(slPrice > 0 ? slPrice.toString() : '');
    setEditingTPSL(true);
  };

  // Progress bar calculations
  const totalRange = riskDistance + rewardDistance;
  const entryPositionPercent = totalRange > 0 ? (riskDistance / totalRange) * 100 : 20;
  let currentPositionPercent: number;
  if (isLong) {
    const priceRange = tp1Price - slPrice;
    currentPositionPercent = priceRange > 0 ? ((currentPrice - slPrice) / priceRange) * 100 : 50;
  } else {
    const priceRange = slPrice - tp1Price;
    currentPositionPercent = priceRange > 0 ? ((slPrice - currentPrice) / priceRange) * 100 : 50;
  }
  currentPositionPercent = Math.max(-5, Math.min(105, currentPositionPercent));

  return (
    <div className="bg-gray-800/50 rounded-lg border border-gray-700/50 overflow-hidden">
      {/* ==================== COLLAPSED ROW ==================== */}
      <div
        className="px-4 py-3 cursor-pointer select-none hover:bg-gray-700/20 transition-colors"
        onClick={() => setExpanded(!expanded)}
      >
        <div className="flex items-center justify-between gap-3">
          {/* Left: Symbol + Direction + Stage */}
          <div className="flex items-center gap-2 min-w-0 flex-shrink-0">
            {expanded ? (
              <ChevronDown className="w-4 h-4 text-gray-400 flex-shrink-0" />
            ) : (
              <ChevronRight className="w-4 h-4 text-gray-400 flex-shrink-0" />
            )}
            <span className="font-bold text-white text-sm">{chain.symbol.replace('USDT', '')}</span>
            <span className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-bold ${
              isLong ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
            }`}>
              {isLong ? <TrendingUp className="w-3 h-3" /> : <TrendingDown className="w-3 h-3" />}
              {isLong ? 'LONG' : 'SHORT'}
            </span>
            {chain.modeCode && (
              <span className="px-1.5 py-0.5 rounded text-[10px] bg-purple-500/20 text-purple-400 font-bold">
                {chain.modeCode}
              </span>
            )}
            {hasAnalytics && (
              <StageBadge stage={analytics!.stage} compact />
            )}
          </div>

          {/* Center: Key prices */}
          <div className="flex items-center gap-3 text-xs flex-shrink-0">
            <div className="text-center">
              <span className="text-gray-500 block text-[10px] leading-tight">Mark</span>
              <span className="text-yellow-400 font-mono font-bold">${currentPrice.toFixed(4)}</span>
            </div>
            <div className="text-center">
              <span className="text-gray-500 block text-[10px] leading-tight">Entry</span>
              <span className="text-gray-300 font-mono">${entryPrice.toFixed(4)}</span>
            </div>
            <div className="text-center">
              <span className="text-gray-500 block text-[10px] leading-tight">SL</span>
              <span className="text-red-400 font-mono">{slPrice > 0 ? `$${slPrice.toFixed(4)}` : '-'}</span>
            </div>
            <div className="text-center">
              <span className="text-gray-500 block text-[10px] leading-tight">TP</span>
              <span className="text-green-400 font-mono">{tp1Price > 0 ? `$${tp1Price.toFixed(4)}` : '-'}</span>
            </div>
          </div>

          {/* Right: R:R compact + PnL */}
          <div className="flex items-center gap-3 flex-shrink-0">
            {/* Compact R:R indicator */}
            {(hasTrailingStop || slPrice > 0) && (
              <RiskRewardTracker position={expandedPositionData} compact />
            )}

            {/* PnL */}
            <div className="text-right min-w-[80px]">
              <div className={`font-bold text-sm ${unrealizedPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                {unrealizedPnl >= 0 ? '+' : ''}{unrealizedPnl.toFixed(2)}
              </div>
              <div className={`text-[10px] ${pnlPercent >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                {pnlPercent >= 0 ? '+' : ''}{pnlPercent.toFixed(2)}%
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* ==================== EXPANDED DETAILS ==================== */}
      {expanded && (
        <div className="border-t border-gray-700/50 p-4 space-y-4">
          {/* Position Progress Bar (SL on left, TP on right, entry in middle) */}
          {slPrice > 0 && tp1Price > 0 && (
            <div>
              {/* Labels */}
              <div className="relative flex items-center text-xs text-gray-500 mb-1 h-4">
                <span className="absolute left-0 text-red-400">
                  SL ${slPrice.toFixed(4)}
                </span>
                <span
                  className="absolute text-blue-400"
                  style={{ left: `${entryPositionPercent}%`, transform: 'translateX(-50%)' }}
                >
                  Entry ${entryPrice.toFixed(4)}
                </span>
                <span className="absolute right-0 text-green-400">
                  TP ${tp1Price.toFixed(4)}
                </span>
              </div>

              {/* Bar */}
              <div className="relative h-3 bg-gray-700 rounded-full overflow-hidden">
                <div className="absolute left-0 h-full bg-red-500/30" style={{ width: `${entryPositionPercent}%` }} />
                <div className="absolute h-full bg-green-500/30" style={{ left: `${entryPositionPercent}%`, width: `${100 - entryPositionPercent}%` }} />
                <div className="absolute top-0 bottom-0 w-0.5 bg-blue-400 z-10" style={{ left: `${entryPositionPercent}%`, transform: 'translateX(-50%)' }} />
                <div className="absolute top-0 bottom-0 w-1.5 bg-white rounded shadow-lg shadow-white/50 transition-all duration-300 z-20" style={{ left: `${currentPositionPercent}%`, transform: 'translateX(-50%)' }} />
              </div>

              {/* R:R label */}
              <div className="flex justify-between mt-1 text-xs">
                <span className="text-gray-500">
                  {isLong && currentPrice < entryPrice ? `${((entryPrice - currentPrice) / riskDistance * 100).toFixed(0)}% to SL` : ''}
                  {!isLong && currentPrice > entryPrice ? `${((currentPrice - entryPrice) / riskDistance * 100).toFixed(0)}% to SL` : ''}
                </span>
                <span className="text-gray-500">
                  R:R {riskDistance > 0 ? `1:${currentRR.toFixed(1)}` : '-'}
                </span>
                <span className="text-gray-500">
                  {isLong && currentPrice > entryPrice ? `${((currentPrice - entryPrice) / rewardDistance * 100).toFixed(0)}% to TP` : ''}
                  {!isLong && currentPrice < entryPrice ? `${((entryPrice - currentPrice) / rewardDistance * 100).toFixed(0)}% to TP` : ''}
                </span>
              </div>
            </div>
          )}

          {/* R:R Milestone Tracker (full mode with milestones, trailing stop) */}
          {(hasTrailingStop || slPrice > 0) && (
            <RiskRewardTracker position={expandedPositionData} />
          )}

          {/* Stage Timeline */}
          {hasAnalytics && (
            <div className="bg-gray-900/50 p-3 rounded-lg border border-gray-700">
              <div className="flex items-center gap-2 mb-3">
                <Activity className="w-4 h-4 text-purple-400" />
                <span className="text-sm font-medium text-gray-300">Stage Progress</span>
                <StageBadge stage={analytics!.stage} compact />
              </div>
              <StageTimeline
                currentStage={analytics!.stage}
                stageEntryTime={analytics!.stage_entry_time}
              />
            </div>
          )}

          {/* Position Metrics Grid */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-2 text-sm">
            <div className="bg-gray-700/30 rounded p-2 text-center">
              <div className="text-xs text-gray-500">Entry Value</div>
              <div className="text-gray-300 font-mono">${entryValue.toFixed(2)}</div>
            </div>
            <div className="bg-gray-700/30 rounded p-2 text-center">
              <div className="text-xs text-gray-500">Quantity</div>
              <div className="text-gray-300 font-mono">
                {remainingQty.toFixed(4)}
                {remainingQty !== entryQty && (
                  <span className="text-gray-500 text-xs"> / {entryQty.toFixed(4)}</span>
                )}
              </div>
            </div>
            <div className="bg-gray-700/30 rounded p-2 text-center">
              <div className="text-xs text-gray-500">Entry Fees</div>
              <div className="text-orange-400 font-mono">${(ps?.entryFees || 0).toFixed(4)}</div>
            </div>
            <div className="bg-gray-700/30 rounded p-2 text-center">
              <div className="text-xs text-gray-500">Breakeven</div>
              <div className="text-yellow-400 font-mono">
                {analytics?.breakeven_price ? `$${analytics.breakeven_price.toFixed(4)}` : '-'}
              </div>
            </div>
          </div>

          {/* Expected TP Profit & SL Loss */}
          {tp1Price > 0 && slPrice > 0 && entryPrice > 0 && remainingQty > 0 && (
            <div className="flex items-center justify-between text-xs px-1">
              {(() => {
                const feeRate = 0.0004;
                const entryFee = entryPrice * remainingQty * feeRate;
                const tpPriceChange = isLong ? tp1Price - entryPrice : entryPrice - tp1Price;
                const tpGrossPnl = tpPriceChange * remainingQty;
                const tpExitFee = tp1Price * remainingQty * feeRate;
                const expectedTpProfit = tpGrossPnl - entryFee - tpExitFee;
                const slPriceChange = isLong ? slPrice - entryPrice : entryPrice - slPrice;
                const slGrossPnl = slPriceChange * remainingQty;
                const slExitFee = slPrice * remainingQty * feeRate;
                const expectedSlLoss = slGrossPnl - entryFee - slExitFee;

                return (
                  <>
                    <span className="text-green-400">TP: +${expectedTpProfit.toFixed(2)}</span>
                    <span className="text-gray-600">|</span>
                    <span className="text-red-400">SL: {expectedSlLoss >= 0 ? '+' : ''}${expectedSlLoss.toFixed(2)}</span>
                  </>
                );
              })()}
            </div>
          )}

          {/* Actions: Edit TP/SL and Close Position */}
          <div className="flex items-center justify-between pt-3 border-t border-gray-700/50">
            <div className="flex items-center gap-2">
              {editingTPSL ? (
                <div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
                  <input
                    type="number"
                    value={tpValue}
                    onChange={(e) => setTpValue(e.target.value)}
                    onFocus={(e) => e.target.select()}
                    placeholder="TP"
                    className="w-24 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-xs"
                  />
                  <input
                    type="number"
                    value={slValue}
                    onChange={(e) => setSlValue(e.target.value)}
                    onFocus={(e) => e.target.select()}
                    placeholder="SL"
                    className="w-24 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-xs"
                  />
                  <button
                    onClick={(e) => { e.stopPropagation(); handleSaveTPSL(); }}
                    disabled={savingTPSL}
                    className="px-2 py-1 bg-green-600 hover:bg-green-500 text-white rounded text-xs"
                  >
                    {savingTPSL ? '...' : 'Save'}
                  </button>
                  <button
                    onClick={(e) => { e.stopPropagation(); setEditingTPSL(false); }}
                    className="px-2 py-1 bg-gray-600 hover:bg-gray-500 text-white rounded text-xs"
                  >
                    Cancel
                  </button>
                </div>
              ) : (
                <button
                  onClick={(e) => { e.stopPropagation(); startEditTPSL(); }}
                  className="px-3 py-1.5 bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 rounded text-xs flex items-center gap-1"
                >
                  <Edit2 className="w-3 h-3" />
                  Edit TP/SL
                </button>
              )}
            </div>

            <button
              onClick={(e) => { e.stopPropagation(); handleClosePosition(); }}
              className="px-3 py-1.5 bg-red-600/20 hover:bg-red-600/30 text-red-400 rounded text-xs flex items-center gap-1"
            >
              <X className="w-3 h-3" />
              Close Position
            </button>
          </div>

          {/* Chain ID Footer */}
          <div className="text-xs text-gray-500 flex items-center justify-between">
            <span>Chain: {chain.chainId}</span>
            <span className={ps?.status === 'ACTIVE' ? 'text-green-400' : ps?.status === 'PARTIAL' ? 'text-yellow-400' : 'text-gray-400'}>
              {ps?.status || 'ACTIVE'}
            </span>
          </div>
        </div>
      )}
    </div>
  );
}
