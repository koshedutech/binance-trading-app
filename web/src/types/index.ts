// API Response types
export interface APIResponse<T> {
  success: boolean;
  data?: T;
  error?: boolean;
  message?: string;
}

// Bot Status
export interface BotStatus {
  running: boolean;
  dry_run: boolean;
  testnet: boolean;
  start_time?: string;
  strategies_count: number;
  open_positions: number;
}

// Position
export interface Position {
  symbol: string;
  entry_price: number;
  current_price?: number;
  quantity: number;
  side: string;
  entry_time: string;
  stop_loss?: number;
  take_profit?: number;
  pnl?: number;
  pnl_percent?: number;
}

// Trade (historical)
export interface Trade {
  id: number;
  symbol: string;
  side: string;
  entry_price: number;
  exit_price?: number;
  quantity: number;
  entry_time: string;
  exit_time?: string;
  stop_loss?: number;
  take_profit?: number;
  pnl?: number;
  pnl_percent?: number;
  strategy_name?: string;
  status: string;
  created_at: string;
  updated_at: string;
}

// Order
export interface Order {
  id: number;
  symbol: string;
  order_type: string;
  side: string;
  price?: number;
  quantity: number;
  executed_qty: number;
  status: string;
  time_in_force?: string;
  created_at: string;
  updated_at: string;
  filled_at?: string;
  trade_id?: number;
}

// Strategy
export interface Strategy {
  name: string;
  symbol: string;
  interval: string;
  enabled: boolean;
  last_signal?: string;
  last_evaluation?: string;
}

// Signal
export interface Signal {
  id: number;
  strategy_name: string;
  symbol: string;
  signal_type: string;
  entry_price: number;
  stop_loss?: number;
  take_profit?: number;
  quantity?: number;
  reason?: string;
  timestamp: string;
  executed: boolean;
  created_at: string;
}

// Pending Signal (awaiting confirmation)
export interface PendingSignal {
  id: number;
  strategy_name: string;
  symbol: string;
  signal_type: string;
  entry_price: number;
  current_price: number;
  stop_loss?: number;
  take_profit?: number;
  quantity?: number;
  reason?: string;
  conditions_met: any;
  timestamp: string;
  status: 'PENDING' | 'CONFIRMED' | 'REJECTED' | 'ARCHIVED';
  confirmed_at?: string;
  rejected_at?: string;
  archived?: boolean;
  archived_at?: string;
  created_at: string;
}

// Parsed pattern data from reason field
export interface PatternData {
  patternName: string;          // e.g., "Morning Star"
  confidence: number;            // 0-100
  confluenceScore: number;       // 0-100
  confluenceGrade: string;       // A, B, C, etc.
  fvgPresent: boolean;
  volumeMultiplier?: number;     // e.g., 2.4
  additionalFactors: string[];   // ["FVG zone", "High volume"]
}

// Signal with parsed pattern data
export interface EnhancedPendingSignal extends PendingSignal {
  patternData?: PatternData;
}

// Screener Result
export interface ScreenerResult {
  id: number;
  symbol: string;
  last_price: number;
  price_change_percent?: number;
  volume?: number;
  quote_volume?: number;
  high_24h?: number;
  low_24h?: number;
  signals: string[];
  timestamp: string;
  created_at: string;
}

// Trading Metrics
export interface TradingMetrics {
  total_trades: number;
  winning_trades: number;
  losing_trades: number;
  win_rate: number;
  total_pnl: number;
  average_pnl: number;
  average_win: number;
  average_loss: number;
  largest_win: number;
  largest_loss: number;
  profit_factor: number;
  open_positions: number;
  active_orders: number;
  total_signals: number;
  executed_signals: number;
  last_trade_time?: string;
}

// System Event
export interface SystemEvent {
  id: number;
  event_type: string;
  source?: string;
  message?: string;
  data?: Record<string, any>;
  timestamp: string;
  created_at: string;
}

// WebSocket Event
export interface WSEvent {
  type: string;
  timestamp: string;
  data: Record<string, any>;
}

// Chart Data
export interface CandleData {
  time: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume?: number;
}

