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

    // Request interceptor to add auth token
    this.client.interceptors.request.use(
      (config) => {
        const token = localStorage.getItem('access_token');
        if (token) {
          config.headers.Authorization = `Bearer ${token}`;
        }
        return config;
      },
      (error) => {
        return Promise.reject(error);
      }
    );

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

  async getTradeHistory(limit = 50, offset = 0, includeAI = false): Promise<FuturesTrade[]> {
    const { data } = await this.client.get<FuturesTrade[]>('/trades/history', {
      params: { limit, offset, include_ai: includeAI, include_open: true },
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

  async setPositionROITarget(
    symbol: string,
    roiPercent: number,
    saveForFuture: boolean = false
  ): Promise<{
    success: boolean;
    message: string;
    symbol: string;
    roi_percent: number;
    save_for_future: boolean;
  }> {
    const { data } = await this.client.post(`/ginie/positions/${symbol}/roi-target`, {
      roi_percent: roiPercent,
      save_for_future: saveForFuture,
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

  async getSentimentNews(limit = 50, ticker?: string): Promise<{
    news: Array<{
      title: string;
      source: string;
      url: string;
      sentiment: number;
      published_at: string;
      tickers: string[];
      topic: string;
      is_important: boolean;
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
    stats: {
      bullish: number;
      bearish: number;
      neutral: number;
    };
    tickers: string[];
    count: number;
  }> {
    const params: Record<string, unknown> = { limit };
    if (ticker) params.ticker = ticker;
    const { data } = await this.client.get('/sentiment/news', { params });
    return data;
  }

  async getBreakingNews(limit = 10): Promise<{
    news: Array<{
      title: string;
      source: string;
      url: string;
      sentiment: number;
      published_at: string;
      tickers: string[];
      topic: string;
      is_important: boolean;
    }>;
    count: number;
  }> {
    const { data } = await this.client.get('/sentiment/breaking', { params: { limit } });
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

  // ==================== AI DECISIONS (from database) ====================

  async getAIDecisions(limit = 50, symbol?: string, action?: string): Promise<{
    success: boolean;
    data: Array<{
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
    }>;
    count: number;
  }> {
    const params: Record<string, string | number> = { limit };
    if (symbol) params.symbol = symbol;
    if (action) params.action = action;

    // This endpoint is at /api/ai-decisions, not /api/futures
    const response = await axios.get('/api/ai-decisions', { params });
    return response.data;
  }

  // ==================== INVESTIGATE / DIAGNOSTICS ====================

  async getInvestigateStatus(): Promise<InvestigateStatus> {
    const { data } = await this.client.get('/autopilot/investigate');
    return data;
  }

  async clearFlipFlopCooldown(): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post('/autopilot/clear-cooldown');
    return data;
  }

  async forceSyncPositions(): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post('/autopilot/force-sync');
    return data;
  }

  async recalculateAllocation(): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post('/autopilot/recalculate-allocation');
    return data;
  }

  // ==================== COIN CLASSIFICATION ====================

  async getCoinClassifications(): Promise<{
    classifications: CoinClassification[];
    settings: CoinClassificationSettings;
  }> {
    const { data } = await this.client.get('/autopilot/coin-classifications');
    return data;
  }

  async getCoinClassificationSummary(): Promise<ClassificationSummary> {
    const { data } = await this.client.get('/autopilot/coin-classifications/summary');
    return data;
  }

  async updateCoinPreference(
    symbol: string,
    enabled: boolean,
    priority = 0
  ): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post('/autopilot/coin-preference', {
      symbol,
      enabled,
      priority,
    });
    return data;
  }

  async updateCategoryAllocation(
    category: string, // e.g., "volatility:stable", "market_cap:blue_chip"
    enabled: boolean,
    allocationPercent: number,
    maxPositions: number
  ): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post('/autopilot/category-allocation', {
      category,
      enabled,
      allocation_percent: allocationPercent,
      max_positions: maxPositions,
    });
    return data;
  }

  async getCoinPreferences(): Promise<{
    coins: CoinPreferenceInfo[];
    total: number;
    settings: CoinClassificationSettings;
  }> {
    const { data } = await this.client.get('/autopilot/coin-preferences');
    return data;
  }

  async bulkUpdateCoinPreferences(
    coins: Array<{ symbol: string; enabled: boolean; priority: number }>
  ): Promise<{ success: boolean; message: string; updated: number }> {
    const { data } = await this.client.post('/autopilot/coin-preferences/bulk', { coins });
    return data;
  }

  async getEligibleCoins(): Promise<{
    eligible_coins: EligibleCoin[];
    total: number;
  }> {
    const { data } = await this.client.get('/autopilot/coins/eligible');
    return data;
  }

  async refreshCoinClassifications(): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post('/autopilot/coin-classifications/refresh');
    return data;
  }

  async enableAllCoins(): Promise<{ success: boolean; message: string; enabled: number }> {
    const { data } = await this.client.post('/autopilot/coins/enable-all');
    return data;
  }

  async disableAllCoins(): Promise<{ success: boolean; message: string; disabled: number }> {
    const { data } = await this.client.post('/autopilot/coins/disable-all');
    return data;
  }

  // ==================== TRADING STYLE ====================

  async getTradingStyle(): Promise<{
    style: 'scalping' | 'swing' | 'position';
    config: TradingStyleConfig;
  }> {
    const { data } = await this.client.get('/autopilot/trading-style');
    return data;
  }

  async setTradingStyle(style: 'scalping' | 'swing' | 'position'): Promise<{
    success: boolean;
    message: string;
    style: string;
    config: TradingStyleConfig;
  }> {
    const { data } = await this.client.post('/autopilot/trading-style', { style });
    return data;
  }

  // ==================== HEDGING ====================

  async getHedgingStatus(): Promise<HedgingStatus> {
    const { data } = await this.client.get('/autopilot/hedging/status');
    return data;
  }

  async getHedgingConfig(): Promise<HedgingStatus> {
    const { data } = await this.client.get('/autopilot/hedging/config');
    return data;
  }

  async updateHedgingConfig(config: {
    enabled?: boolean;
    price_drop_trigger_pct?: number;
    unrealized_loss_trigger?: number;
    ai_enabled?: boolean;
    ai_confidence_min?: number;
    default_percent?: number;
    partial_steps?: number[];
    profit_take_pct?: number;
    close_on_recovery_pct?: number;
    max_simultaneous?: number;
  }): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post('/autopilot/hedging/config', config);
    return data;
  }

  async executeManualHedge(
    symbol: string,
    hedgePercent: number
  ): Promise<{ success: boolean; hedge: HedgePositionInfo }> {
    const { data } = await this.client.post('/autopilot/hedging/manual', {
      symbol,
      hedge_percent: hedgePercent,
    });
    return data;
  }

  async closeHedge(
    symbol: string,
    reason = 'manual_close'
  ): Promise<{ success: boolean; pnl: number; symbol: string; reason: string }> {
    const { data } = await this.client.post('/autopilot/hedging/close', {
      symbol,
      reason,
    });
    return data;
  }

  async enableHedgeMode(): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post('/autopilot/hedging/enable-mode');
    return data;
  }

  async clearAllHedges(): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post('/autopilot/hedging/clear-all');
    return data;
  }

  async getHedgeHistory(symbol: string): Promise<HedgeEvent[]> {
    const { data } = await this.client.get('/autopilot/hedging/history', {
      params: { symbol },
    });
    return data;
  }

  // ==================== GINIE AI TRADER ====================

  async getGinieStatus(): Promise<GinieStatus> {
    const { data } = await this.client.get('/ginie/status');
    return data;
  }

  async getGinieConfig(): Promise<GinieConfig> {
    const { data } = await this.client.get('/ginie/config');
    return data;
  }

  async updateGinieConfig(config: Partial<GinieConfig>): Promise<{
    success: boolean;
    message: string;
    config: GinieConfig;
  }> {
    const { data } = await this.client.post('/ginie/config', config);
    return data;
  }

  async toggleGinie(enabled: boolean): Promise<{
    success: boolean;
    message: string;
    enabled: boolean;
  }> {
    const { data } = await this.client.post('/ginie/toggle', { enabled });
    return data;
  }

  async ginieScanCoin(symbol: string): Promise<GinieCoinScan> {
    const { data } = await this.client.get('/ginie/scan', {
      params: { symbol },
    });
    return data;
  }

  async ginieGenerateDecision(symbol: string): Promise<GinieDecisionReport> {
    const { data } = await this.client.get('/ginie/decision', {
      params: { symbol },
    });
    return data;
  }

  async ginieGetDecisions(): Promise<{
    decisions: GinieDecisionReport[];
    count: number;
  }> {
    const { data } = await this.client.get('/ginie/decisions');
    return data;
  }

  async ginieScanAll(): Promise<{
    scans: GinieCoinScan[];
    count: number;
    symbols: string[];
  }> {
    const { data } = await this.client.post('/ginie/scan-all');
    return data;
  }

  async ginieAnalyzeAll(): Promise<{
    decisions: GinieDecisionReport[];
    count: number;
    best_long: GinieDecisionReport | null;
    best_short: GinieDecisionReport | null;
  }> {
    const { data } = await this.client.post('/ginie/analyze-all');
    return data;
  }

  // ==================== GINIE AUTOPILOT ====================

  async getGinieAutopilotStatus(): Promise<GinieAutopilotStatus> {
    const { data } = await this.client.get('/ginie/autopilot/status');
    return data;
  }

  async getGinieAutopilotConfig(): Promise<GinieAutopilotConfig> {
    const { data } = await this.client.get('/ginie/autopilot/config');
    return data;
  }

  async updateGinieAutopilotConfig(config: Partial<GinieAutopilotConfig>): Promise<{
    success: boolean;
    message: string;
    config: GinieAutopilotConfig;
  }> {
    const { data } = await this.client.post('/ginie/autopilot/config', config);
    return data;
  }

  async startGinieAutopilot(): Promise<{
    success: boolean;
    message: string;
    running: boolean;
  }> {
    const { data } = await this.client.post('/ginie/autopilot/start');
    return data;
  }

  async stopGinieAutopilot(): Promise<{
    success: boolean;
    message: string;
    running: boolean;
  }> {
    const { data } = await this.client.post('/ginie/autopilot/stop');
    return data;
  }

  async getGinieAutopilotPositions(): Promise<{
    positions: GiniePosition[];
    count: number;
  }> {
    const { data } = await this.client.get('/ginie/autopilot/positions');
    return data;
  }

  async getGinieAutopilotTradeHistory(): Promise<{
    trades: GinieTradeResult[];
    count: number;
  }> {
    const { data } = await this.client.get('/ginie/autopilot/history');
    return data;
  }

  async clearGiniePositions(): Promise<{
    success: boolean;
    message: string;
  }> {
    const { data } = await this.client.post('/ginie/autopilot/clear');
    return data;
  }

  async setGinieDryRun(dryRun: boolean): Promise<{
    success: boolean;
    message: string;
    config: GinieAutopilotConfig;
  }> {
    // Get current config, update dry_run, and send back
    const currentConfig = await this.getGinieAutopilotConfig();
    const { data } = await this.client.post('/ginie/autopilot/config', {
      ...currentConfig,
      dry_run: dryRun,
    });
    return data;
  }

  // ==================== GINIE CIRCUIT BREAKER ====================

  async getGinieCircuitBreakerStatus(): Promise<GinieCircuitBreakerStatus> {
    const { data } = await this.client.get('/ginie/circuit-breaker/status');
    return data;
  }

  async resetGinieCircuitBreaker(): Promise<{
    success: boolean;
    message: string;
    status: GinieCircuitBreakerStatus;
  }> {
    const { data } = await this.client.post('/ginie/circuit-breaker/reset');
    return data;
  }

  async toggleGinieCircuitBreaker(enabled: boolean): Promise<{
    success: boolean;
    message: string;
    enabled: boolean;
    status: GinieCircuitBreakerStatus;
  }> {
    const { data } = await this.client.post('/ginie/circuit-breaker/toggle', { enabled });
    return data;
  }

  async updateGinieCircuitBreakerConfig(config: {
    max_loss_per_hour: number;
    max_daily_loss: number;
    max_consecutive_losses: number;
    cooldown_minutes: number;
  }): Promise<{
    success: boolean;
    message: string;
    status: GinieCircuitBreakerStatus;
  }> {
    const { data } = await this.client.post('/ginie/circuit-breaker/config', config);
    return data;
  }

  // ==================== GINIE POSITION SYNC ====================

  async syncGiniePositions(): Promise<{
    success: boolean;
    message: string;
    synced_count: number;
    total_positions: number;
    positions: GiniePosition[];
  }> {
    const { data } = await this.client.post('/ginie/positions/sync');
    return data;
  }

  // ==================== GINIE PANIC BUTTON ====================

  async closeAllGiniePositions(): Promise<{
    success: boolean;
    message: string;
    positions_closed: number;
    total_pnl: number;
  }> {
    const { data } = await this.client.post('/ginie/positions/close-all');
    return data;
  }

  // ==================== GINIE RISK LEVEL ====================

  async getGinieRiskLevel(): Promise<{
    risk_level: string;
    min_confidence: number;
    max_usd: number;
    leverage: number;
  }> {
    const { data } = await this.client.get('/ginie/risk-level');
    return data;
  }

  async setGinieRiskLevel(riskLevel: string): Promise<{
    success: boolean;
    message: string;
    risk_level: string;
    min_confidence: number;
    max_usd: number;
    leverage: number;
  }> {
    const { data } = await this.client.post('/ginie/risk-level', { risk_level: riskLevel });
    return data;
  }

  // ==================== GINIE TREND TIMEFRAMES ====================

  async getGinieTrendTimeframes(): Promise<{
    success: boolean;
    timeframes: {
      scalp: string;
      swing: string;
      position: string;
      block_on_divergence: boolean;
    };
    valid_timeframes: string[];
  }> {
    const { data } = await this.client.get('/ginie/trend-timeframes');
    return data;
  }

  async updateGinieTrendTimeframes(config: {
    scalp_timeframe?: string;
    swing_timeframe?: string;
    position_timeframe?: string;
    ultrafast_timeframe?: string;
    block_on_divergence?: boolean;
  }): Promise<{
    success: boolean;
    message: string;
    timeframes: {
      scalp: string;
      swing: string;
      position: string;
      ultra_fast?: string;
      ultrafast?: string;
      block_on_divergence: boolean;
    };
  }> {
    const { data } = await this.client.post('/ginie/trend-timeframes', config);
    return data;
  }

  // ==================== GINIE SL/TP CONFIGURATION ====================

  async getGinieSLTPConfig(): Promise<{
    success: boolean;
    sltp_config: {
      scalp: {
        sl_percent: number;
        tp_percent: number;
        trailing_enabled: boolean;
        trailing_percent: number;
        trailing_activation: number;
      };
      swing: {
        sl_percent: number;
        tp_percent: number;
        trailing_enabled: boolean;
        trailing_percent: number;
        trailing_activation: number;
      };
      position: {
        sl_percent: number;
        tp_percent: number;
        trailing_enabled: boolean;
        trailing_percent: number;
        trailing_activation: number;
      };
    };
    tp_mode: {
      use_single_tp: boolean;
      single_tp_percent: number;
      tp1_percent: number;
      tp2_percent: number;
      tp3_percent: number;
      tp4_percent: number;
    };
  }> {
    const { data } = await this.client.get('/ginie/sltp-config');
    return data;
  }

  async updateGinieSLTP(mode: 'ultra_fast' | 'scalp' | 'swing' | 'position', config: {
    sl_percent?: number;
    tp_percent?: number;
    trailing_enabled?: boolean;
    trailing_percent?: number;
    trailing_activation?: number;
  }): Promise<{
    success: boolean;
    message: string;
    config: {
      sl_percent: number;
      tp_percent: number;
      trailing_enabled: boolean;
      trailing_percent: number;
      trailing_activation: number;
    };
  }> {
    // API expects 'ultrafast' without underscore
    const apiMode = mode === 'ultra_fast' ? 'ultrafast' : mode;
    const { data } = await this.client.post(`/ginie/sltp/${apiMode}`, config);
    return data;
  }

  async updateGinieTPMode(config: {
    use_single_tp?: boolean;
    single_tp_percent?: number;
    tp1_percent?: number;
    tp2_percent?: number;
    tp3_percent?: number;
    tp4_percent?: number;
  }): Promise<{
    success: boolean;
    message: string;
    config: {
      use_single_tp: boolean;
      single_tp_percent: number;
      tp1_percent: number;
      tp2_percent: number;
      tp3_percent: number;
      tp4_percent: number;
    };
  }> {
    const { data } = await this.client.post('/ginie/tp-mode', config);
    return data;
  }

  // ==================== ULTRAFAST SCALPING MODE ====================

  async getUltraFastConfig(): Promise<{
    success: boolean;
    ultrafast_config: {
      enabled: boolean;
      scan_interval_ms: number;
      monitor_interval_ms: number;
      max_positions: number;
      max_usd_per_pos: number;
      min_confidence: number;
      min_profit_pct: number;
      max_hold_ms: number;
      max_daily_trades: number;
    };
    ultrafast_stats: {
      today_trades: number;
      daily_pnl: number;
      total_pnl: number;
      win_rate: number;
      last_update: string;
    };
  }> {
    const { data } = await this.client.get('/ultrafast/config');
    return data;
  }

  async updateUltraFastConfig(config: {
    enabled?: boolean;
    scan_interval_ms?: number;
    monitor_interval_ms?: number;
    max_positions?: number;
    max_usd_per_pos?: number;
    min_confidence?: number;
    min_profit_pct?: number;
    max_hold_ms?: number;
    max_daily_trades?: number;
  }): Promise<{
    success: boolean;
    message: string;
    config: {
      enabled: boolean;
      scan_interval_ms: number;
      monitor_interval_ms: number;
      max_positions: number;
      max_usd_per_pos: number;
      min_confidence: number;
      min_profit_pct: number;
      max_hold_ms: number;
      max_daily_trades: number;
    };
  }> {
    const { data } = await this.client.post('/ultrafast/config', config);
    return data;
  }

  async toggleUltraFast(enabled: boolean): Promise<{
    success: boolean;
    message: string;
    enabled: boolean;
  }> {
    const { data } = await this.client.post('/ultrafast/toggle', { enabled });
    return data;
  }

  async resetUltraFastStats(): Promise<{
    success: boolean;
    message: string;
    stats: {
      today_trades: number;
      daily_pnl: number;
    };
  }> {
    const { data } = await this.client.post('/ultrafast/reset-stats', {});
    return data;
  }

  // ==================== GINIE MARKET MOVERS ====================

  async getMarketMovers(topN: number = 20): Promise<MarketMoversResponse> {
    const { data } = await this.client.get(`/ginie/market-movers?top=${topN}`);
    return data;
  }

  async refreshDynamicSymbols(topN: number = 15): Promise<{
    success: boolean;
    message: string;
    top_n: number;
    symbol_count: number;
    symbols: string[];
  }> {
    const { data } = await this.client.post('/ginie/symbols/refresh-dynamic', { top_n: topN });
    return data;
  }

  // ==================== GINIE DIAGNOSTICS ====================

  async getGinieDiagnostics(): Promise<GinieDiagnostics> {
    const { data } = await this.client.get('/ginie/diagnostics');
    return data;
  }

  // ==================== GINIE SIGNAL LOGS ====================

  async getGinieSignalLogs(limit = 100, status?: string, symbol?: string): Promise<{
    signals: GinieSignalLog[];
    count: number;
  }> {
    const params: Record<string, string | number> = { limit };
    if (status) params.status = status;
    if (symbol) params.symbol = symbol;
    const { data } = await this.client.get('/ginie/signals', { params });
    return data;
  }

  async getGinieSignalStats(): Promise<GinieSignalStats> {
    const { data } = await this.client.get('/ginie/signals/stats');
    return data;
  }

  // ==================== GINIE SL UPDATE HISTORY ====================

  async getGinieSLHistory(symbol?: string): Promise<{
    history: Record<string, GinieSLUpdateHistory>;
    count: number;
  }> {
    const params: Record<string, string> = {};
    if (symbol) params.symbol = symbol;
    const { data } = await this.client.get('/ginie/sl-history', { params });
    return data;
  }

  async getGinieSLStats(): Promise<GinieSLStats> {
    const { data } = await this.client.get('/ginie/sl-history/stats');
    return data;
  }

  // ==================== GINIE LLM SL STATUS ====================

  async getGinieLLMSLStatus(): Promise<GinieLLMSLStatus> {
    const { data } = await this.client.get('/ginie/llm-sl/status');
    return data;
  }

  async resetGinieLLMSL(symbol: string): Promise<{
    success: boolean;
    message: string;
    symbol: string;
    was_disabled: boolean;
  }> {
    const { data } = await this.client.post(`/ginie/llm-sl/reset/${symbol}`);
    return data;
  }

  // ==================== SYMBOL PERFORMANCE SETTINGS ====================

  async getSymbolPerformanceSettings(): Promise<{
    symbols: Record<string, SymbolPerformanceSettings>;
    category_config: {
      confidence_boost: Record<string, number>;
      size_multiplier: Record<string, number>;
    };
    global_min_confidence: number;
    global_max_usd: number;
  }> {
    const { data } = await this.client.get('/autopilot/symbols');
    return data;
  }

  async getSymbolPerformanceReport(): Promise<{
    report: SymbolPerformanceReport[];
    count: number;
  }> {
    const { data } = await this.client.get('/autopilot/symbols/report');
    return data;
  }

  async getSymbolsByCategory(category: string): Promise<{
    symbols: SymbolPerformanceReport[];
    category: string;
    count: number;
  }> {
    const { data } = await this.client.get(`/autopilot/symbols/category/${category}`);
    return data;
  }

  async getSingleSymbolSettings(symbol: string): Promise<SymbolPerformanceSettings> {
    const { data } = await this.client.get(`/autopilot/symbols/${symbol}`);
    return data;
  }

  async updateSymbolSettings(symbol: string, settings: {
    category?: string;
    enabled?: boolean;
    size_multiplier?: number;
    min_confidence?: number;
    max_position_usd?: number;
    notes?: string;
  }): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.put(`/autopilot/symbols/${symbol}`, settings);
    return data;
  }

  async blacklistSymbol(symbol: string, reason: string): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post(`/autopilot/symbols/${symbol}/blacklist`, { reason });
    return data;
  }

  async unblacklistSymbol(symbol: string): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.delete(`/autopilot/symbols/${symbol}/blacklist`);
    return data;
  }

  async updateCategoryConfig(config: {
    confidence_boost: Record<string, number>;
    size_multiplier: Record<string, number>;
  }): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post('/autopilot/category-config', config);
    return data;
  }

  // ==================== AUTO MODE (LLM-DRIVEN TRADING) ====================

  async getAutoModeConfig(): Promise<AutoModeConfig> {
    const { data } = await this.client.get('/autopilot/auto-mode');
    return data;
  }

  async setAutoModeConfig(config: Partial<AutoModeConfig>): Promise<{
    success: boolean;
    message: string;
    config: AutoModeConfig;
  }> {
    const { data } = await this.client.post('/autopilot/auto-mode', config);
    return data;
  }

  async toggleAutoMode(enabled: boolean): Promise<{
    success: boolean;
    message: string;
    enabled: boolean;
  }> {
    const { data } = await this.client.post('/autopilot/auto-mode/toggle', { enabled });
    return data;
  }

  // ==================== STRATEGY PERFORMANCE ====================

  async getStrategyPerformance(): Promise<{
    strategies: StrategyPerformance[];
    count: number;
  }> {
    const { data } = await this.client.get('/ginie/strategy-performance');
    return data;
  }

  async getSourcePerformance(): Promise<{
    sources: SourcePerformance[];
  }> {
    const { data } = await this.client.get('/ginie/source-performance');
    return data;
  }

  async getPositionsBySource(source: 'ai' | 'strategy' | 'all' = 'all'): Promise<{
    positions: GiniePosition[];
    count: number;
    filter: string;
  }> {
    const { data } = await this.client.get('/ginie/positions/filter', {
      params: { source }
    });
    return data;
  }

  async getTradeHistoryBySource(source: 'ai' | 'strategy' | 'all' = 'all', limit = 100): Promise<{
    trades: GinieTradeResult[];
    count: number;
    filter: string;
  }> {
    const { data } = await this.client.get('/ginie/history/filter', {
      params: { source, limit }
    });
    return data;
  }

  // ==================== MODE ALLOCATION ====================

  async getModeAllocations(): Promise<any> {
    const { data } = await this.client.get('/modes/allocations');
    return data;
  }

  async updateModeAllocations(allocations: any): Promise<any> {
    const { data } = await this.client.post('/modes/allocations', allocations);
    return data;
  }

  async getModeAllocationHistory(mode?: string, limit = 100): Promise<any> {
    const endpoint = mode ? `/modes/allocations/${mode}` : '/modes/allocations/history';
    const { data } = await this.client.get(endpoint, {
      params: { limit }
    });
    return data;
  }

  // ==================== MODE SAFETY ====================

  async getModeSafetyStatus(): Promise<any> {
    const { data } = await this.client.get('/modes/safety');
    return data;
  }

  async resumeMode(mode: string): Promise<any> {
    const { data } = await this.client.post(`/modes/safety/${mode}/resume`, {});
    return data;
  }

  async getModeSafetyHistory(mode?: string, limit = 100): Promise<any> {
    const endpoint = mode ? `/modes/safety/${mode}/history` : '/modes/safety/history';
    const { data } = await this.client.get(endpoint, {
      params: { limit }
    });
    return data;
  }

  // ==================== MODE PERFORMANCE ====================

  async getModePerformance(): Promise<any> {
    const { data } = await this.client.get('/modes/performance');
    return data;
  }

  async getModePerformanceByMode(mode: string): Promise<any> {
    const { data } = await this.client.get(`/modes/performance/${mode}`);
    return data;
  }

  // ==================== TRADE HISTORY & PERFORMANCE ====================

  async getTradeHistoryWithDateRange(startDate?: string, endDate?: string): Promise<{
    trades: GinieTradeResult[];
    count: number;
  }> {
    const params: Record<string, string> = {};
    if (startDate) params.start = startDate;
    if (endDate) params.end = endDate;
    const { data } = await this.client.get('/ginie/trade-history', { params });
    return data;
  }

  async getPerformanceMetrics(startDate?: string, endDate?: string): Promise<any> {
    const params: Record<string, string> = {};
    if (startDate) params.start = startDate;
    if (endDate) params.end = endDate;
    const { data } = await this.client.get('/ginie/performance-metrics', { params });
    return data;
  }

  // ==================== LLM DIAGNOSTICS ====================

  async getLLMDiagnostics(): Promise<{
    switches: Array<{
      timestamp: string;
      symbol: string;
      action: 'enable' | 'disable';
      reason: string;
    }>;
    count: number;
  }> {
    const { data } = await this.client.get('/ginie/llm-diagnostics');
    return data;
  }

  async resetLLMDiagnostics(): Promise<{
    success: boolean;
    message: string;
  }> {
    const { data } = await this.client.post('/ginie/llm-diagnostics/reset', {});
    return data;
  }

  // ==================== MODE CONFIGURATION (Story 2.7) ====================

  async getModeConfigs(): Promise<ModeConfigsResponse> {
    const { data } = await this.client.get('/ginie/mode-configs');
    return data;
  }

  async getModeConfig(mode: string): Promise<{
    success: boolean;
    mode: string;
    config: ModeFullConfig;
  }> {
    const { data } = await this.client.get(`/ginie/mode-config/${mode}`);
    return data;
  }

  async updateModeConfig(mode: string, config: ModeFullConfig): Promise<{
    success: boolean;
    message: string;
    mode: string;
    config: ModeFullConfig;
  }> {
    const { data } = await this.client.put(`/ginie/mode-config/${mode}`, config);
    return data;
  }

  async resetModeConfigs(): Promise<{
    success: boolean;
    message: string;
    mode_configs: Record<string, ModeFullConfig>;
  }> {
    const { data } = await this.client.post('/ginie/mode-config/reset', {});
    return data;
  }

  // ==================== LLM & ADAPTIVE AI (Story 2.8) ====================

  async getLLMConfig(): Promise<LLMConfigResponse> {
    const { data } = await this.client.get('/ginie/llm/config');
    return data;
  }

  async updateLLMConfig(config: Partial<LLMConfig>): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.put('/ginie/llm-config', config);
    return data;
  }

  async updateModeLLMSettings(mode: string, settings: Partial<ModeLLMSettings>): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.put(`/ginie/llm-config/${mode}`, settings);
    return data;
  }

  async getAdaptiveRecommendations(): Promise<AdaptiveRecommendationsResponse> {
    const { data } = await this.client.get('/ginie/adaptive/recommendations');
    return data;
  }

  async applyRecommendation(id: string): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post(`/ginie/adaptive/recommendations/${id}/apply`);
    return data;
  }

  async dismissRecommendation(id: string): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post(`/ginie/adaptive/recommendations/${id}/dismiss`);
    return data;
  }

  async applyAllRecommendations(): Promise<{ success: boolean; applied: string[] }> {
    const { data } = await this.client.post('/ginie/adaptive/recommendations/apply-all');
    return data;
  }

  async getLLMCallDiagnostics(): Promise<LLMCallDiagnostics> {
    const { data } = await this.client.get('/ginie/llm/diagnostics');
    return data;
  }

  async resetLLMCallDiagnostics(): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post('/ginie/llm/diagnostics/reset');
    return data;
  }

  async getTradeHistoryWithAI(limit = 50, offset = 0): Promise<{ trades: TradeWithAI[]; total: number }> {
    const { data } = await this.client.get('/ginie/trades/with-ai', {
      params: { limit, offset }
    });
    return data;
  }

  async updateAdaptiveConfig(config: Partial<AdaptiveAIConfig>): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post('/ginie/adaptive/config', config);
    return data;
  }
}

