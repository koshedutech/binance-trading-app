// ==================== FUTURES TYPES ====================

// Position Side
export type PositionSide = 'LONG' | 'SHORT' | 'BOTH';

// Margin Type
export type MarginType = 'CROSSED' | 'ISOLATED';

// Position Mode
export type PositionMode = 'ONE_WAY' | 'HEDGE';

// Order Types
export type FuturesOrderType =
  | 'LIMIT'
  | 'MARKET'
  | 'STOP'
  | 'STOP_MARKET'
  | 'TAKE_PROFIT'
  | 'TAKE_PROFIT_MARKET'
  | 'TRAILING_STOP_MARKET';

// Time In Force
export type TimeInForce = 'GTC' | 'IOC' | 'FOK' | 'GTX';

// Working Type for TP/SL
export type WorkingType = 'CONTRACT_PRICE' | 'MARK_PRICE';

// Order Status
export type FuturesOrderStatus =
  | 'NEW'
  | 'PARTIALLY_FILLED'
  | 'FILLED'
  | 'CANCELED'
  | 'EXPIRED';

// Trade Status
export type FuturesTradeStatus = 'OPEN' | 'CLOSED' | 'LIQUIDATED';

// ==================== ACCOUNT ====================

export interface FuturesAccountInfo {
  fee_tier: number;
  can_trade: boolean;
  can_deposit: boolean;
  can_withdraw: boolean;
  total_initial_margin: number;
  total_maint_margin: number;
  total_wallet_balance: number;
  total_unrealized_profit: number;
  total_margin_balance: number;
  total_position_initial_margin: number;
  total_open_order_initial_margin: number;
  total_cross_wallet_balance: number;
  total_cross_un_pnl: number;
  available_balance: number;
  max_withdraw_amount: number;
  assets: FuturesAsset[];
  positions: FuturesAccountPosition[];
}

export interface FuturesAsset {
  asset: string;
  walletBalance: number;
  unrealizedProfit: number;
  marginBalance: number;
  maintMargin: number;
  initialMargin: number;
  positionInitialMargin: number;
  openOrderInitialMargin: number;
  crossWalletBalance: number;
  crossUnPnl: number;
  availableBalance: number;
  maxWithdrawAmount: number;
  marginAvailable: boolean;
  updateTime: number;
}

export interface FuturesAccountPosition {
  symbol: string;
  initialMargin: number;
  maintMargin: number;
  unrealizedProfit: number;
  positionInitialMargin: number;
  openOrderInitialMargin: number;
  leverage: number;
  isolated: boolean;
  entryPrice: number;
  maxNotional: number;
  positionSide: PositionSide;
  positionAmt: number;
  notional: number;
  isolatedWallet: number;
  updateTime: number;
}

// ==================== POSITIONS ====================

export interface FuturesPosition {
  symbol: string;
  positionAmt: number;
  entryPrice: number;
  markPrice: number;
  unRealizedProfit: number;
  liquidationPrice: number;
  leverage: number;
  maxNotionalValue: number;
  marginType: MarginType;
  isolatedMargin: number;
  isAutoAddMargin: boolean;
  positionSide: PositionSide;
  notional: number;
  isolatedWallet: number;
  updateTime: number;
  // Calculated fields
  roe?: number;
  pnlPercent?: number;
}

// ==================== ORDERS ====================

export interface PlaceFuturesOrderRequest {
  symbol: string;
  side: 'BUY' | 'SELL';
  position_side: PositionSide;
  order_type: FuturesOrderType;
  quantity: number;
  price?: number;
  stop_price?: number;
  time_in_force?: TimeInForce;
  reduce_only?: boolean;
  close_position?: boolean;
  take_profit?: number;
  stop_loss?: number;
  working_type?: WorkingType;
}

