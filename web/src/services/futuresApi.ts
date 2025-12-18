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

export function formatUSD(value: number): string {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value);
}

export function formatPercent(value: number, includeSign = true): string {
  const sign = includeSign && value > 0 ? '+' : '';
  return `${sign}${value.toFixed(2)}%`;
}

export function formatFundingRate(rate: number): string {
  return `${(rate * 100).toFixed(4)}%`;
}

export function getPositionColor(pnl: number): string {
  if (pnl > 0) return 'text-green-500';
  if (pnl < 0) return 'text-red-500';
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