// ==================== MARKET MOVERS TYPES ====================

export interface MarketMoversResponse {
  success: boolean;
  top_n: number;
  top_gainers: string[];
  top_losers: string[];
  top_volume: string[];
  high_volatility: string[];
}

// ==================== GINIE AUTOPILOT TYPES ====================

export interface GinieAutopilotConfig {
  enabled: boolean;
  max_positions: number;
  max_usd_per_position: number;
  total_max_usd: number;
  default_leverage: number;
  dry_run: boolean;
  risk_level: string;
  enable_scalp_mode: boolean;
  enable_swing_mode: boolean;
  enable_position_mode: boolean;
  tp1_percent: number;
  tp2_percent: number;
  tp3_percent: number;
  tp4_percent: number;
  move_to_breakeven_after_tp1: boolean;
  breakeven_buffer: number;
  scalp_scan_interval: number;
  swing_scan_interval: number;
  position_scan_interval: number;
  min_confidence_to_trade: number;
  max_daily_trades: number;
  max_daily_loss: number;
}

export interface GinieTakeProfitLevel {
  level: number;
  price: number;
  percent: number;
  gain_pct: number;
  status: string;
}

export interface GiniePosition {
  symbol: string;
  side: string;
  mode: GinieTradingMode;
  entry_price: number;
  original_qty: number;
  remaining_qty: number;
  leverage: number;
  entry_time: string;
  take_profits: GinieTakeProfitLevel[];
  current_tp_level: number;
  stop_loss: number;
  original_sl: number;
  moved_to_breakeven: boolean;
  trailing_active: boolean;
  highest_price: number;
  lowest_price: number;
  trailing_percent: number;
  realized_pnl: number;
  unrealized_pnl: number;
  // Trade source tracking
  source: 'ai' | 'strategy';
  strategy_id?: number;
  strategy_name?: string;
}

