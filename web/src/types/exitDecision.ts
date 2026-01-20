/**
 * Exit Decision Types
 * Story 10.3: Exit Decision Monitoring UI
 *
 * These types match the backend Go structs in exit_decision_state.go
 */

// Decision mode for exit evaluation
export type DecisionMode = 'classic' | 'new_engine';

// Status of an individual exit check
export type ExitCheckStatus = 'PASSED' | 'FAILED' | 'PENDING' | 'DISABLED' | 'ACTIVE';

// Overall exit decision
export type OverallDecision = 'HOLD' | 'EXIT';

// Minimum hold time safeguard
export interface MinHoldTimeSafeguard {
  required_seconds: number;
  elapsed_seconds: number;
  met: boolean;
}

// Breakeven achievement safeguard
export interface BreakEvenSafeguard {
  achieved: boolean;
  achieved_at?: number; // Unix timestamp in milliseconds
  be_price?: number;
}

// Consecutive signals safeguard
export interface ConsecutiveSignalsSafeguard {
  required: number;
  current: number;
}

// All hold safeguards
export interface HoldSafeguards {
  min_hold_time: MinHoldTimeSafeguard;
  breakeven: BreakEvenSafeguard;
  consecutive_signals: ConsecutiveSignalsSafeguard;
}

// ADX indicator details
export interface ADXDetails {
  value: number;
  threshold: number;
  status: 'strong_trend' | 'weak_trend' | 'ranging';
  direction: 'increasing' | 'decreasing' | 'flat';
}

// RSI indicator details
export interface RSIDetails {
  value: number;
  state: 'oversold' | 'normal' | 'overbought';
  overbought: number;
  oversold: number;
}

// EMA cross detection details
export interface EMACrossDetails {
  detected: boolean;
  type?: 'bullish' | 'bearish';
}

// Reversal signals count details
export interface ReversalSignalsDetails {
  count: number;
  required: number;
}

// MACD histogram details
export interface MACDDetails {
  histogram: number;
  direction: 'bullish' | 'bearish' | 'neutral';
}

// All trend reversal details for Classic mode
export interface TrendReversalDetails {
  adx: ADXDetails;
  rsi: RSIDetails;
  ema_cross: EMACrossDetails;
  reversal_signals: ReversalSignalsDetails;
  macd?: MACDDetails;
}

// Efficiency-based exit check details
export interface EfficiencyExitDetails {
  peak_profit_percent: number;
  current_profit_percent: number;
  efficiency_percent: number;
  threshold_percent: number;
}

// Trailing stop loss details
export interface TrailingSLDetails {
  active: boolean;
  sl_price: number;
  current_price: number;
  distance_percent?: number;
}

// Dynamic take profit details
export interface DynamicTPDetails {
  active: boolean;
  tp_price?: number;
  current_price: number;
  progress_percent?: number;
}

// New Engine mode specific details
export interface NewEngineDetails {
  strategy: string;
  entry_regime: string;
  current_regime: string;
  regime_changed: boolean;
  technical_score: number;
  exit_signal: number; // 0-100 exit signal strength
  strategy_rules?: string[];
}

// Individual exit check
export interface ExitCheck {
  name: string; // "TREND_REVERSAL", "EFFICIENCY_EXIT", "TRAILING_SL", "DYNAMIC_TP"
  priority: number; // 1-4 (P1 highest)
  status: ExitCheckStatus;
  would_exit: boolean;

  // Details - only one will be populated based on check type
  trend_reversal_details?: TrendReversalDetails;
  efficiency_details?: EfficiencyExitDetails;
  trailing_sl_details?: TrailingSLDetails;
  dynamic_tp_details?: DynamicTPDetails;
  new_engine_details?: NewEngineDetails;
}

// Complete exit decision state for a position
export interface ExitDecisionState {
  symbol: string;
  decision_mode: DecisionMode;
  overall_decision: OverallDecision;
  decision_reason: string;
  last_check_time: number; // Unix timestamp in milliseconds

  hold_safeguards: HoldSafeguards;
  exit_checks: ExitCheck[];
}

// API response wrapper
export interface ExitDecisionResponse {
  success: boolean;
  exit_decision?: ExitDecisionState;
  error?: string;
}

// Helper functions for display

/**
 * Format seconds into human-readable duration
 */
export function formatDuration(seconds: number): string {
  if (seconds < 60) {
    return `${Math.round(seconds)}s`;
  }
  if (seconds < 3600) {
    const mins = Math.floor(seconds / 60);
    const secs = Math.round(seconds % 60);
    return secs > 0 ? `${mins}m ${secs}s` : `${mins}m`;
  }
  const hours = Math.floor(seconds / 3600);
  const mins = Math.floor((seconds % 3600) / 60);
  return mins > 0 ? `${hours}h ${mins}m` : `${hours}h`;
}

/**
 * Get color class for exit check status
 */
export function getStatusColor(status: ExitCheckStatus): string {
  switch (status) {
    case 'PASSED':
      return 'text-green-500';
    case 'FAILED':
      return 'text-red-500';
    case 'ACTIVE':
      return 'text-blue-500';
    case 'PENDING':
      return 'text-yellow-500';
    case 'DISABLED':
      return 'text-gray-500';
    default:
      return 'text-gray-400';
  }
}

/**
 * Get background color class for exit check status
 */
export function getStatusBgColor(status: ExitCheckStatus): string {
  switch (status) {
    case 'PASSED':
      return 'bg-green-500/10';
    case 'FAILED':
      return 'bg-red-500/10';
    case 'ACTIVE':
      return 'bg-blue-500/10';
    case 'PENDING':
      return 'bg-yellow-500/10';
    case 'DISABLED':
      return 'bg-gray-500/10';
    default:
      return 'bg-gray-500/10';
  }
}

/**
 * Get human-readable name for exit check
 */
export function getExitCheckDisplayName(name: string): string {
  switch (name) {
    case 'TREND_REVERSAL':
      return 'Trend Reversal';
    case 'EFFICIENCY_EXIT':
      return 'Efficiency Exit';
    case 'TRAILING_SL':
      return 'Trailing Stop';
    case 'DYNAMIC_TP':
      return 'Dynamic TP';
    default:
      return name;
  }
}

/**
 * Get priority badge text
 */
export function getPriorityBadge(priority: number): string {
  return `P${priority}`;
}
