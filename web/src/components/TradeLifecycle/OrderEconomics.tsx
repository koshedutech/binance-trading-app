// Order Economics Component - Detailed cost/fee/breakeven analysis
// Epic 7: Client Order ID & Trade Lifecycle Tracking
// Collapsible by default, shows entry costs, fees, and breakeven calculation

import React, { useState, useMemo } from 'react';
import {
  ChevronDown,
  ChevronRight,
  Calculator,
  TrendingUp,
  TrendingDown,
  DollarSign,
  Percent,
  Target,
  AlertCircle,
} from 'lucide-react';
import type { PositionState } from './types';

interface OrderEconomicsProps {
  positionState: PositionState;
  positionSide: 'LONG' | 'SHORT';
  currentPrice?: number;
  // User fee rates (from settings)
  makerFeeRate?: number; // Default 0.02% = 0.0002
  takerFeeRate?: number; // Default 0.04% = 0.0004
  // TP and SL prices for projection
  tpPrice?: number;
  slPrice?: number;
}

// Format currency with appropriate precision (null-safe)
function formatCurrency(value: number | null | undefined, decimals = 4): string {
  const safeValue = value ?? 0;
  if (Math.abs(safeValue) >= 100) return `$${safeValue.toFixed(2)}`;
  if (Math.abs(safeValue) >= 1) return `$${safeValue.toFixed(4)}`;
  return `$${safeValue.toFixed(decimals)}`;
}

// Format percentage (null-safe)
function formatPercent(value: number | null | undefined): string {
  const safeValue = value ?? 0;
  return `${safeValue >= 0 ? '+' : ''}${(safeValue * 100).toFixed(4)}%`;
}

// Format price based on magnitude (null-safe)
function formatPrice(price: number | null | undefined): string {
  const safePrice = price ?? 0;
  if (safePrice >= 1000) return safePrice.toFixed(2);
  if (safePrice >= 1) return safePrice.toFixed(4);
  return safePrice.toFixed(8);
}