export interface GinieTradeResult {
  symbol: string;
  action: string;
  side: string;
  quantity: number;
  price: number;
  pnl: number;
  pnl_percent: number;
  reason: string;
  tp_level?: number;
  timestamp: string;
  // Trade source tracking
  source?: 'ai' | 'strategy';
  strategy_id?: number;
  strategy_name?: string;
}

export interface StrategyPerformance {
  strategy_id: number;
  strategy_name: string;
  total_trades: number;
  winning_trades: number;
  losing_trades: number;
  total_pnl: number;
  win_rate: number;
  avg_pnl: number;
  avg_win: number;
  avg_loss: number;
  largest_win: number;
  largest_loss: number;
  last_trade_time: string;
}

export interface SourcePerformance {
  source: string;
  total_trades: number;
  winning_trades: number;
  total_pnl: number;
  win_rate: number;
  avg_pnl: number;
}

export interface GinieAutopilotStats {
  running: boolean;
  dry_run: boolean;
  total_trades: number;
  winning_trades: number;
  win_rate: number;
  total_pnl: number;
  daily_trades: number;
  daily_pnl: number;
  unrealized_pnl: number;
  combined_pnl: number;
  active_positions: number;
  max_positions: number;
}

export interface GinieAutopilotStatus {
  stats: GinieAutopilotStats;
  config: GinieAutopilotConfig;
  positions: GiniePosition[];
  trade_history: GinieTradeResult[];
  available_balance?: number;
  wallet_balance?: number;
}

