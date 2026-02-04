// Position Management Types
// Epic 10: Position Stage Management - Story 10.1: Frontend Implementation

// ==================== Position Stage Types ====================

/**
 * Position stage representing the lifecycle of a position.
 * Based on the backend PositionStage constants.
 */
export type PositionStage =
  | 'RISK_ZONE'
  | 'BREAKEVEN'
  | 'TP1'
  | 'EFFICIENCY';

/**
 * Display configuration for position stages.
 */
export interface StageDisplayConfig {
  name: string;
  color: string;
  bgColor: string;
  borderColor: string;
  icon: string;
  description: string;
}

/**
 * Stage display configurations.
 */
export const STAGE_DISPLAY: Record<PositionStage, StageDisplayConfig> = {
  RISK_ZONE: {
    name: 'Risk Zone',
    color: 'text-red-400',
    bgColor: 'bg-red-500/20',
    borderColor: 'border-red-500/30',
    icon: 'AlertTriangle',
    description: 'Position in initial risk phase, SL active',
  },
  BREAKEVEN: {
    name: 'Breakeven',
    color: 'text-yellow-400',
    bgColor: 'bg-yellow-500/20',
    borderColor: 'border-yellow-500/30',
    icon: 'Shield',
    description: 'Position secured at breakeven point',
  },
  TP1: {
    name: 'TP1 Secured',
    color: 'text-green-400',
    bgColor: 'bg-green-500/20',
    borderColor: 'border-green-500/30',
    icon: 'Target',
    description: 'First take profit target reached',
  },
  EFFICIENCY: {
    name: 'Efficiency',
    color: 'text-purple-400',
    bgColor: 'bg-purple-500/20',
    borderColor: 'border-purple-500/30',
    icon: 'TrendingUp',
    description: 'Maximizing gains with efficiency exit',
  },
};

// ==================== Decision Mode Types ====================

/**
 * Decision mode for position exit decisions.
 */
export type DecisionMode = 'classic' | 'new_engine';

/**
 * Classic decision mode settings.
 */
export interface ClassicDecisionSettings {
  adx_reversal_threshold: number;  // 15-30, default 20
  reversal_confirmations: number;  // 1-4, default 2
  rsi_oversold: number;            // 20-40, default 30
  rsi_overbought: number;          // 60-80, default 70
}

/**
 * New engine decision mode settings.
 */
export interface NewEngineDecisionSettings {
  use_active_strategy: boolean;      // Use regime-appropriate strategy
  exit_on_regime_change: boolean;    // Exit when regime changes
  use_strategy_exit_rules: boolean;  // Use strategy-specific exit rules
}

/**
 * Combined decision settings.
 */
export interface PositionDecisionSettings {
  mode: DecisionMode;
  classic: ClassicDecisionSettings;
  new_engine: NewEngineDecisionSettings;
}

// ==================== Efficiency Exit Types ====================

/**
 * Efficiency exit settings for maximizing gains.
 */
export interface EfficiencyExitSettings {
  enabled: boolean;
  historical_window_hours: number;    // Hours of history to analyze
  min_hold_time_minutes: number;      // Minimum hold time before efficiency exit
  consecutive_signals_required: number; // Number of consecutive signals needed
  current_threshold_percent: number;  // Calculated threshold (read-only display)
}

// ==================== Dynamic SL/TP Types ====================

/**
 * Dynamic stop loss and take profit settings.
 */
export interface DynamicSLTPSettings {
  dynamic_sl_enabled: boolean;
  dynamic_tp_enabled: boolean;
  update_on_binance: boolean;      // Sync to Binance orders
  atr_multiplier_sl: number;       // ATR multiplier for SL (1.0-5.0)
  atr_multiplier_tp: number;       // ATR multiplier for TP (1.5-10.0)
}

// ==================== Complete Position Management Config ====================

/**
 * Complete position management configuration.
 */
export interface PositionManagementConfig {
  decision: PositionDecisionSettings;
  efficiency: EfficiencyExitSettings;
  dynamic_sltp: DynamicSLTPSettings;
}

// ==================== Risk-Reward Tracking Types ====================

/**
 * Risk-Reward milestone level.
 */
export interface RRMilestone {
  level: number;        // 1, 2, 3, or 4
  label: string;        // "1:1", "1:2", "1:3", "1:4"
  price: number;        // Calculated price at this R:R level
  achieved: boolean;    // Whether this level has been reached
  achievedAt?: number;  // Timestamp when achieved
}

/**
 * Trailing stop status for position.
 */
export interface TrailingStopStatus {
  // Current state
  current_rr: number;           // Current risk-reward ratio (e.g., 2.5)
  current_sl: number;           // Current stop-loss price
  initial_sl: number;           // Initial stop-loss price
  risk_amount: number;          // Entry - Initial SL ($ amount at risk)
  highest_price: number;        // Highest price since entry (for LONG)