export interface FuturesOrder {
  orderId: number;
  symbol: string;
  status: FuturesOrderStatus;
  clientOrderId: string;
  price: number;
  avgPrice: number;
  origQty: number;
  executedQty: number;
  cumQuote: number;
  timeInForce: TimeInForce;
  type: FuturesOrderType;
  reduceOnly: boolean;
  closePosition: boolean;
  side: 'BUY' | 'SELL';
  positionSide: PositionSide;
  stopPrice: number;
  workingType: WorkingType;
  priceProtect: boolean;
  origType: string;
  time: number;
  updateTime: number;
}

export interface FuturesOrderResponse {
  orderId: number;
  symbol: string;
  status: FuturesOrderStatus;
  price: number;
  avgPrice: number;
  origQty: number;
  executedQty: number;
  side: 'BUY' | 'SELL';
  positionSide: PositionSide;
  type: FuturesOrderType;
  updateTime: number;
}

// ==================== MARKET DATA ====================

export interface FundingRate {
  symbol: string;
  fundingRate: number;
  fundingTime: number;
  nextFundingTime?: number;
  markPrice: number;
}

export interface MarkPrice {
  symbol: string;
  markPrice: number;
  indexPrice: number;
  estimatedSettlePrice: number;
  lastFundingRate: number;
  nextFundingTime: number;
  interestRate: number;
  time: number;
}

export interface OrderBookEntry {
  price: number;
  quantity: number;
}

export interface OrderBookDepth {
  lastUpdateId: number;
  bids: [string, string][]; // [price, qty]
  asks: [string, string][]; // [price, qty]
}

// ==================== AI DECISION ====================

export interface AIDecision {
  id: number;
  symbol: string;
  current_price: number;
  action: string;
  confidence: number;
  reasoning: string;
  ml_direction?: string;
  ml_confidence?: number;
  sentiment_direction?: string;
  sentiment_confidence?: number;
  llm_direction?: string;
  llm_confidence?: number;
  pattern_direction?: string;
  pattern_confidence?: number;
  bigcandle_direction?: string;
  bigcandle_confidence?: number;
  confluence_count: number;
  risk_level: string;
  executed: boolean;
  created_at: string;
}

// ==================== HISTORY ====================

export interface FuturesTrade {
  id: number;
  symbol: string;
  positionSide: PositionSide;
  side: string;
  entryPrice: number;
  exitPrice?: number;
  markPrice?: number;
  quantity: number;
  leverage: number;
  marginType: MarginType;
  isolatedMargin?: number;
  realizedPnl?: number;
  unrealizedPnl?: number;
  realizedPnlPercent?: number;
  liquidationPrice?: number;
  stopLoss?: number;
  takeProfit?: number;
  trailingStop?: number;
  status: FuturesTradeStatus;
  entryTime: string;
  exitTime?: string;
  tradeSource: string;
  notes?: string;
  ai_decision_id?: number;
  ai_decision?: AIDecision;
  createdAt: string;
  updatedAt: string;
}

export interface FundingFee {
  id: number;
  symbol: string;
  fundingRate: number;
  fundingFee: number;
  positionAmt: number;
  asset: string;
  timestamp: string;
  createdAt: string;
}

export interface FuturesTransaction {
  id: number;
  transactionId: number;
  symbol: string;
  incomeType: string;
  income: number;
  asset: string;
  info?: string;
  timestamp: string;
  futuresTradeId?: number;
  createdAt: string;
}

// ==================== SETTINGS ====================

export interface FuturesAccountSettings {
  id: number;
  symbol: string;
  leverage: number;
  marginType: MarginType;
  positionMode: PositionMode;
  createdAt: string;
  updatedAt: string;
}

export interface SetLeverageRequest {
  symbol: string;
  leverage: number;
}

export interface SetMarginTypeRequest {
  symbol: string;
  margin_type: MarginType;
}

export interface SetPositionModeRequest {
  dual_side_position: boolean;
}

export interface LeverageResponse {
  leverage: number;
  maxNotionalValue: number;
  symbol: string;
}

