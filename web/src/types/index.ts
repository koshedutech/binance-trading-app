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
