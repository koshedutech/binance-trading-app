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
} from '../types';

class APIService {
  private client: AxiosInstance;

  constructor() {
    this.client = axios.create({
      baseURL: '/api',
      timeout: 10000,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Response interceptor for error handling
    this.client.interceptors.response.use(
      (response) => response,
      (error) => {
        console.error('API Error:', error);
        return Promise.reject(error);
      }
    );
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

  async getPositionHistory(limit = 50, offset = 0): Promise<Trade[]> {
    const { data } = await this.client.get<APIResponse<Trade[]>>('/positions/history', {
      params: { limit, offset },
    });
    return data.data || [];
  }

  async closePosition(symbol: string): Promise<void> {
    await this.client.post(`/positions/${symbol}/close`);
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
}

export const apiService = new APIService();
