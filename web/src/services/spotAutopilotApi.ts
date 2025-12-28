import axios, { AxiosInstance } from 'axios';

// Types for Spot Autopilot
export interface SpotAutopilotStatus {
  enabled: boolean;
  running: boolean;
  dry_run: boolean;
  risk_level: string;
  max_positions: number;
  max_usd_per_position: number;
  take_profit_percent: number;
  stop_loss_percent: number;
  min_confidence: number;
  active_positions: number;
  total_trades: number;
  winning_trades: number;
  daily_pnl: number;
  total_pnl: number;
  message?: string;
}

export interface SpotPosition {
  symbol: string;
  quantity: number;
  entry_price: number;
  current_price: number;
  unrealized_pnl: number;
  unrealized_pnl_percent: number;
  entry_time: string;
  take_profit_price?: number;
  stop_loss_price?: number;
  decision_reason?: string;
}

export interface SpotDecision {
  id: string;
  symbol: string;
  action: string; // BUY, SELL, HOLD
  confidence: number;
  reason: string;
  signals: string[];
  timestamp: string;
  executed: boolean;
  execution_result?: string;
}

export interface SpotCircuitBreakerStatus {
  available: boolean;
  enabled: boolean;
  tripped: boolean;
  trip_reason?: string;
  cooldown_until?: string;
  hourly_loss: number;
  daily_loss: number;
  consecutive_losses: number;
  trades_this_minute: number;
  trades_today: number;
  config: {
    max_loss_per_hour: number;
    max_daily_loss: number;
    max_consecutive_losses: number;
    cooldown_minutes: number;
    max_trades_per_minute: number;
    max_daily_trades: number;
  };
  message?: string;
}

export interface SpotCoinPreferences {
  blacklist: string[];
  whitelist: string[];
  use_whitelist: boolean;
  message?: string;
}

export interface SpotProfitStats {
  total_profit: number;
  total_trades: number;
  winning_trades: number;
  losing_trades: number;
  win_rate: number;
  max_usd_per_position: number;
  daily_pnl: number;
}

export interface SpotDecisionStats {
  total_decisions: number;
  buy_decisions: number;
  sell_decisions: number;
  hold_decisions: number;
  executed_trades: number;
  skipped_trades: number;
  message?: string;
}

// Token storage keys
const ACCESS_TOKEN_KEY = 'access_token';
const REFRESH_TOKEN_KEY = 'refresh_token';

class SpotAutopilotAPIService {
  private client: AxiosInstance;
  private isRefreshing = false;
  private failedQueue: Array<{
    resolve: (token: string) => void;
    reject: (error: unknown) => void;
  }> = [];

  constructor() {
    this.client = axios.create({
      baseURL: '/api/spot',
      timeout: 10000, // Reduced from 15s to 10s to prevent timeout race conditions
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

        console.error('Spot Autopilot API Error:', error);
        return Promise.reject(error);
      }
    );
  }

  // ==================== AUTOPILOT STATUS & CONTROL ====================

  async getStatus(): Promise<SpotAutopilotStatus> {
    const { data } = await this.client.get<SpotAutopilotStatus>('/autopilot/status');
    return data;
  }

  async toggle(enabled: boolean, dryRun?: boolean): Promise<{ success: boolean; message: string; status: SpotAutopilotStatus }> {
    const { data } = await this.client.post('/autopilot/toggle', { enabled, dry_run: dryRun });
    return data;
  }

  async setDryRun(dryRun: boolean): Promise<{ success: boolean; message: string; dry_run: boolean; status: SpotAutopilotStatus }> {
    const { data } = await this.client.post('/autopilot/dry-run', { dry_run: dryRun });
    return data;
  }

  async setRiskLevel(riskLevel: string): Promise<{ success: boolean; message: string; risk_level: string; status: SpotAutopilotStatus }> {
    const { data } = await this.client.post('/autopilot/risk-level', { risk_level: riskLevel });
    return data;
  }

