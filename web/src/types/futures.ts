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
  feeTier: number;
  canTrade: boolean;
  canDeposit: boolean;
  canWithdraw: boolean;
  totalInitialMargin: number;
  totalMaintMargin: number;
  totalWalletBalance: number;
  totalUnrealizedProfit: number;
  totalMarginBalance: number;
  totalPositionInitialMargin: number;
  totalOpenOrderInitialMargin: number;
  totalCrossWalletBalance: number;
  totalCrossUnPnl: number;
  availableBalance: number;
  maxWithdrawAmount: number;
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
  averagePnl: number;
  averageWin: number;
  averageLoss: number;
  largestWin: number;
  largestLoss: number;
  profitFactor: number;
  averageLeverage: number;
  openPositions: number;
  openOrders: number;
  // Daily stats
  dailyRealizedPnl: number;
  dailyTrades: number;
  dailyWins: number;
  dailyLosses: number;
  dailyWinRate: number;
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