// Place Order Request
export interface PlaceOrderRequest {
  symbol: string;
  side: 'BUY' | 'SELL';
  order_type: 'MARKET' | 'LIMIT';
  quantity: number;
  price?: number;
}

// Strategy Scanner - Proximity Detection Types

export interface ConditionDetail {
  name: string;
  description: string;
  met: boolean;
  value?: any;
  target?: any;
  distance?: any;
}

export interface ConditionsChecklist {
  total_conditions: number;
  met_conditions: number;
  failed_conditions: number;
  details: ConditionDetail[];
}

export interface TimePrediction {
  min_minutes: number;
  max_minutes: number;
  confidence: number;
  based_on: string;
}

export interface ProximityResult {
  symbol: string;
  strategy_name: string;
  current_price: number;
  target_price: number;
  distance_percent: number;
  distance_absolute: number;
  readiness_score: number;
  trend_direction: 'BULLISH' | 'BEARISH' | 'NEUTRAL';
  conditions: ConditionsChecklist;
  time_prediction?: TimePrediction;
  last_evaluated: string;
  timestamp: string;
}

export interface ScanResult {
  scan_id: string;
  start_time: string;
  end_time: string;
  duration: number;
  symbols_scanned: number;
  results: ProximityResult[];
}

export interface WatchlistItem {
  id: number;
  symbol: string;
  notes?: string;
  added_at: string;
  created_at: string;
}

// Visual Strategy Builder Types
import type { Node, Edge } from '@xyflow/react';

export interface VisualFlowDefinition {
  version: string;
  nodes: Node[];
  edges: Edge[];
  settings: StrategySettings;
}

// Type aliases for React Flow types
export type FlowNode = Node;
export type FlowEdge = Edge;

export interface NodeData {
  label: string;
  [key: string]: any;
}

export interface StrategySettings {
  symbol: string;
  interval: string;
  stopLoss?: {
    enabled: boolean;
    type: 'percentage' | 'absolute' | 'atr';
    value: number;
  };
  takeProfit?: {
    enabled: boolean;
    type: 'percentage' | 'absolute';
    value: number;
  };
  riskManagement?: {
    maxPositionSize: number;
    maxConcurrentTrades: number;
  };
}

export interface ConditionGroup {
  operator: 'AND' | 'OR';
  conditions: Condition[];
  groups: ConditionGroup[];
}

export interface Condition {
  type: 'indicator_comparison' | 'candle_property' | 'price_target' | 'pattern';
  [key: string]: any;
}

// Backtest Types

export interface BacktestRequest {
  symbol: string;
  interval: string;
  start_date: string;
  end_date: string;
}

export interface BacktestResult {
  id: number;
  strategy_config_id: number;
  symbol: string;
  interval: string;
  start_date: string;
  end_date: string;
  total_trades: number;
  winning_trades: number;
  losing_trades: number;
  win_rate: number;
  total_pnl: number;
  total_fees: number;
  net_pnl: number;
  average_win: number;
  average_loss: number;
  largest_win: number;
  largest_loss: number;
  profit_factor: number;
  max_drawdown: number;
  max_drawdown_percent: number;
  avg_trade_duration_minutes: number;
  created_at: string;
  updated_at: string;
}

export interface BacktestTrade {
  id: number;
  backtest_result_id: number;
  entry_time: string;
  entry_price: number;
  entry_reason: string;
  exit_time: string;
  exit_price: number;
  exit_reason: string;
  quantity: number;
  side: 'BUY' | 'SELL';
  pnl: number;
  pnl_percent: number;
  fees: number;
  duration_minutes: number;
  created_at: string;
}

// Strategy Config (extend existing with visual flow support)
export interface StrategyConfig {
  id: number;
  name: string;
  symbol: string;
  timeframe: string;
  indicator_type: string;
  autopilot: boolean;
  enabled: boolean;
  position_size: number;
  stop_loss_percent: number;
  take_profit_percent: number;
  config_params?: {
    visual_flow?: VisualFlowDefinition;
    [key: string]: any;
  };
  created_at: string;
  updated_at: string;
}