export interface GinieCircuitBreakerStatus {
  enabled: boolean;
  can_trade: boolean;
  block_reason: string;
  state: string;
  hourly_loss: number;
  daily_loss: number;
  consecutive_losses: number;
  trades_last_minute: number;
  daily_trades: number;
  trip_reason: string;
  last_trip_time: string;
  max_loss_per_hour: number;
  max_daily_loss: number;
  max_consecutive: number;
  cooldown_minutes: number;
}

// ==================== GINIE AI TRADER TYPES ====================

export type GinieTradingMode = 'scalp' | 'swing' | 'position';
export type GinieScanStatus = 'SCALP-READY' | 'SWING-READY' | 'POSITION-READY' | 'HEDGE-REQUIRED' | 'AVOID';
export type GinieRecommendation = 'EXECUTE' | 'WAIT' | 'SKIP';

export interface LiquidityCheck {
  volume_24h: number;
  volume_usd: number;
  bid_ask_spread: number;
  spread_percent: number;
  slippage_risk: string;
  order_book_depth: number;
  liquidity_score: number;
  passed_scalp: boolean;
  passed_swing: boolean;
}

export interface VolatilityProfile {
  atr_14: number;
  atr_percent: number;
  avg_atr_20: number;
  atr_ratio: number;
  bb_width: number;
  bb_width_percent: number;
  volatility_7d: number;
  volatility_30d: number;
  regime: string;
  volatility_score: number;
}