  async setAllocation(maxUsdPerPosition: number): Promise<{ success: boolean; message: string; max_usd_per_position: number; status: SpotAutopilotStatus }> {
    const { data } = await this.client.post('/autopilot/allocation', { max_usd_per_position: maxUsdPerPosition });
    return data;
  }

  async setMaxPositions(maxPositions: number): Promise<{ success: boolean; message: string; max_positions: number; status: SpotAutopilotStatus }> {
    const { data } = await this.client.post('/autopilot/max-positions', { max_positions: maxPositions });
    return data;
  }

  async setTPSL(takeProfitPercent: number, stopLossPercent: number): Promise<{ success: boolean; message: string; take_profit_percent: number; stop_loss_percent: number; status: SpotAutopilotStatus }> {
    const { data } = await this.client.post('/autopilot/tpsl', { take_profit_percent: takeProfitPercent, stop_loss_percent: stopLossPercent });
    return data;
  }

  async setMinConfidence(minConfidence: number): Promise<{ success: boolean; message: string; min_confidence: number; status: SpotAutopilotStatus }> {
    const { data } = await this.client.post('/autopilot/min-confidence', { min_confidence: minConfidence });
    return data;
  }

  async getProfitStats(): Promise<SpotProfitStats> {
    const { data } = await this.client.get<SpotProfitStats>('/autopilot/profit-stats');
    return data;
  }

  // ==================== CIRCUIT BREAKER ====================

  async getCircuitBreakerStatus(): Promise<SpotCircuitBreakerStatus> {
    const { data } = await this.client.get<SpotCircuitBreakerStatus>('/circuit-breaker/status');
    return data;
  }

  async resetCircuitBreaker(): Promise<{ success: boolean; message: string; status: SpotCircuitBreakerStatus }> {
    const { data } = await this.client.post('/circuit-breaker/reset');
    return data;
  }

  async toggleCircuitBreaker(enabled: boolean): Promise<{ success: boolean; message: string; status: SpotCircuitBreakerStatus }> {
    const { data } = await this.client.post('/circuit-breaker/toggle', { enabled });
    return data;
  }

  async updateCircuitBreakerConfig(config: {
    max_loss_per_hour?: number;
    max_daily_loss?: number;
    max_consecutive_losses?: number;
    cooldown_minutes?: number;
    max_trades_per_minute?: number;
    max_daily_trades?: number;
  }): Promise<{ success: boolean; message: string; status: SpotCircuitBreakerStatus }> {
    const { data } = await this.client.post('/circuit-breaker/config', config);
    return data;
  }

  // ==================== COIN PREFERENCES ====================

  async getCoinPreferences(): Promise<SpotCoinPreferences> {
    const { data } = await this.client.get<SpotCoinPreferences>('/coin-preferences');
    return data;
  }

  async setCoinPreferences(blacklist: string[], whitelist: string[], useWhitelist: boolean): Promise<{ success: boolean; message: string; preferences: SpotCoinPreferences }> {
    const { data } = await this.client.post('/coin-preferences', { blacklist, whitelist, use_whitelist: useWhitelist });
    return data;
  }

  // ==================== AI DECISIONS ====================

  async getRecentDecisions(): Promise<{ success: boolean; decisions: SpotDecision[]; count: number; message?: string }> {
    const { data } = await this.client.get('/ai-decisions');
    return data;
  }

  async getDecisionStats(): Promise<SpotDecisionStats> {
    const { data } = await this.client.get<SpotDecisionStats>('/ai-decisions/stats');
    return data;
  }

  // ==================== POSITIONS ====================

  async getPositions(): Promise<{ success: boolean; positions: SpotPosition[]; count: number; message?: string }> {
    const { data } = await this.client.get('/positions');
    return data;
  }

  async closePosition(symbol: string): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.post(`/positions/${symbol}/close`);
    return data;
  }

  async closeAllPositions(): Promise<{ success: boolean; message: string; closed: number; errors?: string[] }> {
    const { data } = await this.client.post('/positions/close-all');
    return data;
  }
}

// Export singleton instance
export const spotAutopilotApi = new SpotAutopilotAPIService();
export default spotAutopilotApi;
