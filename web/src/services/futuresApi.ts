import axios, { AxiosInstance } from 'axios';
import type {
  FuturesAccountInfo,
  FuturesPosition,
  FuturesOrder,
  FuturesOrderResponse,
  PlaceFuturesOrderRequest,
  SetLeverageRequest,
  SetMarginTypeRequest,
  SetPositionModeRequest,
  LeverageResponse,
  PositionModeResponse,
  FundingRate,
  MarkPrice,
  OrderBookDepth,
  FuturesTrade,
  FundingFee,
  FuturesTransaction,
  FuturesAccountSettings,
  FuturesTradingMetrics,
  TradeSourceStats,
} from '../types/futures';

class FuturesAPIService {
  private client: AxiosInstance;

  constructor() {
    this.client = axios.create({
      baseURL: '/api/futures',
      timeout: 15000,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Response interceptor for error handling
    this.client.interceptors.response.use(
      (response) => response,
      (error) => {
        console.error('Futures API Error:', error);
        return Promise.reject(error);
      }
    );
  }

  // ==================== ACCOUNT ====================

  async getAccountInfo(): Promise<FuturesAccountInfo> {
    const { data } = await this.client.get<FuturesAccountInfo>('/account');
    return data;
  }

  async getWalletBalance(): Promise<{
    total_balance: number;
    available_balance: number;
    total_margin_balance: number;
    total_unrealized_pnl: number;
    currency: string;
    is_simulated: boolean;
    assets: Array<{
      asset: string;
      wallet_balance: number;
      cross_wallet: number;
      available_balance: number;
      unrealized_profit: number;
    }>;
  }> {
    const { data } = await this.client.get('/wallet-balance');
    return data;
  }

  async getPositions(): Promise<FuturesPosition[]> {
    const { data } = await this.client.get<FuturesPosition[]>('/positions');
    return data || [];
  }

  async closePosition(symbol: string): Promise<{ message: string; order: FuturesOrderResponse }> {
    const { data } = await this.client.post(`/positions/${symbol}/close`);
    return data;
  }

  // Panic button - close all futures positions at once
  async closeAllPositions(): Promise<{
    message: string;
    closed: number;
    total: number;
    errors: string[];
    closed_positions: Array<{
      symbol: string;
      side: string;
      quantity: number;
      order_id: number;
    }>;
  }> {
    const { data } = await this.client.post('/positions/close-all');
    return data;
  }

  // ==================== SETTINGS ====================

  async setLeverage(request: SetLeverageRequest): Promise<LeverageResponse> {
    const { data } = await this.client.post<LeverageResponse>('/leverage', request);
    return data;
  }

  async setMarginType(request: SetMarginTypeRequest): Promise<{ message: string }> {
    const { data } = await this.client.post('/margin-type', request);
    return data;
  }

  async setPositionMode(request: SetPositionModeRequest): Promise<{ message: string }> {
    const { data } = await this.client.post('/position-mode', request);
    return data;
  }

  async getPositionMode(): Promise<PositionModeResponse> {
    const { data } = await this.client.get<PositionModeResponse>('/position-mode');
    return data;
  }

  async getAccountSettings(symbol: string): Promise<FuturesAccountSettings> {
    const { data } = await this.client.get<FuturesAccountSettings>(`/settings/${symbol}`);
    return data;
  }

  // ==================== ORDERS ====================

  async placeOrder(request: PlaceFuturesOrderRequest): Promise<{
    order: FuturesOrderResponse;
    takeProfit?: FuturesOrderResponse;
    stopLoss?: FuturesOrderResponse;
    takeProfitError?: string;
    stopLossError?: string;
    tradeId: number;
  }> {
    const { data } = await this.client.post('/orders', request);
    return data;
  }

  async cancelOrder(symbol: string, orderId: number): Promise<{ message: string }> {
    const { data } = await this.client.delete(`/orders/${symbol}/${orderId}`);
    return data;
  }

  async cancelAllOrders(symbol: string): Promise<{ message: string }> {
    const { data } = await this.client.delete(`/orders/${symbol}/all`);
    return data;
  }

  async getOpenOrders(symbol?: string): Promise<FuturesOrder[]> {
    const params = symbol ? { symbol } : {};
    const { data } = await this.client.get<FuturesOrder[]>('/orders/open', { params });
    return data || [];
  }

  async getAllOrders(): Promise<{
    regular_orders: Array<{
      orderId: number;
      symbol: string;
      side: string;
      positionSide: string;
      type: string;
      price: number;
      origQty: number;
      executedQty: number;
      status: string;
      time: number;
      stopPrice?: number;
    }>;
    algo_orders: Array<{
      algoId: number;
      symbol: string;
      side: string;
      positionSide: string;
      quantity: string;
      executedQty: string;
      price: string;
      triggerPrice: string;
      createTime: number;
      updateTime: number;
      orderType: string;
      algoType: string;
      algoStatus: string;
      closePosition: boolean;
      reduceOnly: boolean;
    }>;
    total_regular: number;
    total_algo: number;
  }> {
    const { data } = await this.client.get('/orders/all');
    return data;
  }

  async cancelAlgoOrder(symbol: string, algoId: number): Promise<{ message: string }> {
    const { data } = await this.client.delete(`/algo-orders/${symbol}/${algoId}`);
    return data;
  }

  // ==================== MARKET DATA ====================

  async getFundingRate(symbol: string): Promise<FundingRate> {
    const { data } = await this.client.get<FundingRate>(`/funding-rate/${symbol}`);
    return data;
  }

  async getOrderBook(symbol: string, limit = 20): Promise<OrderBookDepth> {
    const { data } = await this.client.get<OrderBookDepth>(`/orderbook/${symbol}`, {
      params: { limit },
    });
    return data;
  }

  async getMarkPrice(symbol: string): Promise<MarkPrice> {
    const { data } = await this.client.get<MarkPrice>(`/mark-price/${symbol}`);
    return data;
  }

  async getSymbols(): Promise<string[]> {
    const { data } = await this.client.get<string[]>('/symbols');
    return data || [];
  }

  async getKlines(
    symbol: string,
    interval = '1h',
    limit = 100
  ): Promise<any[]> {
    const { data } = await this.client.get('/klines', {
      params: { symbol, interval, limit },
    });
    return data || [];
  }

  // ==================== HISTORY ====================

  async getTradeHistory(limit = 50, offset = 0): Promise<FuturesTrade[]> {
    const { data } = await this.client.get<FuturesTrade[]>('/trades/history', {
      params: { limit, offset },
    });
    return data || [];
  }

  async getFundingFeeHistory(
    symbol?: string,
    limit = 50,
    offset = 0
  ): Promise<FundingFee[]> {
    const { data } = await this.client.get<FundingFee[]>('/funding-fees/history', {
      params: { symbol, limit, offset },
    });
    return data || [];
  }

  async getTransactionHistory(
    symbol?: string,
    incomeType?: string,
    limit = 50,
    offset = 0
  ): Promise<FuturesTransaction[]> {
    const { data } = await this.client.get<FuturesTransaction[]>('/transactions/history', {
      params: { symbol, income_type: incomeType, limit, offset },
    });
    return data || [];
  }

  async getMetrics(): Promise<FuturesTradingMetrics> {
    const { data } = await this.client.get<FuturesTradingMetrics>('/metrics');
    return data;
  }

  async getTradeSourceStats(): Promise<{
    ai: TradeSourceStats;
    strategy: TradeSourceStats;
    manual: TradeSourceStats;
  }> {
    const { data } = await this.client.get('/trade-source-stats');
    return data;
  }

  async getPositionTradeSources(): Promise<{ sources: Record<string, string> }> {
    const { data } = await this.client.get('/position-trade-sources');
    return data;
  }

  // ==================== AUTOPILOT ====================

  async getAutopilotStatus(): Promise<{
    enabled: boolean;
    running: boolean;
    dry_run: boolean;
    risk_level?: string;
    daily_trades?: number;
    daily_pnl?: number;
    active_positions?: Array<{
      symbol: string;
      side: string;
      entry_price: number;
      quantity: number;
      leverage: number;
      take_profit: number;
      stop_loss: number;
      entry_time: string;
    }>;
    config?: {
      default_leverage: number;
      max_leverage: number;
      margin_type: string;
      position_mode: string;
      take_profit: number;
      stop_loss: number;
      min_confidence: number;
      allow_shorts: boolean;
      trailing_stop: boolean;
    };
    message?: string;
  }> {
    const { data } = await this.client.get('/autopilot/status');
    return data;
  }

  async toggleAutopilot(enabled: boolean, dryRun?: boolean): Promise<{
    success: boolean;
    message: string;
    status: unknown;
  }> {
    const { data } = await this.client.post('/autopilot/toggle', {
      enabled,
      dry_run: dryRun,
    });
    return data;
  }

  async setAutopilotDryRun(dryRun: boolean): Promise<{
    success: boolean;
    message: string;
    dry_run: boolean;
    status: unknown;
  }> {
    const { data } = await this.client.post('/autopilot/dry-run', {
      dry_run: dryRun,
    });
    return data;
  }

  async setAutopilotRiskLevel(riskLevel: string): Promise<{
    success: boolean;
    message: string;
    risk_level: string;
    status: unknown;
  }> {
    const { data } = await this.client.post('/autopilot/risk-level', {
      risk_level: riskLevel,
    });
    return data;
  }

  async setAutopilotAllocation(maxUSDAllocation: number): Promise<{
    success: boolean;
    message: string;
    max_usd_allocation: number;
    status: unknown;
  }> {
    const { data } = await this.client.post('/autopilot/allocation', {
      max_usd_allocation: maxUSDAllocation,
    });
    return data;
  }

  async setAutopilotProfitReinvest(
    profitReinvestPercent: number,
    profitRiskLevel: string
  ): Promise<{
    success: boolean;
    message: string;
    profit_reinvest_percent: number;
    profit_risk_level: string;
    status: unknown;
  }> {
    const { data } = await this.client.post('/autopilot/profit-reinvest', {
      profit_reinvest_percent: profitReinvestPercent,
      profit_risk_level: profitRiskLevel,
    });
    return data;
  }

  async getAutopilotProfitStats(): Promise<{
    total_profit: number;
    profit_pool: number;
    total_usd_allocated: number;
    max_usd_allocation: number;
    profit_reinvest_percent: number;
    profit_reinvest_risk_level: string;
    daily_pnl: number;
  }> {
    const { data } = await this.client.get('/autopilot/profit-stats');
    return data;
  }

  async setAutopilotTPSL(
    takeProfitPercent: number,
    stopLossPercent: number
  ): Promise<{
    success: boolean;
    message: string;
    take_profit_percent: number;
    stop_loss_percent: number;
    status: unknown;
  }> {
    const { data } = await this.client.post('/autopilot/tpsl', {
      take_profit_percent: takeProfitPercent,
      stop_loss_percent: stopLossPercent,
    });
    return data;
  }

  async setAutopilotLeverage(leverage: number): Promise<{
    success: boolean;
    message: string;
    leverage: number;
    status: unknown;
  }> {
    const { data } = await this.client.post('/autopilot/leverage', {
      leverage,
    });
    return data;
  }

  async setAutopilotMinConfidence(minConfidence: number): Promise<{
    success: boolean;
    message: string;
    min_confidence: number;
    status: unknown;
  }> {
    const { data } = await this.client.post('/autopilot/min-confidence', {
      min_confidence: minConfidence,
    });
    return data;
  }

  // ==================== CIRCUIT BREAKER (LOSS CONTROL) ====================

  async getCircuitBreakerStatus(): Promise<{
    available: boolean;
    enabled: boolean;
    state: string;
    can_trade: boolean;
    block_reason: string;
    consecutive_losses: number;
    hourly_loss: number;
    daily_loss: number;
    trades_last_minute: number;
    daily_trades: number;
    trip_reason: string;
    config: {
      enabled: boolean;
      max_loss_per_hour: number;
      max_daily_loss: number;
      max_consecutive_losses: number;
      cooldown_minutes: number;
      max_trades_per_minute: number;
      max_daily_trades: number;
    };
    message?: string;
  }> {
    const { data } = await this.client.get('/autopilot/circuit-breaker/status');
    return data;
  }

  async resetCircuitBreaker(): Promise<{
    success: boolean;
    message: string;
    status: unknown;
  }> {
    const { data } = await this.client.post('/autopilot/circuit-breaker/reset');
    return data;
  }

  async updateCircuitBreakerConfig(config: {
    max_loss_per_hour?: number;
    max_daily_loss?: number;
    max_consecutive_losses?: number;
    cooldown_minutes?: number;
    max_trades_per_minute?: number;
    max_daily_trades?: number;
  }): Promise<{
    success: boolean;
    message: string;
    status: unknown;
  }> {
    const { data } = await this.client.post('/autopilot/circuit-breaker/config', config);
    return data;
  }

  async toggleCircuitBreaker(enabled: boolean): Promise<{
    success: boolean;
    message: string;
    status: unknown;
  }> {
    const { data } = await this.client.post('/autopilot/circuit-breaker/toggle', { enabled });
    return data;
  }

  // ==================== DYNAMIC SL/TP (VOLATILITY-BASED) ====================

  async getDynamicSLTPConfig(): Promise<{
    enabled: boolean;
    atr_period: number;
    atr_multiplier_sl: number;
    atr_multiplier_tp: number;
    llm_weight: number;
    min_sl_percent: number;
    max_sl_percent: number;
    min_tp_percent: number;
    max_tp_percent: number;
  }> {
    const { data } = await this.client.get('/autopilot/dynamic-sltp');
    return data;
  }

  async setDynamicSLTPConfig(config: {
    enabled: boolean;
    atr_period: number;
    atr_multiplier_sl: number;
    atr_multiplier_tp: number;
    llm_weight: number;
    min_sl_percent: number;
    max_sl_percent: number;
    min_tp_percent: number;
    max_tp_percent: number;
  }): Promise<{
    success: boolean;
    message: string;
  }> {
    const { data } = await this.client.post('/autopilot/dynamic-sltp', config);
    return data;
  }

  // ==================== SCALPING MODE ====================

  async getScalpingConfig(): Promise<{
    enabled: boolean;
    min_profit: number;
    quick_reentry: boolean;
    reentry_delay_sec: number;
    max_trades_per_day: number;
    trades_today: number;
  }> {
    const { data } = await this.client.get('/autopilot/scalping');
    return data;
  }

  async setScalpingConfig(config: {
    enabled: boolean;
    min_profit: number;
    quick_reentry: boolean;
    reentry_delay_sec: number;
    max_trades_per_day: number;
  }): Promise<{
    success: boolean;
    message: string;
  }> {
    const { data } = await this.client.post('/autopilot/scalping', config);
    return data;
  }

  // ==================== TP/SL MANAGEMENT ====================

  async setPositionTPSL(
    symbol: string,
    positionSide: string,
    takeProfit?: number,
    stopLoss?: number
  ): Promise<{
    success: boolean;
    message: string;
    take_profit_order?: FuturesOrderResponse;
    stop_loss_order?: FuturesOrderResponse;
  }> {
    const { data } = await this.client.post(`/positions/${symbol}/tpsl`, {
      position_side: positionSide,
      take_profit: takeProfit,
      stop_loss: stopLoss,
    });
    return data;
  }

  async getPositionOrders(symbol: string): Promise<{
    symbol: string;
    open_orders: FuturesOrder[];
    take_profit_orders: FuturesOrder[];
    stop_loss_orders: FuturesOrder[];
    trailing_stop_orders: FuturesOrder[];
  }> {
    const { data } = await this.client.get(`/positions/${symbol}/orders`);
    return data;
  }

  // ==================== ACCOUNT TRADES (DIRECT FROM BINANCE) ====================

  async getAccountTrades(symbol?: string, limit = 50): Promise<{
    trades: Array<{
      symbol: string;
      id: number;
      orderId: number;
      side: string;
      positionSide: string;
      price: number;
      qty: number;
      realizedPnl: number;
      marginAsset: string;
      quoteQty: number;
      commission: number;
      commissionAsset: string;
      time: number;
      buyer: boolean;
      maker: boolean;
    }>;
    errors: string[];
    count: number;
  }> {
    const params = symbol ? { symbol, limit } : { limit };
    const { data } = await this.client.get('/account/trades', { params });
    return data;
  }

  // ==================== RECENT DECISIONS ====================

  async getRecentDecisions(): Promise<{
    success: boolean;
    decisions: Array<{
      timestamp: string;
      symbol: string;
      action: string;
      confidence: number;
      approved: boolean;
      executed: boolean;
      rejection_reason?: string;
      quantity?: number;
      leverage?: number;
      entry_price?: number;
    }>;
    count: number;
  }> {
    const { data } = await this.client.get('/autopilot/recent-decisions');
    return data;
  }

  // ==================== SENTIMENT & NEWS ====================

  async getSentimentNews(limit = 20): Promise<{
    news: Array<{
      title: string;
      source: string;
      url: string;
      sentiment: number;
      published_at: string;
    }>;
    sentiment: {
      overall: number;
      fear_greed_index: number;
      fear_greed_label: string;
      news_score: number;
      trend_score: number;
      updated_at: string;
      sources: string[];
    } | null;
    count: number;
  }> {
    const { data } = await this.client.get('/sentiment/news', { params: { limit } });
    return data;
  }

  // ==================== POSITION AVERAGING ====================

  async getAveragingStatus(): Promise<{
    enabled: boolean;
    config: {
      max_entries: number;
      min_confidence: number;
      min_price_improve: number;
      cooldown_mins: number;
      news_weight: number;
    };
    positions: Array<{
      symbol: string;
      side: string;
      entry_count: number;
      avg_entry: number;
      quantity: number;
      entry_history: Array<{
        price: number;
        quantity: number;
        time: string;
        confidence: number;
        news_score: number;
      }>;
    }>;
  }> {
    const { data } = await this.client.get('/autopilot/averaging/status');
    return data;
  }

  async setAveragingConfig(config: {
    enabled?: boolean;
    max_entries?: number;
    min_confidence?: number;
    min_price_improve?: number;
    cooldown_mins?: number;
    news_weight?: number;
  }): Promise<{
    success: boolean;
    message: string;
    status: unknown;
  }> {
    const { data } = await this.client.post('/autopilot/averaging/config', config);
    return data;
  }
}

// Export singleton instance
export const futuresApi = new FuturesAPIService();

// Helper functions

export function calculateROE(
  unrealizedPnl: number,
  entryPrice: number,
  quantity: number,
  leverage: number
): number {
  const positionValue = entryPrice * Math.abs(quantity);
  const margin = positionValue / leverage;
  if (margin === 0) return 0;
  return (unrealizedPnl / margin) * 100;
}

export function calculateLiquidationPrice(
  entryPrice: number,
  leverage: number,
  positionSide: 'LONG' | 'SHORT' | 'BOTH',
  marginType: 'CROSSED' | 'ISOLATED',
  maintenanceMarginRate = 0.004 // 0.4% for most symbols
): number {
  // Simplified calculation - actual calculation is more complex
  const direction = positionSide === 'SHORT' ? -1 : 1;
  const marginRatio = 1 / leverage;

  if (marginType === 'ISOLATED') {
    // For isolated margin
    return entryPrice * (1 - direction * (marginRatio - maintenanceMarginRate));
  } else {
    // For cross margin, liquidation depends on total account equity
    // This is a simplified approximation
    return entryPrice * (1 - direction * (marginRatio - maintenanceMarginRate) * 0.8);
  }
}

export function formatQuantity(quantity: number, precision = 4): string {
  return quantity.toFixed(precision);
}

export function formatPrice(price: number, precision = 2): string {
  return price.toLocaleString('en-US', {
    minimumFractionDigits: precision,
    maximumFractionDigits: precision,
  });
}

export function formatUSD(value: number | string | null | undefined): string {
  const num = typeof value === 'string' ? parseFloat(value) : value;
  if (num === null || num === undefined || isNaN(num)) {
    return '$0.00';
  }
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(num);
}

export function formatPercent(value: number | string | null | undefined, includeSign = true): string {
  const num = typeof value === 'string' ? parseFloat(value) : value;
  if (num === null || num === undefined || isNaN(num)) {
    return '0.00%';
  }
  const sign = includeSign && num > 0 ? '+' : '';
  return `${sign}${num.toFixed(2)}%`;
}

export function formatFundingRate(rate: number): string {
  return `${(rate * 100).toFixed(4)}%`;
}

export function getPositionColor(pnl: number | string | null | undefined): string {
  const num = typeof pnl === 'string' ? parseFloat(pnl) : pnl;
  if (num === null || num === undefined || isNaN(num)) return 'text-gray-400';
  if (num > 0) return 'text-green-500';
  if (num < 0) return 'text-red-500';
  return 'text-gray-400';
}

export function getSideColor(side: string): string {
  return side === 'BUY' ? 'text-green-500' : 'text-red-500';
}

export function getPositionSideLabel(
  positionAmt: number,
  positionSide: string
): { label: string; color: string } {
  if (positionSide === 'LONG' || (positionSide === 'BOTH' && positionAmt > 0)) {
    return { label: 'Long', color: 'text-green-500' };
  }
  if (positionSide === 'SHORT' || (positionSide === 'BOTH' && positionAmt < 0)) {
    return { label: 'Short', color: 'text-red-500' };
  }
  return { label: '-', color: 'text-gray-400' };
}

export function getOrderTypeLabel(type: string): string {
  const labels: Record<string, string> = {
    LIMIT: 'Limit',
    MARKET: 'Market',
    STOP: 'Stop-Limit',
    STOP_MARKET: 'Stop-Market',
    TAKE_PROFIT: 'Take-Profit',
    TAKE_PROFIT_MARKET: 'Take-Profit-Market',
    TRAILING_STOP_MARKET: 'Trailing Stop',
  };
  return labels[type] || type;
}

export function getTimeInForceLabel(tif: string): string {
  const labels: Record<string, string> = {
    GTC: 'Good Till Cancel',
    IOC: 'Immediate or Cancel',
    FOK: 'Fill or Kill',
    GTX: 'Post Only',
  };
  return labels[tif] || tif;
}