  // Milestone flags
  moved_to_breakeven: boolean;  // At 1:2 R:R, SL moved to entry
  moved_to_1r: boolean;         // At 1:3 R:R, SL moved to 1:1 level

  // R:R milestone levels
  milestones: RRMilestone[];

  // Binance order status
  sl_order_id?: number;         // Binance stop-loss order ID (0 if not placed)
  sl_order_synced: boolean;     // Whether SL is synced on Binance

  // Next action
  next_move_at?: number;        // Price where next SL move happens
  next_move_label?: string;     // Description of next move
}

// ==================== Expanded Position Card Types ====================

/**
 * Price level for visualization.
 */
export interface PriceLevel {
  type: 'entry' | 'current' | 'tp' | 'sl' | 'be' | 'tp1' | 'tp2' | 'tp3';
  price: number;
  label: string;
  color: string;
}

/**
 * Efficiency metrics for display.
 */
export interface EfficiencyMetrics {
  peak_profit: number;
  current_profit: number;
  efficiency_percent: number;
  threshold_percent: number;
  peak_timestamp: number;
}

/**
 * Indicator scores for Classic mode.
 */
export interface ClassicIndicatorScores {
  adx: number;
  adx_threshold: number;
  rsi: number;
  rsi_state: 'oversold' | 'normal' | 'overbought';
  reversal_signals: number;
  reversal_required: number;
}

/**
 * Indicator scores for New Engine mode.
 */
export interface NewEngineIndicatorScores {
  technical: number;
  context: number;
  llm: number;
  history: number;
  final: number;
  regime: string;
  strategy: string;
}

/**
 * Complete position data for expanded card.
 */
export interface ExpandedPositionData {
  // Basic position info
  symbol: string;
  side: 'LONG' | 'SHORT';
  entry_price: number;
  current_price: number;
  quantity: number;
  leverage: number;

  // P&L
  unrealized_pnl: number;
  unrealized_pnl_percent: number;
  roe: number;

  // Stage info
  stage: PositionStage;
  stage_entry_time: number;
  stage_progress_percent: number;

  // Price levels
  stop_loss?: number;
  take_profit?: number;
  breakeven_price?: number;
  tp1_price?: number;
  tp2_price?: number;
  tp3_price?: number;

  // Decision info
  decision_mode: DecisionMode;

  // Efficiency metrics (when in EFFICIENCY stage)
  efficiency?: EfficiencyMetrics;

  // Classic indicators
  classic_scores?: ClassicIndicatorScores;

  // New engine indicators
  new_engine_scores?: NewEngineIndicatorScores;

  // R:R tracking and trailing stop status
  trailing_stop_status?: TrailingStopStatus;

  // Timestamps
  entry_time: number;
  last_updated: number;
}

// ==================== API Request/Response Types ====================

/**
 * Get position management config response.
 */
export interface GetPositionManagementConfigResponse {
  success: boolean;
  config: PositionManagementConfig;
}

/**
 * Update position management config request.
 */
export interface UpdatePositionManagementConfigRequest {
  decision?: Partial<PositionDecisionSettings>;
  efficiency?: Partial<EfficiencyExitSettings>;
  dynamic_sltp?: Partial<DynamicSLTPSettings>;
}

/**
 * Update position management config response.
 */
export interface UpdatePositionManagementConfigResponse {
  success: boolean;
  message: string;
  config: PositionManagementConfig;
}

/**
 * Get expanded position data response.
 */
export interface GetExpandedPositionDataResponse {
  success: boolean;
  position: ExpandedPositionData;
}

// ==================== Default Values ====================

/**
 * Default classic decision settings.
 */
export const DEFAULT_CLASSIC_SETTINGS: ClassicDecisionSettings = {
  adx_reversal_threshold: 20,
  reversal_confirmations: 2,
  rsi_oversold: 30,
  rsi_overbought: 70,
};

/**
 * Default new engine decision settings.
 */
export const DEFAULT_NEW_ENGINE_SETTINGS: NewEngineDecisionSettings = {
  use_active_strategy: true,
  exit_on_regime_change: true,
  use_strategy_exit_rules: true,
};

/**
 * Default efficiency exit settings.
 */
export const DEFAULT_EFFICIENCY_SETTINGS: EfficiencyExitSettings = {
  enabled: true,
  historical_window_hours: 24,
  min_hold_time_minutes: 15,
  consecutive_signals_required: 2,
  current_threshold_percent: 0, // Calculated by backend
};

/**
 * Default dynamic SL/TP settings.
 */
export const DEFAULT_DYNAMIC_SLTP_SETTINGS: DynamicSLTPSettings = {
  dynamic_sl_enabled: true,
  dynamic_tp_enabled: true,
  update_on_binance: true,
  atr_multiplier_sl: 2.0,
  atr_multiplier_tp: 3.0,
};

/**
 * Default position management config.
 */
