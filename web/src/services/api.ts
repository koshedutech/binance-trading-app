import axios, { AxiosInstance } from 'axios';
import type {
  APIResponse,
  BotStatus,
  Position,
  Trade,
  Order,
  Strategy,
  Signal,
  ScreenerResult,
  TradingMetrics,
  SystemEvent,
  PlaceOrderRequest,
  ScanResult,
  WatchlistItem,
} from '../types';

// Token storage keys
const ACCESS_TOKEN_KEY = 'access_token';
const REFRESH_TOKEN_KEY = 'refresh_token';

class APIService {
  private client: AxiosInstance;
  private isRefreshing = false;
  private failedQueue: Array<{
    resolve: (token: string) => void;
    reject: (error: unknown) => void;
  }> = [];

  constructor() {
    this.client = axios.create({
      baseURL: '/api',
      timeout: 30000, // Increased for slow Binance API calls
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

        // Handle 401 Unauthorized
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

            // Auth endpoints return data directly, not wrapped
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
          console.error('API Error:', error);
        }
        return Promise.reject(error);
      }
    );
  }

  // Generic post method for any API endpoint
  async post<T = unknown>(url: string, data?: unknown): Promise<{ data: T }> {
    const response = await this.client.post<T>(url, data);
    return { data: response.data };
  }

  // Generic get method for any API endpoint
  async get<T = any>(url: string): Promise<{ data: T }> {
    const response = await this.client.get<T>(url);
    return { data: response.data };
  }

  // Generic put method for any API endpoint
  async put<T = unknown>(url: string, data?: unknown): Promise<{ data: T }> {
    const response = await this.client.put<T>(url, data);
    return { data: response.data };
  }

  // Bot endpoints
  async getBotStatus(): Promise<BotStatus> {
    const { data } = await this.client.get<APIResponse<BotStatus>>('/bot/status');
    return data.data!;
  }

  async getBotConfig(): Promise<any> {
    const { data } = await this.client.get<APIResponse<any>>('/bot/config');
    return data.data!;
  }

  // Position endpoints
  async getPositions(): Promise<Position[]> {
    const { data } = await this.client.get<APIResponse<Position[]>>('/positions');
    return data.data || [];
  }

  async getPositionHistory(limit = 50, offset = 0, includeAI = false): Promise<Trade[]> {
    const { data } = await this.client.get<APIResponse<Trade[]>>('/positions/history', {
      params: { limit, offset, include_ai: includeAI },
    });
    return data.data || [];
  }

  async closePosition(symbol: string): Promise<void> {
    await this.client.post(`/positions/${symbol}/close`);
  }

  async closeAllPositions(): Promise<{ message: string; closed: number; total: number; errors?: string[] }> {
    const { data } = await this.client.post<APIResponse<{ message: string; closed: number; total: number; errors?: string[] }>>('/positions/close-all');
    return data.data!;
  }

  // Order endpoints
  async getActiveOrders(): Promise<Order[]> {
    const { data } = await this.client.get<APIResponse<Order[]>>('/orders');
    return data.data || [];
  }

  async getOrderHistory(limit = 50, offset = 0): Promise<Order[]> {
    const { data } = await this.client.get<APIResponse<Order[]>>('/orders/history', {
      params: { limit, offset },
    });
    return data.data || [];
  }

  async placeOrder(request: PlaceOrderRequest): Promise<{ order_id: number; message: string }> {
    const { data } = await this.client.post<
      APIResponse<{ order_id: number; message: string }>
    >('/orders', request);
    return data.data!;
  }

  async cancelOrder(orderId: number): Promise<void> {
    await this.client.delete(`/orders/${orderId}`);
  }

  // Strategy endpoints
  async getStrategies(): Promise<Strategy[]> {
    const { data } = await this.client.get<APIResponse<Strategy[]>>('/strategies');
    return data.data || [];
  }

  async toggleStrategy(name: string, enabled: boolean): Promise<void> {
    await this.client.put(`/strategies/${name}/toggle`, { enabled });
  }

  // Signal endpoints
  async getSignals(limit = 50): Promise<Signal[]> {
    const { data } = await this.client.get<APIResponse<Signal[]>>('/signals', {
      params: { limit },
    });
    return data.data || [];
  }

  // Screener endpoints
  async getScreenerResults(limit = 50): Promise<ScreenerResult[]> {
    const { data } = await this.client.get<APIResponse<ScreenerResult[]>>(
      '/screener/results',
      {
        params: { limit },
      }
    );
    return data.data || [];
  }

  // Metrics endpoints
  async getMetrics(): Promise<TradingMetrics> {
    const { data } = await this.client.get<APIResponse<TradingMetrics>>('/metrics');
    return data.data!;
  }

  // System events
  async getSystemEvents(limit = 100): Promise<SystemEvent[]> {
    const { data } = await this.client.get<APIResponse<SystemEvent[]>>('/events', {
      params: { limit },
    });
    return data.data || [];
  }

  // Health check
  async healthCheck(): Promise<{ status: string; database: string }> {
    const { data} = await this.client.get('/health');
    return data;
  }

  // Pending signal endpoints
  async getPendingSignals(): Promise<any[]> {
    const { data } = await this.client.get<APIResponse<any[]>>('/pending-signals');
    return data.data || [];
  }

  async getPendingSignal(id: number): Promise<any> {
    const { data } = await this.client.get<APIResponse<any>>(`/pending-signals/${id}`);
    return data.data!;
  }

  async getPendingSignalsByStatus(status: 'CONFIRMED' | 'REJECTED', limit = 50): Promise<any[]> {
    const { data } = await this.client.get<APIResponse<any[]>>('/pending-signals', {
      params: { status, limit },
    });
    return data.data || [];
  }

  async archivePendingSignal(id: number): Promise<void> {
    await this.client.post(`/pending-signals/${id}/archive`);
  }

  async deletePendingSignal(id: number): Promise<void> {
    await this.client.delete(`/pending-signals/${id}`);
  }

  async duplicatePendingSignal(id: number): Promise<any> {
    const { data } = await this.client.post<APIResponse<any>>(`/pending-signals/${id}/duplicate`);
    return data.data;
  }

  async confirmPendingSignal(id: number, action: 'CONFIRM' | 'REJECT'): Promise<void> {
    await this.client.post(`/pending-signals/${id}/confirm`, { action });
  }

  // Strategy config endpoints
  async getStrategyConfigs(): Promise<any[]> {
    const { data } = await this.client.get<APIResponse<any[]>>('/strategy-configs');
    return data.data || [];
  }

  async createStrategyConfig(config: any): Promise<any> {
    const { data } = await this.client.post<APIResponse<any>>('/strategy-configs', config);
    return data.data!;
  }

  async updateStrategyConfig(id: number, config: any): Promise<any> {
    const { data } = await this.client.put<APIResponse<any>>(`/strategy-configs/${id}`, config);
    return data.data!;
  }

  async deleteStrategyConfig(id: number): Promise<void> {
    await this.client.delete(`/strategy-configs/${id}`);
  }

  // Binance data endpoints
  async getBinanceSymbols(): Promise<string[]> {
    const { data } = await this.client.get<APIResponse<{ symbols: string[]; count: number }>>('/binance/symbols');
    return data.data?.symbols || [];
  }

  // Strategy Scanner endpoints
  async getScanResults(): Promise<ScanResult> {
    const { data } = await this.client.get<APIResponse<ScanResult>>('/strategy-scanner/scan');
    return data.data!;
  }

  async refreshScan(): Promise<void> {
    await this.client.post('/strategy-scanner/refresh');
  }

  // Watchlist endpoints
  async getWatchlist(): Promise<WatchlistItem[]> {
    const { data } = await this.client.get<APIResponse<WatchlistItem[]>>('/watchlist');
    return data.data || [];
  }

  async addToWatchlist(symbol: string, notes?: string): Promise<void> {
    await this.client.post('/watchlist', { symbol, notes });
  }

  async removeFromWatchlist(symbol: string): Promise<void> {
    await this.client.delete(`/watchlist/${symbol}`);
  }

  // Pattern Scanner endpoints
  async scanPatterns(request: { symbols: string[]; intervals: string[] }): Promise<any[]> {
    const { data } = await this.client.post('/pattern-scanner/scan', request);
    return data || [];
  }

  async getAllSymbols(): Promise<{ symbols: string[]; count: number }> {
    const { data } = await this.client.get('/binance/all-symbols');
    return data;
  }

  // Visual Strategy & Backtest endpoints
  async getKlines(symbol: string, interval: string, limit = 500): Promise<any[]> {
    const { data } = await this.client.get('/binance/klines', {
      params: { symbol, interval, limit },
    });
    return data.data || [];
  }

  async runBacktest(
    strategyConfigId: number,
    request: {
      symbol: string;
      interval: string;
      start_date: string;
      end_date: string;
    }
  ): Promise<any> {
    const { data } = await this.client.post(
      `/strategy-configs/${strategyConfigId}/backtest`,
      request
    );
    return data.data;
  }

  async getBacktestResults(strategyConfigId: number, limit = 10): Promise<any[]> {
    const { data } = await this.client.get(
      `/strategy-configs/${strategyConfigId}/backtest-results`,
      { params: { limit } }
    );
    return data.data || [];
  }

  async getBacktestTrades(backtestResultId: number): Promise<any[]> {
    const { data } = await this.client.get(`/backtest-results/${backtestResultId}/trades`);
    return data.data || [];
  }

  // ==================== Settings & Control Endpoints ====================

  // Trading Mode
  async getTradingMode(): Promise<{
    dry_run: boolean;
    mode: 'paper' | 'live';
    mode_label: string;
    can_switch: boolean;
    switch_error?: string;
  }> {
    const { data } = await this.client.get('/settings/trading-mode');
    return data;
  }

  async setTradingMode(dryRun: boolean): Promise<{
    success: boolean;
    dry_run: boolean;
    mode: string;
    message: string;
  }> {
    const { data } = await this.client.post('/settings/trading-mode', { dry_run: dryRun });
    return data;
  }

  // Wallet Balance
  async getWalletBalance(): Promise<{
    total_balance: number;
    available_balance: number;
    locked_balance: number;
    currency: string;
    is_simulated: boolean;
    assets: Array<{ asset: string; free: number; locked: number }>;
  }> {
    const { data } = await this.client.get('/settings/wallet-balance');
    return data;
  }

  // Autopilot Status & Control
  async getAutopilotStatus(): Promise<{
    available: boolean;
    enabled: boolean;
    running: boolean;
    dry_run: boolean;
    stats?: {
      total_decisions: number;
      approved_decisions: number;
      rejected_decisions: number;
      total_trades: number;
      winning_trades: number;
      losing_trades: number;
      total_pnl: number;
      daily_pnl: number;
      win_rate: number;
    };
    circuit_breaker?: {
      enabled: boolean;
      state: string;
      can_trade: boolean;
      trip_reason: string;
      stats: any;
    };
  }> {
    const { data } = await this.client.get('/settings/autopilot');
    return data;
  }

  async toggleAutopilot(enabled: boolean): Promise<{
    success: boolean;
    enabled: boolean;
    running: boolean;
    message: string;
  }> {
    const { data } = await this.client.post('/settings/autopilot/toggle', { enabled });
    return data;
  }

  async setAutopilotRules(rules: {
    enabled?: boolean;
    max_daily_loss?: number;
    max_consecutive_losses?: number;
    min_confidence?: number;
    cooldown_minutes?: number;
    require_multi_signal?: boolean;
    risk_level?: string;
  }): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post('/settings/autopilot/rules', rules);
    return data;
  }

  // Circuit Breaker
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
    last_trip_time: string;
    config: {
      max_loss_per_hour: number;
      max_daily_loss: number;
      max_consecutive_losses: number;
      cooldown_minutes: number;
      max_trades_per_minute: number;
      max_daily_trades: number;
    };
  }> {
    const { data } = await this.client.get('/settings/circuit-breaker');
    return data;
  }

  async resetCircuitBreaker(): Promise<{ success: boolean; message: string; state: string }> {
    const { data } = await this.client.post('/settings/circuit-breaker/reset');
    return data;
  }

  async updateCircuitBreakerConfig(config: {
    enabled?: boolean;
    max_loss_per_hour?: number;
    max_daily_loss?: number;
    max_consecutive_losses?: number;
    cooldown_minutes?: number;
    max_trades_per_minute?: number;
    max_daily_trades?: number;
  }): Promise<{
    success: boolean;
    message: string;
    config: {
      enabled: boolean;
      max_loss_per_hour: number;
      max_daily_loss: number;
      max_consecutive_losses: number;
      cooldown_minutes: number;
      max_trades_per_minute: number;
      max_daily_trades: number;
    };
  }> {
    const { data } = await this.client.post('/settings/circuit-breaker/config', config);
    return data;
  }

  // ==================== Strategy Performance Endpoints ====================

  async getStrategyPerformance(timeRange: 'today' | 'week' | 'month' | 'all' = 'all'): Promise<{
    success: boolean;
    performances: Array<{
      strategy_name: string;
      symbol: string;
      total_trades: number;
      winning_trades: number;
      losing_trades: number;
      win_rate: number;
      total_pnl: number;
      avg_pnl: number;
      avg_win: number;
      avg_loss: number;
      largest_win: number;
      largest_loss: number;
      profit_factor: number;
      max_drawdown: number;
      expectancy: number;
      risk_reward: number;
      consecutive_wins: number;
      consecutive_losses: number;
      last_trade_time?: string;
      status: 'active' | 'paused' | 'stopped';
      trend: 'up' | 'down' | 'neutral';
      recent_pnl: number[];
    }>;
    time_range: string;
  }> {
    const { data } = await this.client.get('/strategy-performance', {
      params: { range: timeRange },
    });
    return data;
  }

  async getOverallPerformance(): Promise<{
    total_strategies: number;
    active_strategies: number;
    total_trades: number;
    total_pnl: number;
    overall_win_rate: number;
    today_pnl: number;
    week_pnl: number;
    month_pnl: number;
    best_strategy: string;
    worst_strategy: string;
    avg_trades_per_day: number;
    total_days_trading: number;
  }> {
    const { data } = await this.client.get('/strategy-performance/overall');
    return data;
  }

  async getHistoricalSuccessRate(strategyName?: string): Promise<{
    strategy_name: string;
    daily: Array<{
      period: string;
      start_date: string;
      end_date: string;
      trades: number;
      win_rate: number;
      pnl: number;
      profit_factor: number;
    }>;
    weekly: Array<{
      period: string;
      trades: number;
      win_rate: number;
      pnl: number;
      profit_factor: number;
    }>;
    monthly: Array<{
      period: string;
      trades: number;
      win_rate: number;
      pnl: number;
      profit_factor: number;
    }>;
  }> {
    const { data } = await this.client.get('/strategy-performance/historical', {
      params: strategyName ? { strategy: strategyName } : {},
    });
    return data;
  }

  // ==================== User Profile Endpoints ====================

  async updateProfile(profileData: { name?: string; email?: string }): Promise<void> {
    await this.client.put('/user/profile', profileData);
  }

  async changePassword(passwordData: { current_password: string; new_password: string }): Promise<void> {
    await this.client.post('/user/change-password', passwordData);
  }

  // ==================== API Keys Endpoints ====================

  async getAPIKeys(): Promise<Array<{
    id: string;
    exchange: string;
    api_key_last_four: string;
    is_testnet: boolean;
    is_active: boolean;
    created_at: string;
  }>> {
    const { data } = await this.client.get<APIResponse<any[]>>('/user/api-keys');
    return data.data || [];
  }

  async addAPIKey(keyData: {
    api_key: string;
    secret_key: string;
    is_testnet: boolean;
  }): Promise<void> {
    await this.client.post('/user/api-keys', keyData);
  }

  async deleteAPIKey(keyId: string): Promise<void> {
    await this.client.delete(`/user/api-keys/${keyId}`);
  }

  async testAPIKey(keyId: string): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post(`/user/api-keys/${keyId}/test`);
    return data;
  }

  // ==================== AI API Keys Endpoints ====================

  async getAIKeys(): Promise<Array<{
    id: string;
    provider: string;
    key_last_four: string;
    is_active: boolean;
    created_at: string;
  }>> {
    const { data } = await this.client.get<APIResponse<any[]>>('/user/ai-keys');
    return data.data || [];
  }

  async addAIKey(keyData: {
    provider: string;
    api_key: string;
  }): Promise<void> {
    await this.client.post('/user/ai-keys', keyData);
  }

  async deleteAIKey(keyId: string): Promise<void> {
    await this.client.delete(`/user/ai-keys/${keyId}`);
  }

  async testAIKey(keyId: string): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post(`/user/ai-keys/${keyId}/test`);
    return data;
  }

  // ==================== Billing Endpoints ====================

  async getProfitHistory(): Promise<Array<{
    id: string;
    period_start: string;
    period_end: string;
    starting_balance: number;
    ending_balance: number;
    gross_profit: number;
    net_profit: number;
    profit_share_rate: number;
    profit_share_due: number;
    settlement_status: string;
    created_at: string;
  }>> {
    const { data } = await this.client.get<APIResponse<any[]>>('/billing/profit-history');
    return data.data || [];
  }

  async getInvoices(): Promise<Array<{
    id: string;
    amount: number;
    status: string;
    created_at: string;
    pdf_url?: string;
  }>> {
    const { data } = await this.client.get<APIResponse<any[]>>('/billing/invoices');
    return data.data || [];
  }

  async createCheckoutSession(tierId: string): Promise<{ checkout_url: string }> {
    const { data } = await this.client.post('/billing/checkout', { tier: tierId });
    return data;
  }

  async createCustomerPortal(): Promise<{ portal_url: string }> {
    const { data } = await this.client.post('/billing/portal');
    return data;
  }

  // ==================== Health Status ====================

  async getAPIHealthStatus(): Promise<{
    success: boolean;
    healthy: boolean;
    services: {
      binance_spot: { status: string; message: string };
      binance_futures: { status: string; message: string };
      ai_service: { status: string; message: string };
      database: { status: string; message: string };
    };
  }> {
    const { data } = await this.client.get('/health/status');
    return data;
  }

  // ==================== User Utilities ====================

  async getUserIPAddress(): Promise<{
    success: boolean;
    ip_address: string;
    message: string;
  }> {
    const { data } = await this.client.get('/user/ip-address');
    return data;
  }

  async getUserAPIStatus(): Promise<{
    success: boolean;
    healthy: boolean;
    services: {
      binance_spot: { status: string; message: string };
      binance_futures: { status: string; message: string };
      ai_service: { status: string; message: string };
      database: { status: string; message: string };
    };
  }> {
    const { data } = await this.client.get('/user/api-status');
    return data;
  }
}

export const apiService = new APIService();
export const api = apiService; // Alias for auth context