export interface PositionModeResponse {
  dualSidePosition: boolean;
}

// ==================== METRICS ====================

export interface FuturesTradingMetrics {
  totalTrades: number;
  winningTrades: number;
  losingTrades: number;
  winRate: number;
  totalRealizedPnl: number;
  totalUnrealizedPnl: number;
  totalFundingFees: number;
  totalCommission: number; // Trading fees (negative)
  averagePnl: number;
  averageWin: number;
  averageLoss: number;
  largestWin: number;
  largestLoss: number;
  profitFactor: number;
  averageLeverage: number;
  openPositions: number;
  openOrders: number;

  // Daily stats (detailed breakdown for Daily Net PNL card)
  dailyRealizedPnl: number;  // Net PnL from trades (today only)
  dailyGrossProfit: number;  // Sum of winning trades
  dailyGrossLoss: number;    // Sum of losing trades (negative)
  dailyCommission: number;   // Trading fees (negative)
  dailyFundingFees: number;  // Funding fees (can be + or -)
  dailyTotalFees: number;    // Total fees as positive number
  dailyTrades: number;
  dailyWins: number;
  dailyLosses: number;
  dailyWinRate: number;

  // Weekly stats (detailed breakdown for Weekly Net PNL card)
  weeklyRealizedPnl: number;  // Net PnL from trades (last 7 days)
  weeklyGrossProfit: number;  // Sum of winning trades
  weeklyGrossLoss: number;    // Sum of losing trades (negative)
  weeklyCommission: number;   // Trading fees (negative)
  weeklyFundingFees: number;  // Funding fees (can be + or -)
  weeklyTotalFees: number;    // Total fees as positive number
  weeklyTrades: number;
  weeklyWins: number;
  weeklyLosses: number;
  weeklyWinRate: number;

  // Time boundaries (for countdown timers and period display)
  dailyResetTime: number;     // Next daily reset (user's timezone midnight) in milliseconds
  weeklyStartDate: string;    // Week start date (YYYY-MM-DD)
  weeklyEndDate: string;      // Week end date (YYYY-MM-DD)
  serverTimeUTC: number;      // Current server time in milliseconds
  timezone: string;           // User's timezone identifier (e.g., "Asia/Kolkata")
  timezoneOffset: string;     // User's timezone offset (e.g., "+05:30")

  lastTradeTime?: string;
}

// Stats grouped by trade source (AI, Strategy, Manual)
export interface TradeSourceStats {
  totalTrades: number;
  winningTrades: number;
  losingTrades: number;
  winRate: number;
  totalPnl: number;
  tpHits: number;
  slHits: number;
  avgPnl: number;
}

// ==================== WEBSOCKET EVENTS ====================

export interface FuturesOrderBookUpdate {
  type: 'FUTURES_ORDERBOOK_UPDATE';
  symbol: string;
  bids: [string, string][];
  asks: [string, string][];
  eventTime: number;
}

export interface FuturesMarkPriceUpdate {
  type: 'FUTURES_MARK_PRICE_UPDATE';
  symbol: string;
  markPrice: string;
  indexPrice: string;
  fundingRate: string;
  nextFundingTime: number;
}

export interface FuturesTradeUpdate {
  type: 'FUTURES_TRADE_UPDATE';
  symbol: string;
  price: string;
  quantity: string;
  tradeTime: number;
  isBuyerMaker: boolean;
}

export interface FuturesPositionUpdate {
  type: 'FUTURES_POSITION_UPDATE';
  positions: FuturesPosition[];
}

export type FuturesWSEvent =
  | FuturesOrderBookUpdate
  | FuturesMarkPriceUpdate
  | FuturesTradeUpdate
  | FuturesPositionUpdate;

// ==================== SYMBOL INFO ====================