export const DEFAULT_POSITION_MANAGEMENT_CONFIG: PositionManagementConfig = {
  decision: {
    mode: 'new_engine',
    classic: DEFAULT_CLASSIC_SETTINGS,
    new_engine: DEFAULT_NEW_ENGINE_SETTINGS,
  },
  efficiency: DEFAULT_EFFICIENCY_SETTINGS,
  dynamic_sltp: DEFAULT_DYNAMIC_SLTP_SETTINGS,
};

// ==================== Utility Functions ====================

/**
 * Get stage index for timeline display (0-3).
 */
export function getStageIndex(stage: PositionStage): number {
  const stages: PositionStage[] = ['RISK_ZONE', 'BREAKEVEN', 'TP1', 'EFFICIENCY'];
  return stages.indexOf(stage);
}

/**
 * Get next stage in the progression.
 */
export function getNextStage(stage: PositionStage): PositionStage | null {
  const progression: Record<PositionStage, PositionStage | null> = {
    RISK_ZONE: 'BREAKEVEN',
    BREAKEVEN: 'TP1',
    TP1: 'EFFICIENCY',
    EFFICIENCY: null,
  };
  return progression[stage];
}

/**
 * Calculate price levels for visualization.
 */
export function calculatePriceLevels(position: ExpandedPositionData): PriceLevel[] {
  const levels: PriceLevel[] = [];

  // Entry price
  levels.push({
    type: 'entry',
    price: position.entry_price,
    label: 'Entry',
    color: 'text-blue-400',
  });

  // Current price
  levels.push({
    type: 'current',
    price: position.current_price,
    label: 'Current',
    color: position.unrealized_pnl >= 0 ? 'text-green-400' : 'text-red-400',
  });

  // Stop loss
  if (position.stop_loss) {
    levels.push({
      type: 'sl',
      price: position.stop_loss,
      label: 'SL',
      color: 'text-red-400',
    });
  }

  // Breakeven
  if (position.breakeven_price) {
    levels.push({
      type: 'be',
      price: position.breakeven_price,
      label: 'BE',
      color: 'text-yellow-400',
    });
  }

  // Take profit levels
  if (position.tp1_price) {
    levels.push({
      type: 'tp1',
      price: position.tp1_price,
      label: 'TP1',
      color: 'text-green-400',
    });
  }

  if (position.tp2_price) {
    levels.push({
      type: 'tp2',
      price: position.tp2_price,
      label: 'TP2',
      color: 'text-green-500',
    });
  }

  if (position.tp3_price) {
    levels.push({
      type: 'tp3',
      price: position.tp3_price,
      label: 'TP3',
      color: 'text-green-600',
    });
  }

  // Main take profit
  if (position.take_profit) {
    levels.push({
      type: 'tp',
      price: position.take_profit,
      label: 'TP',
      color: 'text-green-400',
    });
  }

  // Sort by price
  return levels.sort((a, b) => a.price - b.price);
}

/**
 * Format price with 8 decimal precision (Binance standard).
 */
export function formatPositionPrice(price: number): string {
  return price.toFixed(8);
}

/**
 * Format percentage value.
 */
export function formatPositionPercent(value: number): string {
  const sign = value >= 0 ? '+' : '';
  return `${sign}${value.toFixed(2)}%`;
}

/**
 * Calculate R:R milestone prices for a position.
 *
 * For LONG: Higher prices = profit
 * For SHORT: Lower prices = profit
 */
export function calculateRRMilestones(
  entryPrice: number,
  stopLoss: number,
  side: 'LONG' | 'SHORT',
  currentPrice: number,
  movedToBreakeven?: boolean,
  movedTo1R?: boolean
): RRMilestone[] {
  // Calculate risk amount (always positive)
  const riskAmount = side === 'LONG'
    ? entryPrice - stopLoss
    : stopLoss - entryPrice;

  // Calculate price at each R:R level
  const milestones: RRMilestone[] = [];
  const rrLevels = [1, 2, 3, 4];

  for (const level of rrLevels) {
    // Price at this R:R level
    const price = side === 'LONG'
      ? entryPrice + (riskAmount * level)
      : entryPrice - (riskAmount * level);

    // Check if achieved (based on current price)
    const achieved = side === 'LONG'
      ? currentPrice >= price
      : currentPrice <= price;

    milestones.push({
      level,
      label: `1:${level}`,
      price,
      achieved: achieved || (level === 2 && movedToBreakeven) || (level === 3 && movedTo1R),
    });
  }

  return milestones;
}

/**
 * Get the current R:R ratio for a position.
 */
export function calculateCurrentRR(
  entryPrice: number,
  stopLoss: number,
  currentPrice: number,
  side: 'LONG' | 'SHORT'
): number {
  const riskAmount = side === 'LONG'
    ? entryPrice - stopLoss
    : stopLoss - entryPrice;

  if (riskAmount <= 0) return 0;

  const profit = side === 'LONG'
    ? currentPrice - entryPrice
    : entryPrice - currentPrice;

  return profit / riskAmount;
}