export interface TrendHealth {
  adx_value: number;
  adx_strength: string;
  is_trending: boolean;
  is_ranging: boolean;
  trend_direction: string;
  ema_20_distance: number;
  ema_50_distance: number;
  ema_200_distance: number;
  mtf_alignment: boolean;
  aligned_tfs: string[];
  trend_age: number;
  trend_maturity: string;
  trend_score: number;
}

export interface MarketStructure {
  pattern: string;
  key_resistances: number[];
  key_supports: number[];
  nearest_resistance: number;
  nearest_support: number;
  breakout_potential: number;
  breakdown_potential: number;
  consolidation_days: number;
  structure_score: number;
}

export interface CorrelationCheck {
  btc_correlation: number;
  eth_correlation: number;
  sector_correlation: number;
  independent_capable: boolean;
  correlation_score: number;
}

export interface GinieCoinScan {
  symbol: string;
  timestamp: string;
  status: GinieScanStatus;
  liquidity: LiquidityCheck;
  volatility: VolatilityProfile;
  trend: TrendHealth;
  structure: MarketStructure;
  correlation: CorrelationCheck;
  score: number;
  trade_ready: boolean;
  reason: string;
}

export interface GinieSignal {
  name: string;
  description: string;
  status: string;
  value: number;
  threshold: number;
  weight: number;
  met: boolean;
}