export interface FuturesSymbolInfo {
  symbol: string;
  pair: string;
  contractType: string;
  deliveryDate: number;
  onboardDate: number;
  status: string;
  maintMarginPercent: number;
  requiredMarginPercent: number;
  baseAsset: string;
  quoteAsset: string;
  marginAsset: string;
  pricePrecision: number;
  quantityPrecision: number;
  baseAssetPrecision: number;
  quotePrecision: number;
  underlyingType: string;
  underlyingSubType: string[];
  settlePlan: number;
  triggerProtect: number;
  orderTypes: string[];
  timeInForce: string[];
}

// ==================== GAP ANALYSIS UI (Story 11.40) ====================

// Gap direction for blocking reasons
export type GapDirection = 'up' | 'down' | 'in_range';

// Detailed score breakdown with all sub-component scores
export interface ScoreBreakdownDetailed {
  // Technical (0-40)
  technical: number;
  technical_max: number;
  trend_alignment: number;
  trend_alignment_max: number;
  momentum: number;
  momentum_max: number;
  volatility: number;
  volatility_max: number;
  volume: number;
  volume_max: number;

  // Context (0-30)
  context: number;
  context_max: number;
  regime_match: number;
  regime_match_max: number;
  timeframe_align: number;
  timeframe_align_max: number;
  btc_trend: number;
  btc_trend_max: number;

  // LLM (0-20)
  llm: number;
  llm_max: number;

  // History (0-10)
  history: number;
  history_max: number;
  symbol_winrate: number;
  symbol_winrate_max: number;
  strategy_winrate: number;
  strategy_winrate_max: number;

  // Final score and gap
  final: number;
  threshold: number;
  gap_to_threshold: number;
}

// Blocking reason with directional gap information
export interface BlockingReasonWithGap {
  code: string;
  category: 'HARD_BLOCK' | 'SOFT_BLOCK' | 'WARNING';
  description: string;
  current_value: number;
  target_value: number;
  target_range_end?: number;
  gap: number;
  gap_direction: GapDirection;
  gap_display: string;
  overridable: boolean;
  timestamp: number;
}

// Score history for UI display (8 hours of data)
export interface ScoreHistoryForUI {
  timestamps: number[];
  scores: number[];
  trend: 'rising' | 'falling' | 'stable';
  change_8h: number;
}

// Extended coin state with gap analysis data
export interface CoinStateWithGaps {
  symbol: string;
  price: number;
  regime: 'TRENDING' | 'RANGING' | 'VOLATILE' | 'CONSOLIDATING';
  active_strategy: string;
  decision: 'READY' | 'BLOCKED' | 'PENDING';
  adx: number;
  atr: number;
  rsi: number;
  ema_9: number;
  ema_21: number;
  trend_1h: 'BULLISH' | 'BEARISH' | 'NEUTRAL';
  trend_15m: 'BULLISH' | 'BEARISH' | 'NEUTRAL';
  scores: {
    technical: number;
    context: number;
    llm: number;
    history: number;
    final: number;
  };
  blocking: {
    total_reasons: number;
    hard_block_count: number;
    soft_block_count: number;
    warning_count: number;
    is_blocked: boolean;
    can_override: boolean;
    all_reasons: BlockingReasonWithGap[];
  };
  last_updated: number;
  // Gap analysis additions
  score_breakdown: ScoreBreakdownDetailed;
  blocking_with_gaps: BlockingReasonWithGap[];
  score_history: ScoreHistoryForUI;
  overall_gap: number;
  proximity_rank: number;
  can_override: boolean;
  status_label: string;
  status_color: 'green' | 'light-green' | 'yellow' | 'orange' | 'red';
}

// Override entry request
export interface OverrideEntryRequest {
  symbol: string;
  direction: 'LONG' | 'SHORT';
  reason?: string;
}

// Override entry response
export interface OverrideEntryResponse {
  success: boolean;
  message: string;
  symbol: string;
  direction: string;
  reason?: string;
  overridden_blocks: number;
  note: string;
}