export default function OrderEconomics({
  positionState,
  positionSide,
  currentPrice,
  makerFeeRate = 0.0002, // 0.02%
  takerFeeRate = 0.0004, // 0.04%
  tpPrice,
  slPrice,
}: OrderEconomicsProps) {
  const [expanded, setExpanded] = useState(false);

  // Calculate all economics
  const economics = useMemo(() => {
    const entryPrice = positionState.entryPrice;
    const quantity = positionState.entryQuantity;
    const entryValue = positionState.entryValue || entryPrice * quantity;
    const entryFees = positionState.entryFees || 0;

    // Determine if entry was maker or taker based on fees
    // If fee rate is closer to maker rate, it's a maker order
    const entryFeeRate = entryFees / entryValue;
    const isMaker = entryFeeRate <= (makerFeeRate + takerFeeRate) / 2;
    const entryType = isMaker ? 'MAKER' : 'TAKER';

    // Estimate exit fee (assume taker for worst case)
    const exitFeeEstimate = entryValue * takerFeeRate;
    const totalFees = entryFees + exitFeeEstimate;

    // Calculate breakeven price
    // For LONG: need price to go UP to cover fees
    // For SHORT: need price to go DOWN to cover fees
    const feePerUnit = totalFees / quantity;
    const breakevenPrice = positionSide === 'LONG'
      ? entryPrice + feePerUnit
      : entryPrice - feePerUnit;

    // Distance to breakeven
    const breakevenDistance = breakevenPrice - entryPrice;
    const breakevenPercent = breakevenDistance / entryPrice;

    // Current P&L if we have current price
    let currentPnL = 0;
    let currentPnLPercent = 0;
    let distanceToBreakeven = 0;
    let distanceToBreakevenPercent = 0;

    if (currentPrice && currentPrice > 0) {
      const priceChange = positionSide === 'LONG'
        ? currentPrice - entryPrice
        : entryPrice - currentPrice;
      currentPnL = (priceChange * quantity) - totalFees;
      currentPnLPercent = currentPnL / entryValue;

      distanceToBreakeven = positionSide === 'LONG'
        ? breakevenPrice - currentPrice
        : currentPrice - breakevenPrice;
      distanceToBreakevenPercent = distanceToBreakeven / currentPrice;
    }

    // TP/SL projections
    let tpNetPnL = 0;
    let tpGrossPnL = 0;
    let slNetPnL = 0;
    let slGrossPnL = 0;

    if (tpPrice && tpPrice > 0) {
      const tpPriceChange = positionSide === 'LONG'
        ? tpPrice - entryPrice
        : entryPrice - tpPrice;
      tpGrossPnL = tpPriceChange * quantity;
      tpNetPnL = tpGrossPnL - totalFees;
    }

    if (slPrice && slPrice > 0) {
      const slPriceChange = positionSide === 'LONG'
        ? slPrice - entryPrice
        : entryPrice - slPrice;
      slGrossPnL = slPriceChange * quantity;
      slNetPnL = slGrossPnL - totalFees;
    }

    return {
      entryPrice,
      quantity,
      entryValue,
      entryFees,
      entryType,
      isMaker,
      exitFeeEstimate,
      totalFees,
      breakevenPrice,
      breakevenDistance,
      breakevenPercent,
      currentPrice,
      currentPnL,
      currentPnLPercent,
      distanceToBreakeven,
      distanceToBreakevenPercent,
      tpGrossPnL,
      tpNetPnL,
      slGrossPnL,
      slNetPnL,
      makerFeeRate,
      takerFeeRate,
    };
  }, [positionState, positionSide, currentPrice, makerFeeRate, takerFeeRate, tpPrice, slPrice]);

  return (
    <div className="border border-gray-700 rounded-lg overflow-hidden">
      {/* Collapsible Header */}
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between px-4 py-3 bg-gray-800/50 hover:bg-gray-800/70 transition-colors"
      >
        <div className="flex items-center gap-2">
          {expanded ? (
            <ChevronDown className="w-4 h-4 text-cyan-400" />
          ) : (
            <ChevronRight className="w-4 h-4 text-gray-500" />
          )}
          <Calculator className="w-4 h-4 text-cyan-400" />
          <span className="text-sm font-medium text-cyan-400">Order Economics</span>
        </div>

        {/* Summary when collapsed */}
        {!expanded && (
          <div className="flex items-center gap-4 text-xs">
            <span className="text-gray-400">
              Entry: {formatCurrency(economics.entryValue)}
            </span>
            <span className="text-gray-400">
              Fees: {formatCurrency(economics.totalFees)}
            </span>
            <span className="text-amber-400">
              BE: ${formatPrice(economics.breakevenPrice)}
            </span>
          </div>
        )}
      </button>

      {/* Expanded Content */}
      {expanded && (
        <div className="p-4 bg-gray-900/30 space-y-4">
          {/* Entry Details & Fees - Two Column Layout */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {/* Entry Details */}
            <div className="space-y-2">
              <h4 className="text-xs font-semibold text-gray-400 uppercase tracking-wider flex items-center gap-1">
                <DollarSign className="w-3 h-3" />
                Entry Details
              </h4>
              <div className="bg-gray-800/50 rounded-lg p-3 space-y-2 text-sm">
                <div className="flex justify-between">
                  <span className="text-gray-400">Entry Price:</span>
                  <span className="text-gray-200 font-mono">${formatPrice(economics.entryPrice)}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-400">Quantity:</span>
                  <span className="text-gray-200 font-mono">{(economics.quantity ?? 0).toFixed(4)}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-400">Entry Value:</span>
                  <span className="text-gray-200 font-mono">{formatCurrency(economics.entryValue)}</span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-gray-400">Entry Type:</span>
                  <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                    economics.isMaker
                      ? 'bg-green-500/20 text-green-400'
                      : 'bg-red-500/20 text-red-400'
                  }`}>
                    {economics.entryType}
                  </span>
                </div>
              </div>
            </div>

            {/* Fees & Costs */}
            <div className="space-y-2">
              <h4 className="text-xs font-semibold text-gray-400 uppercase tracking-wider flex items-center gap-1">
                <Percent className="w-3 h-3" />
                Fees & Costs
              </h4>
              <div className="bg-gray-800/50 rounded-lg p-3 space-y-2 text-sm">
                <div className="flex justify-between">
                  <span className="text-gray-400">Entry Fee:</span>
                  <span className="text-red-400 font-mono">-{formatCurrency(economics.entryFees)}</span>
                </div>
                <div className="flex justify-between text-xs text-gray-500">
                  <span className="pl-2">Rate:</span>
                  <span>{(economics.entryValue ? (economics.entryFees / economics.entryValue * 100) : 0).toFixed(4)}%</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-400">Exit Fee (est):</span>
                  <span className="text-orange-400 font-mono">~{formatCurrency(economics.exitFeeEstimate)}</span>
                </div>
                <div className="flex justify-between text-xs text-gray-500">
                  <span className="pl-2">Assuming TAKER:</span>
                  <span>{((economics.takerFeeRate ?? 0) * 100).toFixed(2)}%</span>
                </div>
                <div className="border-t border-gray-700 pt-2 flex justify-between font-medium">
                  <span className="text-gray-300">Total Fees:</span>
                  <span className="text-red-400 font-mono">-{formatCurrency(economics.totalFees)}</span>
                </div>
              </div>
            </div>
          </div>

          {/* Breakeven Analysis */}
          <div className="space-y-2">
            <h4 className="text-xs font-semibold text-gray-400 uppercase tracking-wider flex items-center gap-1">
              <Target className="w-3 h-3" />
              Breakeven Analysis
            </h4>
            <div className="bg-amber-500/10 border border-amber-500/30 rounded-lg p-3">
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
                <div>
                  <span className="text-gray-400 text-xs">Breakeven Price</span>
                  <div className="text-amber-400 font-mono text-lg font-semibold">
                    ${formatPrice(economics.breakevenPrice)}
                  </div>
                  <span className="text-xs text-gray-500">
                    {positionSide === 'LONG' ? '↑' : '↓'} {formatCurrency(Math.abs(economics.breakevenDistance))} from entry
                  </span>
                </div>
                <div>
                  <span className="text-gray-400 text-xs">Distance to BE</span>
                  <div className="text-amber-400 font-mono text-lg">
                    {formatPercent(economics.breakevenPercent)}
                  </div>
                  <span className="text-xs text-gray-500">
                    Must {positionSide === 'LONG' ? 'rise' : 'fall'} to breakeven
                  </span>
                </div>
                <div>
                  <span className="text-gray-400 text-xs">To Recover</span>
                  <div className="text-red-400 font-mono text-lg">
                    {formatCurrency(economics.totalFees)}
                  </div>
                  <span className="text-xs text-gray-500">
                    Total fees to cover
                  </span>
                </div>
              </div>

              {/* Current Price vs Breakeven */}
              {economics.currentPrice && economics.currentPrice > 0 && (
                <div className="mt-3 pt-3 border-t border-amber-500/20">
                  <div className="flex items-center justify-between text-sm">
                    <span className="text-gray-400">Current Price:</span>
                    <span className="text-gray-200 font-mono">${formatPrice(economics.currentPrice)}</span>
                  </div>
                  <div className="flex items-center justify-between text-sm mt-1">
                    <span className="text-gray-400">
                      {economics.distanceToBreakeven > 0 ? 'Still need:' : 'Above BE by:'}
                    </span>
                    <span className={`font-mono ${
                      economics.distanceToBreakeven > 0 ? 'text-red-400' : 'text-green-400'
                    }`}>
                      {economics.distanceToBreakeven > 0 ? '' : '+'}{formatPercent(-economics.distanceToBreakevenPercent)}
                    </span>
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* P&L Projection */}
          {(tpPrice || slPrice) && (
            <div className="space-y-2">
              <h4 className="text-xs font-semibold text-gray-400 uppercase tracking-wider flex items-center gap-1">
                <AlertCircle className="w-3 h-3" />
                P&L Projection
              </h4>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                {/* TP Projection */}
                {tpPrice && tpPrice > 0 && (
                  <div className="bg-green-500/10 border border-green-500/30 rounded-lg p-3">
                    <div className="flex items-center gap-2 mb-2">
                      <TrendingUp className="w-4 h-4 text-green-400" />
                      <span className="text-sm font-medium text-green-400">If TP Hits</span>
                      <span className="text-xs text-gray-500">(${formatPrice(tpPrice)})</span>
                    </div>
                    <div className="space-y-1 text-sm">
                      <div className="flex justify-between">
                        <span className="text-gray-400">Gross P&L:</span>
                        <span className="text-gray-300 font-mono">+{formatCurrency(economics.tpGrossPnL)}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-gray-400">Total Fees:</span>
                        <span className="text-red-400 font-mono">-{formatCurrency(economics.totalFees)}</span>
                      </div>
                      <div className="border-t border-green-500/30 pt-1 flex justify-between font-medium">
                        <span className="text-gray-300">Net P&L:</span>
                        <span className={`font-mono ${economics.tpNetPnL >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {economics.tpNetPnL >= 0 ? '+' : ''}{formatCurrency(economics.tpNetPnL)}
                        </span>
                      </div>
                    </div>
                  </div>
                )}

                {/* SL Projection */}
                {slPrice && slPrice > 0 && (
                  <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-3">
                    <div className="flex items-center gap-2 mb-2">
                      <TrendingDown className="w-4 h-4 text-red-400" />
                      <span className="text-sm font-medium text-red-400">If SL Hits</span>
                      <span className="text-xs text-gray-500">(${formatPrice(slPrice)})</span>
                    </div>
                    <div className="space-y-1 text-sm">
                      <div className="flex justify-between">
                        <span className="text-gray-400">Gross P&L:</span>
                        <span className="text-gray-300 font-mono">{formatCurrency(economics.slGrossPnL)}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-gray-400">Total Fees:</span>
                        <span className="text-red-400 font-mono">-{formatCurrency(economics.totalFees)}</span>
                      </div>
                      <div className="border-t border-red-500/30 pt-1 flex justify-between font-medium">
                        <span className="text-gray-300">Net P&L:</span>
                        <span className="text-red-400 font-mono">{formatCurrency(economics.slNetPnL)}</span>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            </div>
          )}

          {/* Already Spent Summary */}
          <div className="bg-gray-800/30 rounded-lg p-3 flex items-center justify-between text-sm">
            <div className="flex items-center gap-2">
              <DollarSign className="w-4 h-4 text-red-400" />
              <span className="text-gray-400">Already Spent (Entry Fee):</span>
            </div>
            <span className="text-red-400 font-mono font-medium">-{formatCurrency(economics.entryFees)}</span>
          </div>
        </div>
      )}
    </div>
  );
}