export interface GinieSignalSet {
  mode: GinieTradingMode;
  primary_timeframe: string;
  confirm_timeframe: string;
  primary_signals: GinieSignal[];
  primary_met: number;
  primary_required: number;
  primary_passed: boolean;
  secondary_signals: GinieSignal[];
  secondary_met: number;
  signal_strength: string;
  strength_score: number;
  direction: string;
}

export interface GinieTakeProfitLevel {
  level: number;
  price: number;
  percent: number;
  gain_pct: number;
  status: string;
}

export interface GinieTradeExecution {
  action: string;
  entry_low: number;
  entry_high: number;
  position_pct: number;
  risk_usd: number;
  leverage: number;
  take_profits: GinieTakeProfitLevel[];
  stop_loss: number;
  stop_loss_pct: number;
  risk_reward: number;
  trailing_stop: number;
}

export interface GinieHedgeRecommendation {
  required: boolean;
  hedge_type: string;
  hedge_size: number;
  entry_rule: string;
  exit_rule: string;
  reason: string;
}

export interface GinieDecisionReport {
  symbol: string;
  timestamp: string;
  scan_status: GinieScanStatus;
  selected_mode: GinieTradingMode;
  market_conditions: {
    trend: string;
    adx: number;
    volatility: string;
    atr: number;
    volume: string;
    btc_correlation: number;
    sentiment: string;
    sentiment_value: number;
  };
  signal_analysis: GinieSignalSet;
  trade_execution: GinieTradeExecution;
  hedge: GinieHedgeRecommendation;
  invalidation_conditions: string[];
  re_evaluate_conditions: string[];
  next_review: string;
  confidence_score: number;
  recommendation: GinieRecommendation;
  recommendation_note: string;
}

export interface GinieConfig {
  enabled: boolean;
  scalp_adx_max: number;
  swing_adx_min: number;
  swing_adx_max: number;
  position_adx_min: number;
  high_volatility_ratio: number;
  min_scalp_volume: number;
  min_swing_volume: number;
  max_bid_ask_spread: number;
  scalp_signals_required: number;
  swing_signals_required: number;
  position_signals_required: number;
  max_daily_drawdown: number;
  max_weekly_drawdown: number;
  max_monthly_drawdown: number;
  max_scalp_positions: number;
  max_swing_positions: number;
  max_position_positions: number;
  auto_override_enabled: boolean;
  scalp_monitor_interval: number;
  swing_monitor_interval: number;
  position_monitor_interval: number;
}

export interface GinieStatus {
  enabled: boolean;
  active_mode: GinieTradingMode;
  active_positions: number;
  max_positions: number;
  last_scan_time: string;
  last_decision_time: string;
  daily_pnl: number;
  daily_trades: number;
  win_rate: number;
  config: GinieConfig;
  recent_decisions: GinieDecisionReport[];
  watched_symbols: string[];
  scanned_symbols: number;
}

// Investigate Status Types
export interface InvestigateStatus {
  trading_status: 'active' | 'blocked' | 'stopped';
  block_reasons: string[];
  last_decision_time: string;
  active_positions: number;
  recent_rejections: RejectionSummary[];
  rejection_stats: RejectionStats;
  modes: Record<string, ModeStatus>;
  constraints: ConstraintStatus;
  signal_health: SignalHealthStatus;
  alerts: AlertItem[];
  api_health: Record<string, string>;
}

export interface RejectionSummary {
  timestamp: string;
  symbol: string;
  action: string;
  reason: string;
}

export interface RejectionStats {
  total_decisions: number;
  total_rejections: number;
  rejection_rate: number;
  common_reasons: Record<string, number>;
  avg_confidence: number;
}

export interface ModeStatus {
  enabled: boolean;
  status: string;
  details: string;
}

export interface ConstraintStatus {
  usd_allocation: ConstraintItem;
  daily_trades: ConstraintItem;
  daily_pnl: ConstraintItem;
  hourly_loss: ConstraintItem;
  consecutive_loss: ConstraintItem;
}

export interface ConstraintItem {
  current: number;
  max: number;
  percent: number;
  status: 'ok' | 'warning' | 'critical';
}

export interface SignalHealthStatus {
  ml_predictor: ComponentHealth;
  llm_analyzer: ComponentHealth;
  sentiment_analyzer: ComponentHealth;
  avg_confidence: number;
  confluence_rate: number;
}

export interface ComponentHealth {
  available: boolean;
  last_used: string;
  success_rate: number;
}

export interface AlertItem {
  level: 'info' | 'warning' | 'critical';
  type: string;
  message: string;
}

// ==================== COIN CLASSIFICATION TYPES ====================

export type VolatilityClass = 'stable' | 'medium' | 'high';
export type MarketCapClass = 'blue_chip' | 'large_cap' | 'mid_small';
export type MomentumClass = 'gainer' | 'neutral' | 'loser';

export interface CoinClassification {
  symbol: string;
  last_price: number;
  volatility: VolatilityClass;
  volatility_atr: number;
  market_cap: MarketCapClass;
  momentum: MomentumClass;
  momentum_24h_pct: number;
  volume_24h: number;
  quote_volume_24h: number;
  risk_score: number;
  opportunity_score: number;
  enabled: boolean;
  last_updated: string;
}

export interface CategoryAllocation {
  enabled: boolean;
  allocation_percent: number;
  max_positions: number;
}

export interface CoinClassificationSettings {
  volatility_stable_max: number;
  volatility_medium_max: number;
  momentum_gainer_min: number;
  momentum_loser_max: number;
  min_volume_24h: number;
  atr_period: number;
  atr_timeframe: string;
  refresh_interval_secs: number;
  volatility_allocations: Record<VolatilityClass, CategoryAllocation>;
  market_cap_allocations: Record<MarketCapClass, CategoryAllocation>;
  momentum_allocations: Record<MomentumClass, CategoryAllocation>;
}

export interface ClassificationSummary {
  total_symbols: number;
  enabled_symbols: number;
  by_volatility: Record<VolatilityClass, string[]>;
  by_market_cap: Record<MarketCapClass, string[]>;
  by_momentum: Record<MomentumClass, string[]>;
  top_gainers: CoinClassification[];
  top_losers: CoinClassification[];
  top_volume: CoinClassification[];
  last_updated: string;
}

export interface CoinPreferenceInfo {
  symbol: string;
  enabled: boolean;
  priority: number;
  volatility?: string;
  market_cap?: string;
  momentum?: string;
  atr_percent?: number;
  change_24h?: number;
}

