import { useEffect, useState, useCallback } from 'react';
import { futuresApi, type ModeFullConfig } from '../services/futuresApi';
import { wsService } from '../services/websocket';
import type { WSEvent, GinieStatusPayload } from '../types';
import CollapsibleCard from './CollapsibleCard';
import {
  Repeat,
  Eye,
  TrendingUp,
  TrendingDown,
  ArrowLeftRight,
} from 'lucide-react';

interface Position {
  symbol: string;
  side: 'LONG' | 'SHORT';
  mode: string;
  entry_price: number;
  original_qty: number;
  remaining_qty: number;
  unrealized_pnl: number;
  realized_pnl: number;
  current_tp_level: number;
  take_profits?: Array<{
    level: number;
    status: string;
    percent: number;
    gain_pct: number;
  }>;
}

export default function PositionOptimizationMonitor() {
  const [positions, setPositions] = useState<Position[]>([]);
  const [modeConfigs, setModeConfigs] = useState<Record<string, ModeFullConfig>>({});
  const [activeTab, setActiveTab] = useState<'monitor' | 'hedge'>('monitor');
  const [isExpanded, setIsExpanded] = useState(false);
  const [loading, setLoading] = useState(true);

  // Fetch autopilot status for positions
  const fetchPositions = useCallback(async () => {
    try {
      const data = await futuresApi.getGinieAutopilotStatus();
      if (data?.positions) {
        setPositions(data.positions);
      }
    } catch (err) {
      console.error('Failed to fetch autopilot positions:', err);
    }
  }, []);

  // Fetch mode configs for hedge settings
  const fetchModeConfigs = useCallback(async () => {
    try {
      const modes = ['ultra_fast', 'scalp', 'swing', 'position'];
      const configs: Record<string, ModeFullConfig> = {};
      for (const mode of modes) {
        const config = await futuresApi.getModeConfig(mode);
        if (config) configs[mode] = config;
      }
      setModeConfigs(configs);
    } catch (err) {
      console.error('Failed to fetch mode configs:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  // Initial fetch
  useEffect(() => {
    fetchPositions();
    fetchModeConfigs();
  }, [fetchPositions, fetchModeConfigs]);

  // WebSocket subscription for real-time updates
  useEffect(() => {
    const handleGinieUpdate = (event: WSEvent) => {
      const status = event.data.status as GinieStatusPayload;
      if (status?.positions) {
        setPositions(status.positions as Position[]);
      }
    };

    wsService.subscribe('GINIE_STATUS_UPDATE', handleGinieUpdate);
    return () => {
      wsService.unsubscribe('GINIE_STATUS_UPDATE', handleGinieUpdate);
    };
  }, []);

  // Check if any mode has hedge enabled
  const hasHedgeEnabled = Object.values(modeConfigs).some(
    cfg => cfg?.position_optimization?.hedge_mode_enabled
  );

  return (
    <CollapsibleCard
      title="Position Optimization Monitor"
      icon={<Repeat className="w-4 h-4" />}
      defaultExpanded={false}
      badge={positions.length > 0 ? `${positions.length}` : undefined}
      badgeColor="purple"
    >
      <div className="space-y-3">
        {/* Tab Navigation */}
        <div className="flex gap-1 border-b border-gray-700 pb-1">
          <button
            onClick={() => setActiveTab('monitor')}
            className={`px-2 py-1 text-[10px] rounded-t transition-colors ${
              activeTab === 'monitor'
                ? 'bg-purple-900/50 text-purple-400 border border-purple-700 border-b-0'
                : 'text-gray-400 hover:text-gray-300 hover:bg-gray-700/30'
            }`}
          >
            <Eye className="w-3 h-3 inline mr-1" />
            TP & Rebuy
          </button>
          {hasHedgeEnabled && (
            <button
              onClick={() => setActiveTab('hedge')}
              className={`px-2 py-1 text-[10px] rounded-t transition-colors ${
                activeTab === 'hedge'
                  ? 'bg-orange-900/50 text-orange-400 border border-orange-700 border-b-0'
                  : 'text-gray-400 hover:text-gray-300 hover:bg-gray-700/30'
              }`}
            >
              <TrendingUp className="w-3 h-3 inline mr-1" />
              Hedge
            </button>
          )}
        </div>

        {/* Monitor Tab */}
        {activeTab === 'monitor' && (
          <div className="space-y-2">
            {positions.length > 0 ? (
              [...positions]
                .sort((a, b) => {
                  const aProgress = a.current_tp_level || 0;
                  const bProgress = b.current_tp_level || 0;
                  if (bProgress !== aProgress) return bProgress - aProgress;
                  return a.symbol.localeCompare(b.symbol);
                })
                .map(pos => {
                  const originalValue = pos.entry_price * pos.original_qty;
                  const currentPnlPct = originalValue > 0 ? (pos.unrealized_pnl / originalValue) * 100 : 0;

                  return (
                    <div key={pos.symbol} className="bg-gray-700/50 rounded p-2 space-y-1.5">
                      {/* Header Row */}
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-1.5">
                          <span className={`text-[9px] px-1.5 py-0.5 rounded uppercase font-medium ${
                            pos.mode === 'scalp' ? 'bg-yellow-900/50 text-yellow-400' :
                            pos.mode === 'swing' ? 'bg-blue-900/50 text-blue-400' :
                            pos.mode === 'position' ? 'bg-green-900/50 text-green-400' :
                            'bg-red-900/50 text-red-400'
                          }`}>
                            {pos.mode}
                          </span>
                          <span className="font-medium text-sm text-gray-200">{pos.symbol}</span>
                          {pos.side === 'LONG' ? (
                            <TrendingUp className="w-3.5 h-3.5 text-green-400" />
                          ) : (
                            <TrendingDown className="w-3.5 h-3.5 text-red-400" />
                          )}
                        </div>
                        <div className={`text-xs font-medium ${pos.unrealized_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {currentPnlPct >= 0 ? '+' : ''}{currentPnlPct.toFixed(2)}% (${pos.unrealized_pnl?.toFixed(2) || '0.00'})
                        </div>
                      </div>

                      {/* Position Value Summary */}
                      <div className="flex items-center justify-between text-[9px] text-gray-400 px-1">
                        <span>Entry: ${pos.entry_price?.toFixed(4)} × {pos.original_qty?.toFixed(4)} = ${(pos.entry_price * pos.original_qty).toFixed(2)}</span>
                        <span className="text-purple-400">Holding: ${(pos.remaining_qty * pos.entry_price).toFixed(2)}</span>
                      </div>

                      {/* TP Progress Cards */}
                      <div className="grid grid-cols-3 gap-1">
                        {(() => {
                          const tp1 = pos.take_profits?.find(t => t.level === 1);
                          const tp2 = pos.take_profits?.find(t => t.level === 2);
                          const tp3 = pos.take_profits?.find(t => t.level === 3);
                          const tp1SellPct = tp1?.percent || 30;
                          const tp2SellPct = tp2?.percent || 30;
                          const tp3SellPct = tp3?.percent || 20;
                          const totalTPSellPct = tp1SellPct + tp2SellPct + tp3SellPct;
                          const trailingRemainingPct = Math.max(0, 100 - totalTPSellPct);

                          return [1, 2, 3].map(level => {
                            const tp = pos.take_profits?.find(t => t.level === level);
                            const isHit = tp?.status === 'hit' || (pos.current_tp_level >= level);
                            const targetPct = tp?.gain_pct || 0;
                            const progressPct = targetPct > 0 ? Math.min(100, Math.max(0, (currentPnlPct / targetPct) * 100)) : 0;
                            const distanceToTarget = targetPct - currentPnlPct;
                            const sellPct = tp?.percent || (level === 1 ? tp1SellPct : level === 2 ? tp2SellPct : tp3SellPct);
                            const qtyToSell = pos.original_qty * sellPct / 100;
                            const cumulativeSoldPct = level === 1 ? tp1SellPct :
                                                      level === 2 ? tp1SellPct + tp2SellPct :
                                                      tp1SellPct + tp2SellPct + tp3SellPct;
                            const remainingPct = 100 - cumulativeSoldPct;
                            const remainingAfterTP = pos.original_qty * remainingPct / 100;
                            const profitValue = qtyToSell * pos.entry_price * (targetPct / 100);

                            return (
                              <div key={level} className={`p-1.5 rounded ${isHit ? 'bg-green-900/30 border border-green-700/50' : 'bg-gray-800/50'}`}>
                                <div className="flex items-center justify-between mb-1">
                                  <span className={`text-[10px] font-medium ${isHit ? 'text-green-400' : 'text-gray-400'}`}>
                                    TP{level} <span className="text-[8px] opacity-70">({sellPct}%)</span>
                                  </span>
                                  <span className={`text-[9px] ${isHit ? 'text-green-400' : 'text-gray-500'}`}>
                                    @{targetPct.toFixed(1)}%
                                  </span>
                                </div>

                                <div className="h-1.5 bg-gray-700 rounded overflow-hidden mb-1">
                                  <div
                                    className={`h-full transition-all ${isHit ? 'bg-green-500' : currentPnlPct > 0 ? 'bg-blue-500' : 'bg-gray-600'}`}
                                    style={{ width: `${isHit ? 100 : progressPct}%` }}
                                  />
                                </div>

                                {isHit ? (
                                  <div className="text-[8px] space-y-0.5">
                                    <div className="text-green-400 font-medium">SOLD {sellPct}%</div>
                                    <div className="text-green-300">{qtyToSell.toFixed(4)} @ ${(qtyToSell * pos.entry_price).toFixed(2)}</div>
                                    <div className="text-green-300">+${profitValue.toFixed(2)} profit</div>
                                    {level === 3 && trailingRemainingPct > 0 ? (
                                      <div className="text-purple-400">Trail: {trailingRemainingPct}% ({(pos.original_qty * trailingRemainingPct / 100).toFixed(4)})</div>
                                    ) : (
                                      <div className="text-gray-400">Left: {remainingPct}% ({remainingAfterTP.toFixed(4)})</div>
                                    )}
                                  </div>
                                ) : (
                                  <div className="text-[8px] space-y-0.5">
                                    <div className="text-gray-400">
                                      {currentPnlPct > 0
                                        ? `${progressPct.toFixed(0)}% (${distanceToTarget.toFixed(2)}% to go)`
                                        : `${Math.abs(distanceToTarget).toFixed(2)}% to target`
                                      }
                                    </div>
                                    <div className="text-gray-500">Sell: {sellPct}% = {qtyToSell.toFixed(4)}</div>
                                    {level === 3 && trailingRemainingPct > 0 ? (
                                      <div className="text-purple-400/70">Trail: {trailingRemainingPct}% remains</div>
                                    ) : (
                                      <div className="text-gray-500">Keep: {remainingPct}% = {remainingAfterTP.toFixed(4)}</div>
                                    )}
                                  </div>
                                )}
                              </div>
                            );
                          });
                        })()}
                      </div>

                      {/* Summary Row */}
                      <div className="flex items-center justify-between text-[9px] pt-1 border-t border-gray-700/50">
                        {pos.realized_pnl > 0 ? (
                          <span className="text-green-400">Realized: +${pos.realized_pnl.toFixed(2)}</span>
                        ) : (
                          <span className="text-gray-500">No profit realized yet</span>
                        )}
                        {pos.original_qty > 0 && (
                          <span className="text-purple-400">
                            Holding: {pos.remaining_qty.toFixed(4)} (${(pos.remaining_qty * pos.entry_price).toFixed(2)}) - {Math.round((pos.remaining_qty / pos.original_qty) * 100)}%
                          </span>
                        )}
                      </div>
                    </div>
                  );
                })
            ) : (
              <div className="p-3 text-center text-xs text-gray-500">
                No active positions
              </div>
            )}
          </div>
        )}

        {/* Hedge Tab */}
        {activeTab === 'hedge' && (
          <div className="space-y-2">
            <div className="px-2 py-1.5 bg-purple-900/20 border border-purple-700/30 rounded text-[10px] text-purple-400">
              Positions eligible for hedging. Hedge triggers when position goes into loss.
            </div>

            {positions.length > 0 ? (
              [...positions]
                .sort((a, b) => (a.unrealized_pnl || 0) - (b.unrealized_pnl || 0))
                .map(pos => {
                  const originalValue = pos.entry_price * pos.original_qty;
                  const currentPnlPct = originalValue > 0 ? (pos.unrealized_pnl / originalValue) * 100 : 0;
                  const remainingValue = pos.remaining_qty * pos.entry_price;
                  const soldQty = pos.original_qty - pos.remaining_qty;
                  const soldValue = soldQty * pos.entry_price;
                  const modeConfig = modeConfigs[pos.mode];
                  const hedgeEnabled = modeConfig?.position_optimization?.hedge_mode_enabled ?? false;
                  const isInLoss = (pos.unrealized_pnl || 0) < 0;

                  return (
                    <div key={pos.symbol} className={`rounded p-2 space-y-1.5 ${
                      isInLoss ? 'bg-red-900/20 border border-red-700/30' : 'bg-gray-700/50'
                    }`}>
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-1.5">
                          <span className={`text-[9px] px-1.5 py-0.5 rounded uppercase font-medium ${
                            pos.mode === 'scalp' ? 'bg-yellow-900/50 text-yellow-400' :
                            pos.mode === 'swing' ? 'bg-blue-900/50 text-blue-400' :
                            pos.mode === 'position' ? 'bg-green-900/50 text-green-400' :
                            'bg-red-900/50 text-red-400'
                          }`}>
                            {pos.mode}
                          </span>
                          <span className="font-medium text-sm text-gray-200">{pos.symbol}</span>
                          {pos.side === 'LONG' ? (
                            <TrendingUp className="w-3.5 h-3.5 text-green-400" />
                          ) : (
                            <TrendingDown className="w-3.5 h-3.5 text-red-400" />
                          )}
                          {hedgeEnabled ? (
                            isInLoss ? (
                              <span className="text-[8px] px-1 py-0.5 rounded bg-orange-900/50 text-orange-400 border border-orange-700">
                                HEDGE READY
                              </span>
                            ) : (
                              <span className="text-[8px] px-1 py-0.5 rounded bg-gray-700/50 text-gray-400">
                                Hedge On Profit
                              </span>
                            )
                          ) : (
                            <span className="text-[8px] px-1 py-0.5 rounded bg-gray-700/50 text-gray-500">
                              No Hedge
                            </span>
                          )}
                        </div>
                        <div className={`text-xs font-medium ${pos.unrealized_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {currentPnlPct >= 0 ? '+' : ''}{currentPnlPct.toFixed(2)}% (${pos.unrealized_pnl?.toFixed(2) || '0.00'})
                        </div>
                      </div>

                      <div className="grid grid-cols-3 gap-2 text-[9px]">
                        <div className="bg-gray-800/50 rounded p-1.5">
                          <div className="text-gray-500">Entry Price</div>
                          <div className="text-gray-200 font-medium">${pos.entry_price?.toFixed(4)}</div>
                        </div>
                        <div className="bg-gray-800/50 rounded p-1.5">
                          <div className="text-gray-500">Original Qty</div>
                          <div className="text-gray-200 font-medium">{pos.original_qty?.toFixed(4)}</div>
                          <div className="text-gray-400">(${originalValue.toFixed(2)})</div>
                        </div>
                        <div className="bg-gray-800/50 rounded p-1.5">
                          <div className="text-gray-500">Current Qty</div>
                          <div className="text-purple-400 font-medium">{pos.remaining_qty?.toFixed(4)}</div>
                          <div className="text-purple-300">(${remainingValue.toFixed(2)})</div>
                        </div>
                      </div>

                      <div className="flex items-center justify-between text-[9px] pt-1 border-t border-gray-700/50">
                        {soldQty > 0 ? (
                          <span className="text-green-400">Sold: {soldQty.toFixed(4)} (${soldValue.toFixed(2)})</span>
                        ) : (
                          <span className="text-gray-500">Nothing sold yet</span>
                        )}
                        <span className="text-purple-400">
                          Remaining: {pos.remaining_qty?.toFixed(4)} (${remainingValue.toFixed(2)})
                        </span>
                      </div>

                      {pos.realized_pnl > 0 && (
                        <div className="text-[9px] text-green-400">
                          Realized Profit: +${pos.realized_pnl.toFixed(2)}
                        </div>
                      )}

                      {isInLoss && hedgeEnabled && (
                        <div className="mt-1 p-1.5 bg-orange-900/30 border border-orange-700/50 rounded text-[9px]">
                          <div className="flex items-center gap-1 text-orange-400 font-medium">
                            <ArrowLeftRight className="w-3 h-3" />
                            Hedge Available
                          </div>
                          <div className="text-orange-300 mt-0.5">
                            Position is {Math.abs(currentPnlPct).toFixed(2)}% in loss (${Math.abs(pos.unrealized_pnl || 0).toFixed(2)}).
                          </div>
                        </div>
                      )}
                    </div>
                  );
                })
            ) : (
              <div className="p-3 text-center text-xs text-gray-500">
                No active positions
              </div>
            )}
          </div>
        )}

        {/* Help Text */}
        <div className="px-2 py-1.5 bg-purple-900/20 border border-purple-700/30 rounded text-[10px] text-purple-400">
          Monitor TP1/TP2/TP3 progress and rebuy status. Configure settings in Ginie Panel's mode configuration.
        </div>
      </div>
    </CollapsibleCard>
  );
}
