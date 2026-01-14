import axios, { AxiosInstance } from 'axios';
import { apiService } from './api';
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

// Token storage keys
const ACCESS_TOKEN_KEY = 'access_token';
const REFRESH_TOKEN_KEY = 'refresh_token';

class FuturesAPIService {
  private client: AxiosInstance;
  private isRefreshing = false;
  private failedQueue: Array<{
    resolve: (token: string) => void;
    reject: (error: unknown) => void;
  }> = [];

  constructor() {
    this.client = axios.create({
      baseURL: '/api/futures',
      timeout: 30000, // Increased for LLM calls and slow operations
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Request interceptor to add auth token
    this.client.interceptors.request.use(
      (config) => {
        const token = localStorage.getItem(ACCESS_TOKEN_KEY);
        if (token) {
          config.headers = config.headers || {};
          config.headers.Authorization = `Bearer ${token}`;
        }
        return config;
      },
      (error) => {
        return Promise.reject(error);
      }
    );

    // Response interceptor for error handling and token refresh
    this.client.interceptors.response.use(
      (response) => response,
      async (error) => {
        const originalRequest = error.config;

        // Handle 401 Unauthorized - attempt token refresh
        if (error.response?.status === 401 && !originalRequest._retry) {
          if (this.isRefreshing) {
            // Queue the request while refreshing
            return new Promise((resolve, reject) => {
              this.failedQueue.push({ resolve, reject });
            }).then((token) => {
              originalRequest.headers.Authorization = `Bearer ${token}`;
              return this.client(originalRequest);
            });
          }

          originalRequest._retry = true;
          this.isRefreshing = true;

          try {
            const refreshToken = localStorage.getItem(REFRESH_TOKEN_KEY);
            if (!refreshToken) {
              throw new Error('No refresh token');
            }

            const response = await axios.post('/api/auth/refresh', {
              refresh_token: refreshToken,
            });

            const { access_token, refresh_token: newRefreshToken } = response.data;
            localStorage.setItem(ACCESS_TOKEN_KEY, access_token);
            localStorage.setItem(REFRESH_TOKEN_KEY, newRefreshToken);

            // Process queued requests
            this.failedQueue.forEach((request) => request.resolve(access_token));
            this.failedQueue = [];

            originalRequest.headers.Authorization = `Bearer ${access_token}`;
            return this.client(originalRequest);
          } catch (refreshError) {
            // Refresh failed, clear tokens and redirect to login
            localStorage.removeItem(ACCESS_TOKEN_KEY);
            localStorage.removeItem(REFRESH_TOKEN_KEY);
            localStorage.removeItem('user');

            // Process queued requests with error
            this.failedQueue.forEach((request) => request.reject(refreshError));
            this.failedQueue = [];

            // Redirect to login if not already there
            if (window.location.pathname !== '/login') {
              window.location.href = '/login';
            }
            return Promise.reject(refreshError);
          } finally {
            this.isRefreshing = false;
          }
        }

        // Don't log auth errors (401/403) - they're expected when not logged in
        if (error.response?.status !== 401 && error.response?.status !== 403) {
          console.error('Futures API Error:', error);
        }
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

  async getModeCircuitBreakerStatus(): Promise<ModeCircuitBreakerStatusResponse> {
    const { data } = await this.client.get('/ginie/mode-circuit-breaker-status');
    return data;
  }

  async resetModeCircuitBreaker(mode: string): Promise<{
    success: boolean;
    message: string;
  }> {
    const { data } = await this.client.post(`/ginie/mode-circuit-breaker/${mode}/reset`);
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

  // ==================== GINIE PENDING ORDERS ====================

  /**
   * Get pending limit orders that are waiting to be filled
   * Returns unfilled LIMIT orders with timeout information
   */
  async getGiniePendingOrders(): Promise<PendingOrdersResponse> {
    const { data } = await this.client.get('/ginie/pending-orders');
    return data;
  }

  // ==================== GINIE TRADE CONDITIONS ====================

  /**
   * Get detailed status of all pre-trade conditions
   * Returns a checklist of conditions that must pass before trading
   */
  async getGinieTradeConditions(): Promise<TradeConditionsResponse> {
    const { data } = await this.client.get('/ginie/trade-conditions');
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

  // ==================== INSTANCE CONTROL (Story 9.6) ====================

  async getInstanceStatus(): Promise<{
    instance_id: string;        // "dev" or "prod"
    is_active: boolean;         // This instance's status
    active_instance: string;    // Which instance is active
    other_alive: boolean;       // Is other instance running
    last_heartbeat: string;     // Last heartbeat time (ISO string)
    can_take_control: boolean;  // Can this instance take over
  }> {
    const { data } = await this.client.get('/ginie/instance-status');
    return data;
  }

  async takeControl(request: { force?: boolean } = {}): Promise<{
    success: boolean;
    message: string;
    wait_seconds?: number;
  }> {
    const { data } = await this.client.post('/ginie/take-control', request);
    return data;
  }

  async releaseControl(): Promise<{
    success: boolean;
    message: string;
  }> {
    const { data } = await this.client.post('/ginie/release-control');
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

  async refreshSymbolPerformance(): Promise<{
    success: boolean;
    symbols_updated: number;
    report: SymbolPerformanceReport[];
    by_category: Record<string, SymbolPerformanceReport[]>;
    total_symbols: number;
  }> {
    const { data } = await this.client.post('/autopilot/symbols/refresh-performance');
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

  // ==================== SYMBOL BLOCKING (Daily Worst Performer Blocking) ====================

  async getBlockedSymbols(): Promise<{
    success: boolean;
    blocked_symbols: Array<{
      symbol: string;
      blocked_until: string;
      reason: string;
      remaining: string;
    }>;
    total: number;
  }> {
    const { data } = await this.client.get('/autopilot/symbols/blocked');
    return data;
  }

  async blockSymbolForDay(symbol: string, reason?: string): Promise<{
    success: boolean;
    symbol: string;
    blocked_until: string;
    reason: string;
  }> {
    const { data } = await this.client.post(`/autopilot/symbols/${symbol}/block-day`, { reason });
    return data;
  }

  async unblockSymbol(symbol: string): Promise<{ success: boolean; symbol: string; message: string }> {
    const { data } = await this.client.post(`/autopilot/symbols/${symbol}/unblock`);
    return data;
  }

  async getSymbolBlockStatus(symbol: string): Promise<{
    symbol: string;
    is_blocked: boolean;
    blocked_until?: string;
    reason?: string;
    remaining?: string;
  }> {
    const { data } = await this.client.get(`/autopilot/symbols/${symbol}/block-status`);
    return data;
  }

  async autoBlockWorstPerformers(): Promise<{
    success: boolean;
    blocked_symbols: string[];
    count: number;
    message: string;
  }> {
    const { data } = await this.client.post('/autopilot/symbols/auto-block-worst');
    return data;
  }

  async clearExpiredBlocks(): Promise<{ success: boolean; cleared: number; message: string }> {
    const { data } = await this.client.post('/autopilot/symbols/clear-expired-blocks');
    return data;
  }

  // ==================== MORNING AUTO-BLOCK ====================

  async getMorningAutoBlockConfig(): Promise<{
    success: boolean;
    enabled: boolean;
    hour_utc: number;
    minute_utc: number;
    next_run: string;
    time_until: string;
  }> {
    const { data } = await this.client.get('/autopilot/morning-auto-block/config');
    return data;
  }

  async updateMorningAutoBlockConfig(config: {
    enabled?: boolean;
    hour_utc?: number;
    minute_utc?: number;
  }): Promise<{
    success: boolean;
    enabled: boolean;
    hour_utc: number;
    minute_utc: number;
    next_run: string;
    time_until: string;
    message: string;
  }> {
    const { data } = await this.client.post('/autopilot/morning-auto-block/config', config);
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

  async toggleModeEnabled(mode: string, enabled: boolean): Promise<{
    success: boolean;
    mode: string;
    enabled: boolean;
  }> {
    const { data } = await this.client.post(`/ginie/mode-config/${mode}/toggle`, { enabled });
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

  // ==================== PROTECTION STATUS ====================

  async getProtectionStatus(): Promise<ProtectionStatusResponse> {
    const { data } = await this.client.get('/ginie/protection/status');
    return data;
  }

  // ==================== TRADE LIFECYCLE EVENTS ====================

  /**
   * Get all lifecycle events for a specific trade
   */
  async getTradeLifecycleEvents(tradeId: number): Promise<TradeLifecycleEventsResponse> {
    const { data } = await this.client.get(`/trades/${tradeId}/events`);
    return data;
  }

  /**
   * Get an aggregated summary of a trade's lifecycle
   */
  async getTradeLifecycleSummary(tradeId: number): Promise<TradeLifecycleSummaryResponse> {
    const { data } = await this.client.get(`/trades/${tradeId}/lifecycle-summary`);
    return data;
  }

  /**
   * Get lifecycle events of a specific type for a trade
   */
  async getTradeLifecycleEventsByType(tradeId: number, eventType: string): Promise<TradeLifecycleEventsResponse> {
    const { data } = await this.client.get(`/trades/${tradeId}/events/${eventType}`);
    return data;
  }

  /**
   * Get recent lifecycle events across all trades
   */
  async getRecentTradeLifecycleEvents(limit = 50): Promise<RecentTradeEventsResponse> {
    const { data } = await this.client.get('/trade-events/recent', { params: { limit } });
    return data;
  }

  /**
   * Get the number of SL revisions for a trade
   */
  async getTradeSLRevisionCount(tradeId: number): Promise<SLRevisionCountResponse> {
    const { data } = await this.client.get(`/trades/${tradeId}/sl-revisions`);
    return data;
  }

  // ==================== SCAN SOURCE CONFIG ====================

  /**
   * Get scan source configuration for current user
   */
  async getScanSourceConfig(): Promise<ScanSourceConfig> {
    const { data } = await this.client.get('/ginie/scan-config');
    return data.config;
  }

  /**
   * Update scan source configuration
   */
  async updateScanSourceConfig(config: Partial<ScanSourceConfig>): Promise<{ success: boolean }> {
    const { data } = await this.client.post('/ginie/scan-config', config);
    return data;
  }

  /**
   * Get saved coins list
   */
  async getSavedCoins(): Promise<{ coins: string[]; count: number; enabled: boolean }> {
    const { data } = await this.client.get('/ginie/saved-coins');
    return { coins: data.saved_coins || [], count: data.count || 0, enabled: data.enabled || false };
  }

  /**
   * Update saved coins list
   */
  async updateSavedCoins(coins: string[]): Promise<{ success: boolean }> {
    const { data } = await this.client.post('/ginie/saved-coins', { coins });
    return data;
  }

  /**
   * Get scan preview - shows which coins will be scanned with current config
   */
  async getScanPreview(): Promise<ScanPreview> {
    const { data } = await this.client.get('/ginie/scan-preview');
    return data;
  }

  // ==================== SCALP RE-ENTRY CONFIG ====================

  /**
   * Get scalp re-entry mode configuration
   */
  async getScalpReentryConfig(): Promise<ScalpReentryConfig> {
    const { data } = await this.client.get('/ginie/scalp-reentry-config');
    return data.config;
  }

  /**
   * Update scalp re-entry mode configuration
   */
  async updateScalpReentryConfig(config: Partial<ScalpReentryConfig>): Promise<{ success: boolean; config: ScalpReentryConfig }> {
    const { data } = await this.client.post('/ginie/scalp-reentry-config', config);
    return data;
  }

  /**
   * Toggle scalp re-entry mode on/off
   */
  async toggleScalpReentry(enabled: boolean): Promise<{ success: boolean; enabled: boolean; message: string }> {
    const { data } = await this.client.post('/ginie/scalp-reentry/toggle', { enabled });
    return data;
  }

  // ==================== SCALP RE-ENTRY MONITOR ====================

  /**
   * Get all positions in scalp_reentry mode with enhanced status
   */
  async getScalpReentryPositions(): Promise<ScalpReentryPositionsResponse> {
    const { data } = await this.client.get('/ginie/scalp-reentry/positions');
    return data;
  }

  /**
   * Get detailed status for a single scalp_reentry position
   */
  async getScalpReentryPositionStatus(symbol: string): Promise<ScalpReentryPositionDetailResponse> {
    const { data } = await this.client.get(`/ginie/scalp-reentry/positions/${symbol}`);
    return data;
  }

  // ==================== HEDGE MODE API METHODS ====================

  /**
   * Get hedge mode configuration
   */
  async getHedgeModeConfig(): Promise<HedgeModeConfig> {
    const { data } = await this.client.get('/ginie/hedge-config');
    return data.config;
  }

  /**
   * Update hedge mode configuration
   */
  async updateHedgeModeConfig(config: Partial<HedgeModeConfig>): Promise<{ success: boolean; config: HedgeModeConfig; message: string }> {
    const { data } = await this.client.post('/ginie/hedge-config', config);
    return data;
  }

  /**
   * Toggle hedge mode on/off
   */
  async toggleHedgeMode(enabled: boolean): Promise<{ success: boolean; enabled: boolean; message: string }> {
    const { data } = await this.client.post('/ginie/hedge-mode/toggle', { enabled });
    return data;
  }

  /**
   * Get all positions with active hedge mode state
   */
  async getHedgeModePositions(): Promise<HedgeModePositionsResponse> {
    const { data } = await this.client.get('/ginie/hedge-mode/positions');
    return data;
  }

  // ==================== RESET TO DEFAULTS APIs (Story 4.17) ====================

  /**
   * Load default configuration for a specific mode (preview or apply)
   */
  async loadModeDefaults(mode: string, preview: boolean = true): Promise<ConfigResetPreview | ConfigResetResult> {
    const { data } = await this.client.post(`/ginie/modes/${mode}/load-defaults${preview ? '?preview=true' : ''}`);
    return data;
  }

  /**
   * Load default configuration for circuit breaker (preview or apply)
   */
  async loadCircuitBreakerDefaults(preview: boolean = true): Promise<ConfigResetPreview | ConfigResetResult> {
    const { data } = await this.client.post(`/ginie/circuit-breaker/load-defaults${preview ? '?preview=true' : ''}`);
    return data;
  }

  /**
   * Load default configuration for LLM config (preview or apply)
   */
  async loadLLMConfigDefaults(preview: boolean = true): Promise<ConfigResetPreview | ConfigResetResult> {
    const { data } = await this.client.post(`/ginie/llm-config/load-defaults${preview ? '?preview=true' : ''}`);
    return data;
  }

  /**
   * Load default configuration for capital allocation (preview or apply)
   */
  async loadCapitalAllocationDefaults(preview: boolean = true): Promise<ConfigResetPreview | ConfigResetResult> {
    const { data } = await this.client.post(`/ginie/capital-allocation/load-defaults${preview ? '?preview=true' : ''}`);
    return data;
  }

  /**
   * Load default configuration for all modes (preview or apply)
   */
  async loadAllModesDefaults(preview: boolean = true): Promise<ConfigResetPreview | ConfigResetResult> {
    const { data } = await this.client.post(`/ginie/modes/load-defaults${preview ? '?preview=true' : ''}`);
    return data;
  }

  /**
   * Get differences between current mode config and defaults
   */
  async getModeDiff(mode: string): Promise<ConfigResetPreview> {
    const { data } = await this.client.get(`/settings/diff/modes/${mode}`);
    return data;
  }

  /**
   * Load default configuration for hedge mode (preview or apply)
   */
  async loadHedgeDefaults(preview: boolean = true): Promise<ConfigResetPreview | ConfigResetResult> {
    const { data } = await this.client.post(`/ginie/hedge-mode/load-defaults${preview ? '?preview=true' : ''}`);
    return data;
  }

  /**
   * Load default configuration for scalp reentry mode (preview or apply)
   */
  async loadScalpReentryDefaults(preview: boolean = true): Promise<ConfigResetPreview | ConfigResetResult> {
    const { data } = await this.client.post(`/ginie/modes/scalp_reentry/load-defaults${preview ? '?preview=true' : ''}`);
    return data;
  }

  /**
   * Load default configuration for safety settings (preview or apply)
   * Story 9.4: Per-mode safety controls (rate limits, profit monitoring, win-rate monitoring)
   */
  async loadSafetySettingsDefaults(preview: boolean = true): Promise<ConfigResetPreview | ConfigResetResult> {
    const { data } = await this.client.post(`/ginie/safety-settings/load-defaults${preview ? '?preview=true' : ''}`);
    return data;
  }

  /**
   * Get all default settings from default-settings.json (Story 9.4)
   * Returns all sections for display in ResetSettings UI
   */
  async getAllDefaultSettings(): Promise<{ success: boolean; defaults: Record<string, unknown> }> {
    const { data } = await this.client.get('/ginie/default-settings');
    return data;
  }
}

// ==================== HEDGE MODE TYPES ====================

export interface HedgeModeConfig {
  hedge_mode_enabled: boolean;
  trigger_on_profit_tp: boolean;
  trigger_on_loss_tp: boolean;
  dca_on_loss: boolean;
  max_position_multiple: number;
  combined_roi_exit_pct: number;
  wide_sl_atr_multiplier: number;
  disable_ai_sl: boolean;
  rally_exit_enabled: boolean;
  rally_adx_threshold: number;
  rally_sustained_move_pct: number;
  neg_tp1_percent: number;
  neg_tp1_add_percent: number;
  neg_tp2_percent: number;
  neg_tp2_add_percent: number;
  neg_tp3_percent: number;
  neg_tp3_add_percent: number;
}

export interface HedgeModePositionData {
  symbol: string;
  mode: string; // Trading mode that owns this position (scalp, swing, position, etc.)
  original_side: string;
  entry_price: number;
  current_price: number;
  original: {
    remaining_qty: number;
    current_be: number;
    tp_level: number;
    accum_profit: number;
    unrealized_pnl: number;
  };
  hedge: {
    active: boolean;
    side: string;
    entry_price: number;
    remaining_qty: number;
    current_be: number;
    tp_level: number;
    accum_profit: number;
    unrealized_pnl: number;
    trigger_type: string;
  };
  combined: {
    roi_percent: number;
    realized_pnl: number;
    unrealized_pnl: number;
    total_pnl: number;
  };
  dca: {
    enabled: boolean;
    additions_count: number;
    total_qty: number;
    neg_tp_triggered: number;
  };
  wide_sl: {
    price: number;
    atr_multiplier: number;
    ai_blocked: boolean;
  };
  debug_log?: string[];
}

export interface HedgeModePositionsResponse {
  success: boolean;
  count: number;
  positions: HedgeModePositionData[];
}

// ==================== RESET TO DEFAULTS TYPES (Story 4.17) ====================

export interface SettingDiff {
  path: string;
  current: any;
  default: any;
  risk_level: 'high' | 'medium' | 'low';
  impact?: string;
  recommendation?: string;
}

export interface ConfigResetPreview {
  preview: boolean;
  config_type: string;
  all_match: boolean;
  total_changes: number;
  differences: SettingDiff[];
  // Admin-specific fields (when is_admin is true, default_value contains all defaults)
  is_admin?: boolean;
  default_value?: any;
  message?: string;
}

export interface ConfigResetResult {
  success: boolean;
  config_type: string;
  changes_applied: number;
  message: string;
}

// ==================== SCAN SOURCE CONFIG TYPES ====================

export interface ScanSourceConfig {
  id?: string;
  user_id?: string;
  max_coins: number;
  use_saved_coins: boolean;
  saved_coins: string[];
  use_llm_list: boolean;
  use_market_movers: boolean;
  mover_gainers: boolean;
  mover_losers: boolean;
  mover_volume: boolean;
  mover_volatility: boolean;
  mover_new_listings: boolean;
  gainers_limit: number;
  losers_limit: number;
  volume_limit: number;
  volatility_limit: number;
  new_listings_limit: number;
}

export interface ScanPreviewCoin {
  symbol: string;
  sources: string[];
}

export interface ScanPreview {
  coins: ScanPreviewCoin[];
  total_count: number;
  max_coins: number;
}

// ==================== SCALP RE-ENTRY CONFIG TYPES ====================

export interface ScalpReentryConfig {
  // Master toggle
  enabled: boolean;

  // TP Levels configuration
  tp1_percent: number;      // 0.3 (0.3% profit)
  tp1_sell_percent: number; // 30 (sell 30%)
  tp2_percent: number;      // 0.6 (0.6% profit)
  tp2_sell_percent: number; // 50 (sell 50% of remaining)
  tp3_percent: number;      // 1.0 (1% profit)
  tp3_sell_percent: number; // 80 (sell 80%, keep 20%)

  // Re-entry configuration
  reentry_percent: number;      // 80 (buy back 80% of sold qty)
  reentry_price_buffer: number; // 0.05 (0.05% buffer from breakeven)
  max_reentry_attempts: number; // 3 max attempts before skipping
  reentry_timeout_sec: number;  // 300 (5 min timeout)

  // Final portion (20% remaining after 1%)
  final_trailing_percent: number; // 5.0 (5% trailing from peak)
  final_hold_min_percent: number; // 20 (minimum 20% to hold)

  // Dynamic SL after 1% reached
  dynamic_sl_max_loss_pct: number; // 40 (can lose 40% of profit max)
  dynamic_sl_protect_pct: number;  // 60 (protect 60% of profit)
  dynamic_sl_update_int: number;   // 30 (update every 30s)

  // AI Configuration
  use_ai_decisions: boolean;    // Enable AI for re-entry decisions
  ai_min_confidence: number;    // 0.65 minimum confidence
  ai_tp_optimization: boolean;  // Use AI to optimize TP timing
  ai_dynamic_sl: boolean;       // Use AI for dynamic SL decisions

  // Multi-agent configuration
  use_multi_agent: boolean;         // Enable multi-agent system
  enable_sentiment_agent: boolean;  // Enable sentiment analysis
  enable_risk_agent: boolean;       // Enable risk management agent
  enable_tp_agent: boolean;         // Enable TP timing agent

  // Adaptive learning
  enable_adaptive_learning: boolean;
  adaptive_window_trades: number;      // 20 trades window
  adaptive_min_trades: number;         // 10 trades before adjusting
  adaptive_max_reentry_adjust: number; // Max 20% adjustment

  // Risk limits
  max_cycles_per_position: number; // 10 max cycles
  max_daily_reentries: number;     // 50 max per day
  min_position_size_usd: number;   // $10 minimum
}

// ==================== SCALP RE-ENTRY MONITOR TYPES ====================

export interface ScalpReentryCycleInfo {
  cycle_number: number;
  tp_level: number;
  state: string;           // NONE, WAITING, EXECUTING, COMPLETED, FAILED, SKIPPED

  // Sell info
  sell_price: number;
  sell_quantity: number;
  sell_pnl: number;
  sell_time: string;

  // Reentry info
  reentry_target: number;
  reentry_filled: number;
  reentry_price: number;
  reentry_time: string;

  // Outcome
  outcome: string;         // profit, loss, skipped, pending
  outcome_pnl: number;
  outcome_reason: string;

  // AI Decision
  ai_reasoning?: string;
  ai_confidence?: number;
}

export interface ScalpReentryPositionStatus {
  symbol: string;
  side: string;
  mode: string;
  entry_price: number;
  current_price: number;
  unrealized_pnl: number;
  unrealized_pnl_pct: number;

  // Scalp Re-entry specific fields
  scalp_reentry_active: boolean;
  tp_level_unlocked: number;     // 0, 1, 2, or 3
  next_tp_level: number;         // Next target TP
  next_tp_percent: number;       // Target % for next TP
  next_tp_blocked: boolean;      // Waiting for reentry

  // Current cycle info
  current_cycle_num: number;
  current_cycle_state: string;   // WAITING, EXECUTING, COMPLETED, etc
  reentry_target_price: number;  // Breakeven target
  distance_to_reentry: number;   // % distance to reentry price

  // Accumulated stats
  accumulated_profit: number;
  total_cycles: number;
  successful_reentries: number;
  skipped_reentries: number;

  // Final portion tracking
  final_portion_active: boolean;
  final_portion_qty: number;
  final_trailing_peak: number;
  dynamic_sl_active: boolean;
  dynamic_sl_price: number;

  // Cycle history
  cycles: ScalpReentryCycleInfo[];

  // Debug info
  last_update: string;

  // Hedge mode status
  hedge_mode_active: boolean;
  hedge_side?: string;
}

export interface ScalpReentryPositionsSummary {
  total_positions: number;
  total_accumulated_pnl: number;
  total_cycles: number;
  total_reentries: number;
  config_enabled: boolean;
}

export interface ScalpReentryPositionsResponse {
  success: boolean;
  positions: ScalpReentryPositionStatus[];
  count: number;
  summary: ScalpReentryPositionsSummary;
}

export interface ScalpReentryPositionDetailResponse {
  success: boolean;
  symbol: string;
  side: string;
  mode: string;
  entry_price: number;
  current_price: number;
  unrealized_pnl: number;
  unrealized_pnl_pct: number;
  original_qty: number;
  remaining_qty: number;

  scalp_reentry: {
    enabled: boolean;
    tp_level_unlocked: number;
    next_tp_blocked: boolean;
    current_cycle: number;
    accumulated_profit: number;
    original_entry_price: number;
    current_breakeven: number;
    remaining_quantity: number;

    // Dynamic SL
    dynamic_sl_active: boolean;
    dynamic_sl_price: number;
    protected_profit: number;
    max_allowable_loss: number;

    // Final portion
    final_portion_active: boolean;
    final_portion_qty: number;
    final_trailing_peak: number;
    final_trailing_percent: number;
    final_trailing_active: boolean;

    // Stats
    total_cycles_completed: number;
    total_reentries: number;
    successful_reentries: number;
    skipped_reentries: number;
    total_cycle_pnl: number;

    // Timestamps
    started_at: string;
    last_update: string;

    // Debug log
    debug_log: string[];
  };

  tp_levels: {
    tp1: { percent: number; sell_percent: number; hit: boolean };
    tp2: { percent: number; sell_percent: number; hit: boolean };
    tp3: { percent: number; sell_percent: number; hit: boolean };
  };

  cycles: ScalpReentryCycleInfo[];
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

// Stuck position alert for positions that need manual intervention
export interface StuckPositionAlert {
  symbol: string;
  side: string;
  mode: string;
  reason: string;
  alerted_at: string;
  remaining_quantity: number;
  entry_price: number;
}

export interface GinieAutopilotStatus {
  stats: GinieAutopilotStats;
  config: GinieAutopilotConfig;
  positions: GiniePosition[];
  trade_history: GinieTradeResult[];
  available_balance?: number;
  wallet_balance?: number;
  stuck_positions?: StuckPositionAlert[];
  has_stuck_positions?: boolean;
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

// FVG (Fair Value Gap) types
export interface FairValueGap {
  type: 'bullish' | 'bearish';
  top_price: number;
  bottom_price: number;
  mid_price: number;
  gap_size: number;
  gap_percent: number;
  candle_index: number;
  timestamp: string;
  filled: boolean;
  tested: boolean;
  strength: 'weak' | 'medium' | 'strong';
}

export interface FVGAnalysis {
  bullish_fvgs: FairValueGap[];
  bearish_fvgs: FairValueGap[];
  nearest_bullish: FairValueGap | null;
  nearest_bearish: FairValueGap | null;
  total_unfilled: number;
  in_fvg_zone: boolean;
  fvg_zone_type: string;
  fvg_confluence: boolean;
}

// Order Block types
export interface OrderBlock {
  type: 'bullish' | 'bearish';
  high_price: number;
  low_price: number;
  mid_price: number;
  open_price: number;
  close_price: number;
  volume: number;
  candle_index: number;
  timestamp: string;
  mitigated: boolean;
  tested: boolean;
  test_count: number;
  strength: 'weak' | 'medium' | 'strong';
  move_percent: number;
}

export interface OrderBlockAnalysis {
  bullish_obs: OrderBlock[];
  bearish_obs: OrderBlock[];
  nearest_bullish: OrderBlock | null;
  nearest_bearish: OrderBlock | null;
  total_unmitigated: number;
  in_ob_zone: boolean;
  ob_zone_type: string;
  ob_confluence: boolean;
}

// Chart Pattern Types
export interface SwingPoint {
  price: number;
  index: number;
  timestamp: string;
  volume?: number;
}

export interface Trendline {
  start_price: number;
  end_price: number;
  start_index: number;
  end_index: number;
  slope: number;
  touch_points: number[];
}

export interface HeadAndShouldersPattern {
  type: 'head_and_shoulders' | 'inverse_head_and_shoulders';
  left_shoulder: SwingPoint;
  head: SwingPoint;
  right_shoulder: SwingPoint;
  neckline_left: SwingPoint;
  neckline_right: SwingPoint;
  neckline_slope: number;
  neckline_price: number;
  target_price: number;
  pattern_height: number;
  pattern_percent: number;
  symmetry_score: number;
  volume_confirmed: boolean;
  completed: boolean;
  candle_index: number;
  timestamp: string;
  strength: 'weak' | 'moderate' | 'strong';
}

export interface DoubleTopBottomPattern {
  type: 'double_top' | 'double_bottom';
  first_peak: SwingPoint;
  second_peak: SwingPoint;
  neckline: SwingPoint;
  neckline_price: number;
  target_price: number;
  pattern_height: number;
  pattern_percent: number;
  peak_difference: number;
  bars_between: number;
  volume_confirmed: boolean;
  status: 'forming' | 'confirmed' | 'invalid';
  completed: boolean;
  candle_index: number;
  timestamp: string;
  strength: 'weak' | 'moderate' | 'strong';
}

export interface TrianglePattern {
  type: 'ascending' | 'descending' | 'symmetrical';
  upper_trendline: Trendline;
  lower_trendline: Trendline;
  apex_price: number;
  apex_index: number;
  pattern_start: number;
  pattern_width: number;
  base_height: number;
  base_percent: number;
  current_height: number;
  contraction_pct: number;
  volume_decline: boolean;
  breakout_bias: 'up' | 'down' | 'neutral';
  breakout_target: number;
  touches_upper: number;
  touches_lower: number;
  completed: boolean;
  breakout_dir: string;
  candle_index: number;
  timestamp: string;
  strength: 'weak' | 'moderate' | 'strong';
}

export interface WedgePattern {
  type: 'rising_wedge' | 'falling_wedge';
  upper_trendline: Trendline;
  lower_trendline: Trendline;
  apex_price: number;
  apex_index: number;
  pattern_start: number;
  pattern_width: number;
  slope_ratio: number;
  base_height: number;
  base_percent: number;
  breakout_bias: 'up' | 'down';
  breakout_target: number;
  touches_upper: number;
  touches_lower: number;
  volume_decline: boolean;
  completed: boolean;
  breakout_dir: string;
  candle_index: number;
  timestamp: string;
  strength: 'weak' | 'moderate' | 'strong';
}

export interface FlagPennantPattern {
  type: 'bull_flag' | 'bear_flag' | 'bull_pennant' | 'bear_pennant';
  direction: 'bullish' | 'bearish';
  flagpole_start: SwingPoint;
  flagpole_end: SwingPoint;
  flagpole_height: number;
  flagpole_percent: number;
  flagpole_bars: number;
  flagpole_volume: number;
  consolidation_type: 'channel' | 'triangle';
  consolidation_high: number;
  consolidation_low: number;
  consolidation_bars: number;
  retracement_pct: number;
  consolidation_vol: number;
  breakout_level: number;
  target_price: number;
  stop_loss: number;
  completed: boolean;
  volume_confirmed: boolean;
  candle_index: number;
  timestamp: string;
  strength: 'weak' | 'moderate' | 'strong';
}

export interface PatternSummary {
  type: string;
  direction: 'bullish' | 'bearish';
  strength: 'weak' | 'moderate' | 'strong';
  target_price: number;
  breakout_level: number;
  completion_pct: number;
}

export interface ChartPatternAnalysis {
  head_and_shoulders: HeadAndShouldersPattern[];
  double_tops_bottoms: DoubleTopBottomPattern[];
  triangles: TrianglePattern[];
  wedges: WedgePattern[];
  flags_pennants: FlagPennantPattern[];
  active_pattern: PatternSummary | null;
  pattern_score: number;
  pattern_bias: 'bullish' | 'bearish' | 'neutral';
  pattern_confluence: boolean;
  has_bullish_pattern: boolean;
  has_bearish_pattern: boolean;
  near_breakout: boolean;
  estimated_target: number;
  total_patterns: number;
  reversal_patterns: number;
  continuation_patterns: number;
  consolidation_patterns: number;
}

// Combined Price Action Analysis
export interface PriceActionAnalysis {
  fvg: FVGAnalysis;
  order_blocks: OrderBlockAnalysis;
  chart_patterns?: ChartPatternAnalysis;
  has_bullish_setup: boolean;
  has_bearish_setup: boolean;
  setup_quality: 'weak' | 'moderate' | 'good' | 'excellent';
  confluence_score: number;
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
  price_action?: PriceActionAnalysis;
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

// Rejection tracking types - shows WHY a coin with good score isn't being traded
export interface RejectionTracker {
  is_blocked: boolean;
  block_reason: string;
  all_reasons: string[];
  trend_divergence?: TrendDivergenceRejection;
  signal_strength?: SignalStrengthRejection;
  liquidity?: LiquidityRejection;
  adx_strength?: ADXStrengthRejection;
  counter_trend?: CounterTrendRejection;
  confidence?: ConfidenceRejection;
  position_limit?: PositionLimitRejection;
  insufficient_funds?: InsufficientFundsRejection;
  circuit_breaker?: CircuitBreakerRejection;
  scan_quality?: ScanQualityRejection;
}

export interface TrendDivergenceRejection {
  blocked: boolean;
  scan_timeframe: string;
  scan_trend: string;
  decision_timeframe: string;
  decision_trend: string;
  severity: string;
  reason: string;
}

export interface SignalStrengthRejection {
  blocked: boolean;
  signals_met: number;
  signals_required: number;
  failed_signals: string[];
  reason: string;
}

export interface LiquidityRejection {
  blocked: boolean;
  volume_24h: number;
  required_volume: number;
  bid_ask_spread: number;
  max_spread: number;
  reason: string;
}

export interface ADXStrengthRejection {
  blocked: boolean;
  adx_value: number;
  threshold: number;
  penalty: number;
  reason: string;
}

export interface CounterTrendRejection {
  blocked: boolean;
  signal_direction: string;
  trend_direction: string;
  missing_requirements: string[];
  reason: string;
}

export interface ConfidenceRejection {
  blocked: boolean;
  confidence_score: number;
  execute_threshold: number;
  wait_threshold: number;
  reason: string;
}

export interface PositionLimitRejection {
  blocked: boolean;
  current_positions: number;
  max_positions: number;
  mode: string;
  reason: string;
}

export interface InsufficientFundsRejection {
  blocked: boolean;
  required_usd: number;
  available_usd: number;
  position_size_usd: number;
  reason: string;
}

export interface CircuitBreakerRejection {
  blocked: boolean;
  trip_reason: string;
  cooldown_mins: number;
  resume_at: string;
  reason: string;
}

export interface ScanQualityRejection {
  blocked: boolean;
  scan_score: number;
  min_score: number;
  trade_ready: boolean;
  scan_status: string;
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
  rejection_tracking?: RejectionTracker;  // Shows WHY a coin isn't being traded
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

// ==================== PENDING LIMIT ORDERS TYPES ====================

export interface PendingOrderInfo {
  order_id: number;
  symbol: string;
  direction: string;  // "LONG" or "SHORT"
  side: string;       // "BUY" or "SELL"
  entry_price: number;
  quantity: number;
  placed_at: string;
  timeout_at: string;
  seconds_left: number;
  source: string;
  mode: string;
  status: string;     // "pending" or "expired"
}

export interface PendingOrdersResponse {
  pending_orders: PendingOrderInfo[];
  count: number;
}

// ==================== TRADE CONDITIONS TYPES ====================

export interface TradeCondition {
  name: string;
  passed: boolean;
  detail: string;
}

export interface TradeConditionsResponse {
  conditions: TradeCondition[];
  all_passed: boolean;
  blocking_count: number;
  timestamp: string;
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
  // Additional sizing parameters
  safety_margin?: number;                 // 0.90
  min_balance_usd?: number;               // 25.0
  min_position_size_usd?: number;         // 10.0
  risk_multiplier_conservative?: number;  // 0.6
  risk_multiplier_moderate?: number;      // 0.8
  risk_multiplier_aggressive?: number;    // 1.0
  confidence_multiplier_base?: number;    // 0.5
  confidence_multiplier_scale?: number;   // 0.7
  // Auto AI/LLM sizing
  auto_size_enabled?: boolean;            // Use AI/LLM to determine position size
  auto_size_min_cover_fee?: number;       // Minimum size to cover fees (default: $15)
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

// Per-mode circuit breaker status
export interface ModeCircuitBreakerStatusItem {
  mode: string;
  is_paused: boolean;
  pause_reason: string;
  cooldown_remaining: number;  // seconds
  hourly_loss: number;
  daily_loss: number;
  hourly_pnl: number;
  daily_pnl: number;
  consecutive_losses: number;
  consecutive_wins: number;
  trades_last_minute: number;
  trades_last_hour: number;
  trades_today: number;
  win_count: number;
  loss_count: number;
  win_rate: number;
}

export interface ModeCircuitBreakerSummary {
  total_modes: number;
  tripped_modes: string[];
  tripped_count: number;
  all_clear: boolean;
}

export interface ModeCircuitBreakerStatusResponse {
  success: boolean;
  circuit_breaker_configs: Record<string, ModeCircuitBreakerConfig>;
  mode_status: Record<string, ModeCircuitBreakerStatusItem> & {
    summary?: ModeCircuitBreakerSummary;
  };
  global_status: GinieCircuitBreakerStatus;
  valid_modes: string[];
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
  // ATR/LLM blending parameters
  atr_sl_multiplier?: number;         // Base SL × ATR
  atr_tp_multiplier?: number;         // Base TP × ATR
  atr_sl_min?: number;                // Min SL % bound
  atr_sl_max?: number;                // Max SL % bound
  atr_tp_min?: number;                // Min TP % bound
  atr_tp_max?: number;                // Max TP % bound
  llm_weight?: number;                // LLM blend weight (0.7)
  atr_weight?: number;                // ATR blend weight (0.3)
  tp_gain_levels?: number[];          // Multi-TP levels per mode
  // Auto AI/LLM SL/TP management
  auto_sltp_enabled?: boolean;        // Use AI/LLM to determine SL/TP levels
  auto_trailing_enabled?: boolean;    // Use AI/LLM for trailing stop management
  min_profit_to_trail_pct?: number;   // Min profit % before trailing (covers fees)
  min_sl_distance_from_zero?: number; // Min SL distance from entry (avoid near-zero closes)
}

export interface ModeRiskConfig {
  risk_level: string;                    // "conservative" | "moderate" | "aggressive"
  risk_multiplier_conservative: number;  // 0.6
  risk_multiplier_moderate: number;      // 0.8
  risk_multiplier_aggressive: number;    // 1.0
  max_drawdown_percent: number;          // 5.0 for scalp, 15.0 for position
  daily_loss_limit_percent: number;      // 3.0
  weekly_loss_limit_percent: number;     // 10.0
  max_portfolio_risk_percent: number;    // 2.0 per trade
  correlation_penalty: number;           // 0.5 reduce size for correlated positions
}

export interface ModeTrendDivergenceConfig {
  enabled: boolean;                  // Enable divergence checking
  block_on_divergence: boolean;      // Block trades on divergence
  timeframes_to_check: string[];     // ["5m", "15m", "1h", "4h"]
  min_aligned_timeframes: number;    // 2 = at least 2 must agree
  adx_threshold: number;             // 25.0 = min trend strength
  counter_trend_penalty: number;     // 0.5 = reduce confidence by 50%
  allow_counter_trend: boolean;      // false for swing, true for scalp
}

export interface ModeFundingRateConfig {
  enabled: boolean;
  max_funding_rate: number;          // 0.001 = 0.1%
  block_time_minutes: number;        // 30
  exit_time_minutes: number;         // 10
  fee_threshold_percent: number;     // 0.3 = 30%
  extreme_funding_rate: number;      // 0.003 = 0.3%
  high_rate_reduction: number;       // 0.5 = 50%
  elevated_rate_reduction: number;   // 0.75 = 75%
}

export interface PositionAveragingConfig {
  allow_averaging?: boolean;           // Enable position averaging
  average_up_profit_percent?: number;  // Add when position is in profit by X%
  average_down_loss_percent?: number;  // Add when position is in loss by X%
  add_size_percent?: number;           // Size of add as % of original position
  max_averages?: number;               // Maximum number of averaging entries
  min_confidence_for_average?: number; // Minimum confidence to average
  use_llm_for_averaging?: boolean;     // Use AI to decide averaging
}

// [Story 9.9] Position Optimization Config - embedded in each mode
export interface PositionOptimizationConfig {
  enabled?: boolean;
  // Progressive Profit Taking
  tp1_percent?: number;
  tp1_sell_percent?: number;
  tp2_percent?: number;
  tp2_sell_percent?: number;
  tp3_percent?: number;
  tp3_sell_percent?: number;
  // Re-entry settings
  reentry_percent?: number;
  reentry_price_buffer?: number;
  max_reentry_attempts?: number;
  reentry_timeout_sec?: number;
  reentry_require_trend_confirmation?: boolean;
  reentry_min_adx?: number;
  // Final trailing
  final_trailing_percent?: number;
  final_hold_min_percent?: number;
  // Dynamic SL
  dynamic_sl_max_loss_pct?: number;
  dynamic_sl_protect_pct?: number;
  dynamic_sl_update_int?: number;
  // AI settings
  use_ai_decisions?: boolean;
  ai_min_confidence?: number;
  ai_tp_optimization?: boolean;
  ai_dynamic_sl?: boolean;
  // Multi-agent settings
  use_multi_agent?: boolean;
  enable_sentiment_agent?: boolean;
  enable_risk_agent?: boolean;
  enable_tp_agent?: boolean;
  // Adaptive learning
  enable_adaptive_learning?: boolean;
  adaptive_window_trades?: number;
  adaptive_min_trades?: number;
  adaptive_max_reentry_adjust?: number;
  // Limits
  max_cycles_per_position?: number;
  max_daily_reentries?: number;
  min_position_size_usd?: number;
  stop_loss_percent?: number;
  // Hedging - Same fields as HedgeModeConfig
  hedge_mode_enabled?: boolean;
  allow_hedge_chains?: boolean;
  max_hedge_chain_depth?: number;
  profit_protection_enabled?: boolean;
  profit_protection_percent?: number;
  max_loss_of_earned_profit?: number;
  // Hedge Triggers
  trigger_on_profit_tp?: boolean;
  trigger_on_loss_tp?: boolean;
  dca_on_loss?: boolean;
  max_position_multiple?: number;
  combined_roi_exit_pct?: number;
  wide_sl_atr_multiplier?: number;
  disable_ai_sl?: boolean;
  // Rally Exit
  rally_exit_enabled?: boolean;
  rally_adx_threshold?: number;
  rally_sustained_move_pct?: number;
  // Negative TP Levels (hedge DCA levels)
  neg_tp1_percent?: number;
  neg_tp1_add_percent?: number;
  neg_tp2_percent?: number;
  neg_tp2_add_percent?: number;
  neg_tp3_percent?: number;
  neg_tp3_add_percent?: number;
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
  risk?: ModeRiskConfig;
  trend_divergence?: ModeTrendDivergenceConfig;
  funding_rate?: ModeFundingRateConfig;
  averaging?: PositionAveragingConfig;
  position_optimization?: PositionOptimizationConfig; // [Story 9.9] Position optimization settings per mode
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

// ==================== TRADE LIFECYCLE EVENT TYPES ====================

export type TradeLifecycleEventType =
  | 'position_opened'
  | 'sltp_placed'
  | 'sl_revised'
  | 'moved_to_breakeven'
  | 'tp_hit'
  | 'trailing_activated'
  | 'trailing_updated'
  | 'position_closed'
  | 'external_close'
  | 'sl_hit';

export type TradeLifecycleEventSource = 'ginie' | 'trailing' | 'manual' | 'external' | 'system';

export interface TradeLifecycleEvent {
  id: number;
  futures_trade_id?: number;
  event_type: TradeLifecycleEventType;
  event_subtype?: string;
  timestamp: string;
  trigger_price?: number;
  old_value?: number;
  new_value?: number;
  mode?: string;
  source: TradeLifecycleEventSource;
  sl_revision_count?: number;
  tp_level?: number;
  quantity_closed?: number;
  pnl_realized?: number;
  pnl_percent?: number;
  reason?: string;
  conditions_met?: Record<string, unknown>;
  details?: Record<string, unknown>;
}

export interface TradeLifecycleSummary {
  trade_id: number;
  symbol: string;
  mode: string;
  entry_time: string;
  entry_price: number;
  exit_time?: string;
  exit_price?: number;
  total_events: number;
  sl_revisions: number;
  tp_hits: number;
  trailing_updates: number;
  final_pnl?: number;
  final_pnl_percent?: number;
  close_reason?: string;
  events_by_type: Record<TradeLifecycleEventType, number>;
}

export interface TradeLifecycleEventsResponse {
  success: boolean;
  trade_id: number;
  events: TradeLifecycleEvent[];
  count: number;
  event_type?: string;
}

export interface TradeLifecycleSummaryResponse {
  success: boolean;
  summary: TradeLifecycleSummary;
}

export interface RecentTradeEventsResponse {
  success: boolean;
  events: TradeLifecycleEvent[];
  count: number;
  limit: number;
}

export interface SLRevisionCountResponse {
  success: boolean;
  trade_id: number;
  sl_revisions: number;
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

// ==================== BULLETPROOF PROTECTION STATUS ====================

export interface ProtectionPositionStatus {
  symbol: string;
  side: string;
  entry_time: string;
  protection_state: string;
  sl_verified: boolean;
  tp_verified: boolean;
  failure_count: number;
  heal_attempts: number;
  last_failure: string;
  time_in_state: string;
  is_protected: boolean;
}

export interface ProtectionSummary {
  total: number;
  protected: number;
  unprotected: number;
  healing: number;
  emergency: number;
  health_pct: number;
}

export interface ProtectionStatusResponse {
  success: boolean;
  positions: ProtectionPositionStatus[];
  summary: ProtectionSummary;
  timestamp: string;
}

// Get protection status for all positions
export async function getProtectionStatus(): Promise<ProtectionStatusResponse> {
  return futuresApi.getProtectionStatus();
}

// ==================== RESET DEFAULTS WRAPPER FUNCTIONS ====================

/**
 * Load default configuration for a specific mode (preview or apply)
 */
export async function loadModeDefaults(mode: string, preview: boolean = true): Promise<ConfigResetPreview | ConfigResetResult> {
  return futuresApi.loadModeDefaults(mode, preview);
}

/**
 * Load default configuration for circuit breaker (preview or apply)
 */
export async function loadCircuitBreakerDefaults(preview: boolean = true): Promise<ConfigResetPreview | ConfigResetResult> {
  return futuresApi.loadCircuitBreakerDefaults(preview);
}

/**
 * Load default configuration for LLM config (preview or apply)
 */
export async function loadLLMConfigDefaults(preview: boolean = true): Promise<ConfigResetPreview | ConfigResetResult> {
  return futuresApi.loadLLMConfigDefaults(preview);
}

/**
 * Load default configuration for capital allocation (preview or apply)
 */
export async function loadCapitalAllocationDefaults(preview: boolean = true): Promise<ConfigResetPreview | ConfigResetResult> {
  return futuresApi.loadCapitalAllocationDefaults(preview);
}

/**
 * Load default configuration for all modes (preview or apply)
 */
export async function loadAllModesDefaults(preview: boolean = true): Promise<ConfigResetPreview | ConfigResetResult> {
  return futuresApi.loadAllModesDefaults(preview);
}

/**
 * Get differences between current mode config and defaults
 */
export async function getModeDiff(mode: string): Promise<ConfigResetPreview> {
  return futuresApi.getModeDiff(mode);
}

/**
 * Load default configuration for hedge mode (preview or apply)
 */
export async function loadHedgeDefaults(preview: boolean = true): Promise<ConfigResetPreview | ConfigResetResult> {
  return futuresApi.loadHedgeDefaults(preview);
}

/**
 * Load default configuration for scalp reentry mode (preview or apply)
 */
export async function loadScalpReentryDefaults(preview: boolean = true): Promise<ConfigResetPreview | ConfigResetResult> {
  return futuresApi.loadScalpReentryDefaults(preview);
}

/**
 * Load default configuration for safety settings (preview or apply)
 * Story 9.4: Per-mode safety controls (rate limits, profit monitoring, win-rate monitoring)
 */
export async function loadSafetySettingsDefaults(preview: boolean = true): Promise<ConfigResetPreview | ConfigResetResult> {
  return futuresApi.loadSafetySettingsDefaults(preview);
}

/**
 * Get all default settings from default-settings.json (Story 9.4)
 * Returns all sections for display in ResetSettings UI
 */
export async function getAllDefaultSettings(): Promise<{ success: boolean; defaults: Record<string, unknown> }> {
  return futuresApi.getAllDefaultSettings();
}

// ==================== ADMIN DEFAULTS APIs ====================

/**
 * Admin save defaults response
 */
export interface AdminSaveDefaultsResponse {
  success: boolean;
  config_type: string;
  message: string;
  changes_count: number;
}

/**
 * Admin: Save edited default values directly to default-settings.json
 * Only available to admin users
 */
export async function saveAdminDefaults(
  configType: string,
  editedValues: Record<string, any>
): Promise<AdminSaveDefaultsResponse> {
  const response = await apiService.post<AdminSaveDefaultsResponse>(
    `/admin/defaults/${configType}`,
    { edited_values: editedValues }
  );
  return response.data;
}

/**
 * Get pending limit orders that are waiting to be filled
 */
export async function getGiniePendingOrders(): Promise<PendingOrdersResponse> {
  return futuresApi.getGiniePendingOrders();
}

/**
 * Get detailed status of all pre-trade conditions
 */
export async function getGinieTradeConditions(): Promise<TradeConditionsResponse> {
  return futuresApi.getGinieTradeConditions();
}