export interface EligibleCoin {
  symbol: string;
  priority: number;
  volatility: string;
  market_cap: string;
  momentum: string;
  atr_percent: number;
  change_24h: number;
}

// ==================== TRADING STYLE TYPES ====================

export interface TradingStyleConfig {
  name: string;
  default_leverage: number;
  max_leverage: number;
  sl_atr_multiple: number;
  tp_atr_multiple: number;
  min_hold_time: number;
  max_hold_time: number;
  allow_averaging: boolean;
  max_avg_entries: number;
  allow_hedging: boolean;
  min_confidence: number;
  required_confluence: number;
  trend_timeframes: string[];
  signal_timeframe: string;
  entry_timeframe: string;
}

// ==================== HEDGING TYPES ====================

export type HedgeTrigger = 'price_drop' | 'unrealized_loss' | 'ai_recommendation' | 'manual';

export interface HedgeEvent {
  timestamp: string;
  trigger: HedgeTrigger;
  action: 'open' | 'close' | 'partial_close';
  hedge_percent: number;
  hedge_price: number;
  quantity: number;
  pnl?: number;
  reason: string;
}

export interface HedgePositionInfo {
  symbol: string;
  side: string;
  entry_price: number;
  quantity: number;
  leverage: number;
  trigger_reason: HedgeTrigger;
  trigger_price: number;
  open_time: string;
  current_pnl: number;
  current_pnl_pct: number;
}

export interface HedgingStatus {
  enabled: boolean;
  hedge_mode_enabled: boolean;
  active_hedges: Array<{
    symbol: string;
    side: string;
    entry_price: number;
    quantity: number;
    trigger: HedgeTrigger;
    trigger_price: number;
    current_pnl: number;
    current_pnl_pct: number;
    open_time: string;
  }>;
  active_count: number;
  max_simultaneous: number;
  price_drop_trigger: number;
  loss_trigger: number;
  ai_enabled: boolean;
  default_percent: number;
  profit_take_pct: number;
  close_on_recovery: number;
}

// ==================== GINIE DIAGNOSTICS TYPES ====================

export interface GinieDiagnostics {
  timestamp: string;
  autopilot_running: boolean;
  is_live_mode: boolean;
  can_trade: boolean;
  can_trade_reason: string;
  circuit_breaker: CBDiagnostics;
  positions: PositionDiagnostics;
  scanning: ScanDiagnostics;
  signals: SignalDiagnostics;
  profit_booking: ProfitDiagnostics;
  blocked_coins: BlockedCoinInfo[] | null;
  llm_status: LLMDiagnostics;
  issues: DiagnosticIssue[];
}

export interface CBDiagnostics {
  enabled: boolean;
  state: string;
  hourly_loss: number;
  hourly_loss_limit: number;
  daily_loss: number;
  daily_loss_limit: number;
  consecutive_losses: number;
  cooldown_remaining: string;
}

export interface PositionDiagnostics {
  open_count: number;
  max_allowed: number;
  slots_available: number;
  total_unrealized_pnl: number;
}

export interface ScanDiagnostics {
  last_scan_time: string;
  seconds_since_last_scan: number;
  symbols_in_watchlist: number;
  symbols_scanned_last_cycle: number;
  scalp_enabled: boolean;
  swing_enabled: boolean;
  position_enabled: boolean;
}

export interface SignalDiagnostics {
  total_generated: number;
  executed: number;
  rejected: number;
  execution_rate_pct: number;
  top_rejection_reasons: Record<string, number>;
}

export interface ProfitDiagnostics {
  positions_with_pending_tp: number;
  tp_hits_last_hour: number;
  partial_closes_last_hour: number;
  failed_closes_last_hour: number;
  trailing_active_count: number;
}

export interface BlockedCoinInfo {
  symbol: string;
  block_reason: string;
  block_time: string;
  loss_amount: number;
  loss_roi: number;
  consec_losses: number;
  auto_unblock: string;
  block_count: number;
  manual_only: boolean;
}

export interface LLMDiagnostics {
  connected: boolean;
  provider: string;
  last_call_time: string;
  coin_list_cached: boolean;
  coin_list_age: string;
  disabled_symbols: string[];
}

export interface DiagnosticIssue {
  severity: 'critical' | 'warning' | 'info';
  category: string;
  message: string;
  suggestion: string;
}

// ==================== GINIE SIGNAL LOG TYPES ====================

export interface GinieSignalLog {
  id: string;
  symbol: string;
  timestamp: string;
  direction: string;
  mode: string;
  confidence: number;
  status: 'executed' | 'rejected' | 'pending';
  rejection_reason?: string;
  entry_price: number;
  stop_loss: number;
  take_profit_1: number;
  leverage: number;
  risk_reward: number;
  trend: string;
  volatility: string;
  atr_percent: number;
  signal_names: string[];
  primary_met: number;
  primary_required: number;
  current_price: number;
}

export interface GinieSignalStats {
  total: number;
  executed: number;
  rejected: number;
  pending: number;
  execution_rate: number;
  rejection_reasons: Record<string, number>;
}

// ==================== GINIE SL UPDATE HISTORY TYPES ====================

export interface GinieSLUpdateRecord {
  timestamp: string;
  old_sl: number;
  new_sl: number;
  current_price: number;
  status: 'applied' | 'rejected';
  rejection_rule?: string;
  source: string;
  llm_confidence?: number;
}

export interface GinieSLUpdateHistory {
  symbol: string;
  total_attempts: number;
  applied: number;
  rejected: number;
  updates: GinieSLUpdateRecord[];
}

export interface GinieSLStats {
  total_attempts: number;
  applied: number;
  rejected: number;
  approval_rate: number;
  rejections_by_rule: Record<string, number>;
  symbols_tracked: number;
}

export interface GinieLLMSLStatus {
  kill_switch_active: Record<string, boolean>;
  bad_call_counts: Record<string, number>;
  disabled_symbols: string[];
  threshold: number;
}

// ==================== AUTO MODE TYPES (LLM-DRIVEN TRADING) ====================

export interface AutoModeConfig {
  enabled: boolean;
  max_positions: number;
  max_leverage: number;
  max_position_size: number;
  max_total_usd: number;
  allow_averaging: boolean;
  max_averages: number;
  min_hold_minutes: number;
  quick_profit_mode: boolean;
  min_profit_for_exit: number;
}

// ==================== SYMBOL PERFORMANCE TYPES ====================

export type SymbolPerformanceCategory = 'best' | 'good' | 'neutral' | 'poor' | 'worst' | 'blacklist';

export interface SymbolPerformanceSettings {
  symbol: string;
  category: SymbolPerformanceCategory;
  min_confidence: number;
  max_position_usd: number;
  size_multiplier: number;
  leverage_override: number;
  enabled: boolean;
  notes: string;
  total_trades: number;
  winning_trades: number;
  total_pnl: number;
  win_rate: number;
  avg_pnl: number;
  last_updated: string;
}

export interface SymbolPerformanceReport {
  symbol: string;
  category: SymbolPerformanceCategory;
  total_trades: number;
  winning_trades: number;
  losing_trades: number;
  total_pnl: number;
  win_rate: number;
  avg_pnl: number;
  avg_win: number;
  avg_loss: number;
  min_confidence: number;
  max_position_usd: number;
  size_multiplier: number;
  enabled: boolean;
}

// ==================== MODE CONFIGURATION TYPES (Story 2.7) ====================

export interface ModeTimeframeConfig {
  trend_timeframe: string;    // e.g., "5m", "15m", "1h", "4h"
  entry_timeframe: string;    // e.g., "1m", "5m", "15m", "1h"
  analysis_timeframe: string; // e.g., "1m", "15m", "4h", "1d"
}

export interface ModeConfidenceConfig {
  min_confidence: number;    // Minimum to enter (e.g., 50, 60, 65, 75)
  high_confidence: number;   // Threshold for size multiplier (e.g., 70, 75, 80, 85)
  ultra_confidence: number;  // Threshold for max size (e.g., 85, 88, 90, 92)
}

export interface ModeSizeConfig {
  base_size_usd: number;       // Base position size
  max_size_usd: number;        // Max with multiplier
  max_positions: number;       // Max concurrent positions
  leverage: number;            // Default leverage
  size_multiplier_lo: number;  // Min multiplier
  size_multiplier_hi: number;  // Max multiplier on high confidence
}

export interface ModeCircuitBreakerConfig {
  max_loss_per_hour: number;       // e.g., $20, $40, $80, $150
  max_loss_per_day: number;        // e.g., $50, $100, $200, $400
  max_consecutive_losses: number;  // e.g., 3, 5, 7, 10
  cooldown_minutes: number;        // e.g., 15, 30, 60, 120
  max_trades_per_minute: number;   // e.g., 5, 3, 2, 1
  max_trades_per_hour: number;     // e.g., 30, 20, 10, 5
  max_trades_per_day: number;      // e.g., 100, 50, 20, 10
  win_rate_check_after: number;    // Trades before evaluation
  min_win_rate: number;            // Threshold %
}

export interface ModeSLTPConfig {
  stop_loss_percent: number;          // Default SL %
  take_profit_percent: number;        // Default TP %
  trailing_stop_enabled: boolean;     // Enable trailing
  trailing_stop_percent: number;      // Trail distance
  trailing_stop_activation: number;   // Activate at profit %
  trailing_activation_price: number;  // Activate at specific price (0 = use profit %)
  max_hold_duration: string;          // Force exit after (e.g., "3s", "4h", "3d")
  use_single_tp: boolean;             // true = 100% at TP, false = multi-level
  // ROI-based SL/TP
  use_roi_based_sltp: boolean;        // Use ROI % instead of price %
  roi_stop_loss_percent: number;      // Close at this ROI % loss (e.g., -10)
  roi_take_profit_percent: number;    // Close at this ROI % profit (e.g., 25)
  // Margin configuration
  margin_type: string;                // "CROSS" or "ISOLATED"
  isolated_margin_percent: number;    // Margin % for isolated mode (10-100)
}

export interface ModeFullConfig {
  mode_name: string;  // "ultra_fast", "scalp", "swing", "position"
  enabled: boolean;   // Enable this mode

  // Sub-configurations
  timeframe?: ModeTimeframeConfig;
  confidence?: ModeConfidenceConfig;
  size?: ModeSizeConfig;
  circuit_breaker?: ModeCircuitBreakerConfig;
  sltp?: ModeSLTPConfig;
}

export interface ModeConfigsResponse {
  success: boolean;
  mode_configs: Record<string, ModeFullConfig>;
  valid_modes: string[];
}

// ==================== LLM & ADAPTIVE AI TYPES (Story 2.8) ====================

// LLM Configuration
export interface LLMConfig {
  enabled: boolean;
  provider: string;
  model: string;
  fallback_provider: string;
  fallback_model: string;
  timeout_ms: number;
  retry_count: number;
  cache_duration_sec: number;
}

export interface ModeLLMSettings {
  llm_enabled: boolean;
  llm_weight: number;
  skip_on_timeout: boolean;
  min_llm_confidence: number;
  block_on_disagreement: boolean;
  cache_enabled: boolean;
}

export interface AdaptiveAIConfig {
  enabled: boolean;
  learning_window_trades: number;
  learning_window_hours: number;
  auto_adjust_enabled: boolean;
  max_auto_adjustment_percent: number;
  require_approval: boolean;
  min_trades_for_learning: number;
  store_decision_context: boolean;
}

export interface LLMConfigResponse {
  success: boolean;
  llm_config: LLMConfig;
  mode_settings: Record<string, ModeLLMSettings>;
  adaptive_config: AdaptiveAIConfig;
}

export interface AdaptiveRecommendation {
  id: string;
  created_at: string;
  type: string;
  mode: string;
  current_value: any;
  suggested_value: any;
  reason: string;
  expected_improvement: string;
  applied_at?: string;
  dismissed: boolean;
}

export interface ModeStatistics {
  mode: string;
  total_trades: number;
  wins: number;
  losses: number;
  win_rate: number;
  avg_win_percent: number;
  avg_loss_percent: number;
  total_profit: number;
  agreement_win_rate: number;
  disagreement_win_rate: number;
}

export interface AdaptiveRecommendationsResponse {
  success: boolean;
  recommendations: AdaptiveRecommendation[];
  statistics: Record<string, ModeStatistics>;
  last_analysis: string;
  total_outcomes_analyzed: number;
}

export interface DecisionContext {
  technical_confidence: number;
  llm_confidence: number;
  final_confidence: number;
  technical_direction: string;
  llm_direction: string;
  agreement: boolean;
  llm_reasoning: string;
  llm_key_factors: string[];
}

export interface TradeWithAI {
  trade_id: string;
  symbol: string;
  mode: string;
  direction: string;
  entry_time: string;
  exit_time: string;
  pnl_percent: number;
  outcome: string;
  decision_context?: DecisionContext;
}

export interface LLMCallDiagnostics {
  total_calls: number;
  cache_hits: number;
  cache_misses: number;
  avg_latency_ms: number;
  error_rate: number;
  calls_by_provider: Record<string, number>;
  recent_errors: string[];
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

// Safe toFixed helper that handles null, undefined, strings, and NaN
export function safeToFixed(value: number | string | null | undefined, precision = 2): string {
  const num = typeof value === 'string' ? parseFloat(value) : value;
  if (num === null || num === undefined || isNaN(num)) {
    return (0).toFixed(precision);
  }
  return Number(num).toFixed(precision);
}

export function formatQuantity(quantity: number | string | null | undefined, precision = 4): string {
  return safeToFixed(quantity, precision);
}

export function formatPrice(price: number | string | null | undefined, precision = 2): string {
  const num = typeof price === 'string' ? parseFloat(price) : price;
  if (num === null || num === undefined || isNaN(num)) {
    return (0).toLocaleString('en-US', {
      minimumFractionDigits: precision,
      maximumFractionDigits: precision,
    });
  }
  return num.toLocaleString('en-US', {
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
  return `${sign}${safeToFixed(num, 2)}%`;
}

export function formatFundingRate(rate: number | string | null | undefined): string {
  const num = typeof rate === 'string' ? parseFloat(rate) : rate;
  if (num === null || num === undefined || isNaN(num)) {
    return '0.0000%';
  }
  return `${safeToFixed(num * 100, 4)}%`;
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
