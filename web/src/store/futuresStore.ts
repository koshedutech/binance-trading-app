import { create } from 'zustand';
import { futuresApi } from '../services/futuresApi';
import { apiService } from '../services/api';

// Default futures symbols as fallback when API is unavailable
const DEFAULT_FUTURES_SYMBOLS = [
  // Major
  'BTCUSDT', 'ETHUSDT', 'BNBUSDT', 'SOLUSDT', 'XRPUSDT',
  // Popular altcoins
  'DOGEUSDT', 'ADAUSDT', 'AVAXUSDT', 'LINKUSDT', 'MATICUSDT',
  // Additional popular
  'DOTUSDT', 'LTCUSDT', 'ATOMUSDT', 'UNIUSDT', 'NEARUSDT',
  // Memecoins
  'SHIBUSDT', 'PEPEUSDT', 'WIFUSDT',
  // Layer 2 & DeFi
  'ARBUSDT', 'OPUSDT', 'AAVEUSDT', 'MKRUSDT',
  // Gaming
  'SANDUSDT', 'MANAUSDT', 'AXSUSDT',
  // Infrastructure
  'FILUSDT', 'ICPUSDT', 'APTUSDT', 'SUIUSDT', 'SEIUSDT',
];

import type {
  FuturesAccountInfo,
  FuturesPosition,
  FuturesOrder,
  FuturesTrade,
  FundingFee,
  FuturesTransaction,
  FuturesAccountSettings,
  FuturesTradingMetrics,
  OrderBookDepth,
  FundingRate,
  MarkPrice,
  PositionSide,
  MarginType,
  FuturesOrderType,
  TimeInForce,
  PlaceFuturesOrderRequest,
} from '../types/futures';

// ==================== ORDER FORM STATE ====================

interface OrderFormState {
  symbol: string;
  side: 'BUY' | 'SELL';
  positionSide: PositionSide;
  orderType: FuturesOrderType;
  quantity: string;
  usdAmount: string;
  amountMode: 'coin' | 'usd';
  price: string;
  stopPrice: string;
  takeProfit: string;
  stopLoss: string;
  takeProfitPercent: string;
  stopLossPercent: string;
  tpSlMode: 'price' | 'percent';
  timeInForce: TimeInForce;
  reduceOnly: boolean;
  leverage: number;
  marginType: MarginType;
}

const defaultOrderForm: OrderFormState = {
  symbol: 'BTCUSDT',
  side: 'BUY',
  positionSide: 'BOTH',
  orderType: 'LIMIT',
  quantity: '',
  usdAmount: '',
  amountMode: 'coin',
  price: '',
  stopPrice: '',
  takeProfit: '',
  stopLoss: '',
  takeProfitPercent: '2',
  stopLossPercent: '1',
  tpSlMode: 'percent',
  timeInForce: 'GTC',
  reduceOnly: false,
  leverage: 10,
  marginType: 'CROSSED',
};

// ==================== STORE STATE ====================

interface TradingModeState {
  dryRun: boolean;
  mode: 'paper' | 'live';
  modeLabel: string;
  canSwitch: boolean;
}

interface FuturesState {
  // Connection & Loading
  isLoading: boolean;
  error: string | null;

  // Trading Mode
  tradingMode: TradingModeState;

  // Account
  accountInfo: FuturesAccountInfo | null;
  positionMode: 'ONE_WAY' | 'HEDGE';

  // Positions & Orders
  positions: FuturesPosition[];
  openOrders: FuturesOrder[];

  // Market Data
  selectedSymbol: string;
  orderBook: OrderBookDepth | null;
  markPrice: MarkPrice | null;
  fundingRate: FundingRate | null;
  symbols: string[];

  // History
  tradeHistory: FuturesTrade[];
  fundingFees: FundingFee[];
  transactions: FuturesTransaction[];

  // Settings per symbol
  accountSettings: Record<string, FuturesAccountSettings>;

  // Metrics
  metrics: FuturesTradingMetrics | null;

  // Order Form
  orderForm: OrderFormState;

  // Actions - Data Fetching
  fetchAccountInfo: () => Promise<void>;
  fetchPositions: () => Promise<void>;
  fetchOpenOrders: (symbol?: string) => Promise<void>;
  fetchOrderBook: (symbol: string, limit?: number) => Promise<void>;
  fetchMarkPrice: (symbol: string) => Promise<void>;
  fetchFundingRate: (symbol: string) => Promise<void>;
  fetchSymbols: () => Promise<void>;
  fetchTradeHistory: (limit?: number, offset?: number) => Promise<void>;
  fetchFundingFees: (symbol?: string, limit?: number, offset?: number) => Promise<void>;
  fetchTransactions: (symbol?: string, incomeType?: string, limit?: number, offset?: number) => Promise<void>;
  fetchAccountSettings: (symbol: string) => Promise<void>;
  fetchMetrics: () => Promise<void>;
  fetchPositionMode: () => Promise<void>;
  fetchTradingMode: () => Promise<void>;
  setTradingMode: (mode: TradingModeState) => void;

  // Actions - Trading
  placeOrder: () => Promise<boolean>;
  cancelOrder: (symbol: string, orderId: number) => Promise<boolean>;
  cancelAllOrders: (symbol: string) => Promise<boolean>;
  closePosition: (symbol: string) => Promise<boolean>;

  // Actions - Settings
  setLeverage: (symbol: string, leverage: number) => Promise<boolean>;
  setMarginType: (symbol: string, marginType: MarginType) => Promise<boolean>;
  setPositionMode: (dualSide: boolean) => Promise<boolean>;

  // Actions - UI
  setSelectedSymbol: (symbol: string) => void;
  updateOrderForm: (updates: Partial<OrderFormState>) => void;
  resetOrderForm: () => void;
  setError: (error: string | null) => void;
  resetState: () => void;  // Reset all state on logout - CRITICAL for multi-user isolation

  // Actions - WebSocket Updates
  updateOrderBook: (orderBook: OrderBookDepth) => void;
  updateMarkPrice: (markPrice: MarkPrice) => void;
  updatePositions: (positions: FuturesPosition[]) => void;
}

// ==================== STORE IMPLEMENTATION ====================

export const useFuturesStore = create<FuturesState>((set, get) => ({
  // Initial State
  isLoading: false,
  error: null,
  tradingMode: {
    dryRun: true,
    mode: 'paper',
    modeLabel: 'Paper Trading',
    canSwitch: true,
  },
  accountInfo: null,
  positionMode: 'ONE_WAY',
  positions: [],
  openOrders: [],
  selectedSymbol: 'BTCUSDT',
  orderBook: null,
  markPrice: null,
  fundingRate: null,
  symbols: DEFAULT_FUTURES_SYMBOLS, // Start with default symbols
  tradeHistory: [],
  fundingFees: [],
  transactions: [],
  accountSettings: {},
  metrics: null,
  orderForm: { ...defaultOrderForm },

  // ==================== DATA FETCHING ====================

  fetchAccountInfo: async () => {
    try {
      set({ isLoading: true });
      const accountInfo = await futuresApi.getAccountInfo();
      set({ accountInfo, isLoading: false, error: null });
    } catch (error: any) {
      console.error('Failed to fetch account info:', error);
      // Don't show error on initial load, just log it
      set({ isLoading: false });
    }
  },

  fetchPositions: async () => {
    try {
      const positions = await futuresApi.getPositions();
      set({ positions });
    } catch (error: any) {
      console.error('Failed to fetch positions:', error);
      // Don't show error for positions fetch
    }
  },

  fetchOpenOrders: async (symbol?: string) => {
    try {
      const openOrders = await futuresApi.getOpenOrders(symbol);
      set({ openOrders });
    } catch (error: any) {
      set({ error: error.message || 'Failed to fetch open orders' });
    }
  },

  fetchOrderBook: async (symbol: string, limit = 20) => {
    try {
      const orderBook = await futuresApi.getOrderBook(symbol, limit);
      set({ orderBook });
    } catch (error: any) {
      set({ error: error.message || 'Failed to fetch order book' });
    }
  },

  fetchMarkPrice: async (symbol: string) => {
    try {
      const markPrice = await futuresApi.getMarkPrice(symbol);
      set({ markPrice });
    } catch (error: any) {
      // Silently fail for mark price - it's fetched frequently
      console.debug('Failed to fetch mark price:', error);
    }
  },

  fetchFundingRate: async (symbol: string) => {
    try {
      const fundingRate = await futuresApi.getFundingRate(symbol);
      set({ fundingRate });
    } catch (error: any) {
      // Silently fail for funding rate - it's fetched frequently
      console.debug('Failed to fetch funding rate:', error);
    }
  },

  fetchSymbols: async () => {
    try {
      const symbols = await futuresApi.getSymbols();
      if (symbols && symbols.length > 0) {
        set({ symbols });
      } else {
        // Fallback to default symbols if empty response
        set({ symbols: DEFAULT_FUTURES_SYMBOLS });
      }
    } catch (error: any) {
      console.error('Failed to fetch symbols:', error);
      // Use fallback symbols on error instead of showing error to user
      set({ symbols: DEFAULT_FUTURES_SYMBOLS });
    }
  },

  fetchTradeHistory: async (limit = 50, offset = 0) => {
    try {
      const tradeHistory = await futuresApi.getTradeHistory(limit, offset);
      set({ tradeHistory });
    } catch (error: any) {
      set({ error: error.message || 'Failed to fetch trade history' });
    }
  },

  fetchFundingFees: async (symbol?: string, limit = 50, offset = 0) => {
    try {
      const fundingFees = await futuresApi.getFundingFeeHistory(symbol, limit, offset);
      set({ fundingFees });
    } catch (error: any) {
      set({ error: error.message || 'Failed to fetch funding fees' });
    }
  },

  fetchTransactions: async (symbol?: string, incomeType?: string, limit = 50, offset = 0) => {
    try {
      const transactions = await futuresApi.getTransactionHistory(symbol, incomeType, limit, offset);
      set({ transactions });
    } catch (error: any) {
      set({ error: error.message || 'Failed to fetch transactions' });
    }
  },

  fetchAccountSettings: async (symbol: string) => {
    try {
      const settings = await futuresApi.getAccountSettings(symbol);
      set((state) => ({
        accountSettings: { ...state.accountSettings, [symbol]: settings },
        orderForm: {
          ...state.orderForm,
          leverage: settings.leverage,
          marginType: settings.marginType,
        },
      }));
    } catch (error: any) {
      set({ error: error.message || 'Failed to fetch account settings' });
    }
  },

  fetchMetrics: async () => {
    try {
      const metrics = await futuresApi.getMetrics();
      set({ metrics });
    } catch (error: any) {
      console.error('Failed to fetch metrics:', error);
      // Don't show error for metrics fetch
    }
  },

  fetchPositionMode: async () => {
    try {
      const response = await futuresApi.getPositionMode();
      set({ positionMode: response.dualSidePosition ? 'HEDGE' : 'ONE_WAY' });
    } catch (error: any) {
      console.error('Failed to fetch position mode:', error);
      // Default to ONE_WAY on error
    }
  },

  fetchTradingMode: async () => {
    try {
      const response = await apiService.getTradingMode();
      set({
        tradingMode: {
          dryRun: response.dry_run,
          mode: response.mode,
          modeLabel: response.mode_label,
          canSwitch: response.can_switch,
        },
      });
    } catch (error: any) {
      // Default to paper trading on error
      console.error('Failed to fetch trading mode:', error);
    }
  },

  // Set trading mode directly (called from WebSocket events)
  setTradingMode: (mode: TradingModeState) => {
    set({ tradingMode: mode });
    console.log('FuturesStore: Trading mode updated via WebSocket:', mode);
  },

  // ==================== TRADING ====================

  placeOrder: async () => {
    const { orderForm, markPrice } = get();

    // Calculate quantity based on amount mode
    let quantity: number;
    if (orderForm.amountMode === 'usd') {
      if (!orderForm.usdAmount || parseFloat(orderForm.usdAmount) <= 0) {
        set({ error: 'Invalid USD amount' });
        return false;
      }
      const currentPrice = markPrice?.markPrice || 0;
      if (currentPrice <= 0) {
        set({ error: 'Cannot calculate quantity: price unavailable' });
        return false;
      }
      quantity = parseFloat(orderForm.usdAmount) / currentPrice;
    } else {
      if (!orderForm.quantity || parseFloat(orderForm.quantity) <= 0) {
        set({ error: 'Invalid quantity' });
        return false;
      }
      quantity = parseFloat(orderForm.quantity);
    }

    const request: PlaceFuturesOrderRequest = {
      symbol: orderForm.symbol,
      side: orderForm.side,
      position_side: orderForm.positionSide,
      order_type: orderForm.orderType,
      quantity: quantity,
      time_in_force: orderForm.timeInForce,
      reduce_only: orderForm.reduceOnly,
    };

    // Add price for limit orders
    if (orderForm.orderType === 'LIMIT' && orderForm.price) {
      request.price = parseFloat(orderForm.price);
    }

    // Add stop price for stop orders
    if (['STOP', 'STOP_MARKET', 'TAKE_PROFIT', 'TAKE_PROFIT_MARKET'].includes(orderForm.orderType) && orderForm.stopPrice) {
      request.stop_price = parseFloat(orderForm.stopPrice);
    }

    // Calculate TP/SL based on mode
    const currentPrice = markPrice?.markPrice || 0;
    const isLong = orderForm.side === 'BUY';

    if (orderForm.tpSlMode === 'percent' && currentPrice > 0) {
      // Calculate from percentages
      if (orderForm.takeProfitPercent && parseFloat(orderForm.takeProfitPercent) > 0) {
        const tpPercent = parseFloat(orderForm.takeProfitPercent) / 100;
        request.take_profit = isLong
          ? currentPrice * (1 + tpPercent)
          : currentPrice * (1 - tpPercent);
      }
      if (orderForm.stopLossPercent && parseFloat(orderForm.stopLossPercent) > 0) {
        const slPercent = parseFloat(orderForm.stopLossPercent) / 100;
        request.stop_loss = isLong
          ? currentPrice * (1 - slPercent)
          : currentPrice * (1 + slPercent);
      }
    } else {
      // Use exact prices
      if (orderForm.takeProfit) {
        request.take_profit = parseFloat(orderForm.takeProfit);
      }
      if (orderForm.stopLoss) {
        request.stop_loss = parseFloat(orderForm.stopLoss);
      }
    }

    try {
      set({ isLoading: true, error: null });
      const response = await futuresApi.placeOrder(request);

      // Check for TP/SL errors in response
      const warnings: string[] = [];
      if (response?.takeProfitError) {
        warnings.push(`TP failed: ${response.takeProfitError}`);
      }
      if (response?.stopLossError) {
        warnings.push(`SL failed: ${response.stopLossError}`);
      }

      if (warnings.length > 0) {
        // Order succeeded but TP/SL had issues
        set({ error: warnings.join(' | '), isLoading: false });
      } else {
        set({ isLoading: false });
      }

      // Refresh data
      get().fetchPositions();
      get().fetchOpenOrders();
      get().fetchAccountInfo();

      return true;
    } catch (error: any) {
      set({ error: error.response?.data?.error || error.message || 'Failed to place order', isLoading: false });
      return false;
    }
  },

  cancelOrder: async (symbol: string, orderId: number) => {
    try {
      set({ isLoading: true, error: null });
      await futuresApi.cancelOrder(symbol, orderId);
      set({ isLoading: false });
      get().fetchOpenOrders();
      return true;
    } catch (error: any) {
      set({ error: error.message || 'Failed to cancel order', isLoading: false });
      return false;
    }
  },

  cancelAllOrders: async (symbol: string) => {
    try {
      set({ isLoading: true, error: null });
      await futuresApi.cancelAllOrders(symbol);
      set({ isLoading: false });
      get().fetchOpenOrders();
      return true;
    } catch (error: any) {
      set({ error: error.message || 'Failed to cancel all orders', isLoading: false });
      return false;
    }
  },

  closePosition: async (symbol: string) => {
    try {
      set({ isLoading: true, error: null });
      await futuresApi.closePosition(symbol);
      set({ isLoading: false });
      get().fetchPositions();
      get().fetchAccountInfo();
      return true;
    } catch (error: any) {
      set({ error: error.message || 'Failed to close position', isLoading: false });
      return false;
    }
  },

  // ==================== SETTINGS ====================

  setLeverage: async (symbol: string, leverage: number) => {
    try {
      set({ isLoading: true, error: null });
      await futuresApi.setLeverage({ symbol, leverage });
      set((state) => ({
        isLoading: false,
        orderForm: { ...state.orderForm, leverage },
        accountSettings: {
          ...state.accountSettings,
          [symbol]: { ...state.accountSettings[symbol], leverage },
        },
      }));
      return true;
    } catch (error: any) {
      set({ error: error.message || 'Failed to set leverage', isLoading: false });
      return false;
    }
  },

  setMarginType: async (symbol: string, marginType: MarginType) => {
    try {
      set({ isLoading: true, error: null });
      await futuresApi.setMarginType({ symbol, margin_type: marginType });
      set((state) => ({
        isLoading: false,
        orderForm: { ...state.orderForm, marginType },
        accountSettings: {
          ...state.accountSettings,
          [symbol]: { ...state.accountSettings[symbol], marginType },
        },
      }));
      return true;
    } catch (error: any) {
      set({ error: error.message || 'Failed to set margin type', isLoading: false });
      return false;
    }
  },

  setPositionMode: async (dualSide: boolean) => {
    try {
      set({ isLoading: true, error: null });
      await futuresApi.setPositionMode({ dual_side_position: dualSide });
      set({
        isLoading: false,
        positionMode: dualSide ? 'HEDGE' : 'ONE_WAY',
      });
      return true;
    } catch (error: any) {
      set({ error: error.message || 'Failed to set position mode', isLoading: false });
      return false;
    }
  },

  // ==================== UI ====================

  setSelectedSymbol: (symbol: string) => {
    set((state) => ({
      selectedSymbol: symbol,
      orderForm: { ...state.orderForm, symbol },
      orderBook: null,
      markPrice: null,
      fundingRate: null,
    }));

    // Fetch data for new symbol
    get().fetchOrderBook(symbol);
    get().fetchMarkPrice(symbol);
    get().fetchFundingRate(symbol);
    get().fetchAccountSettings(symbol);
  },

  updateOrderForm: (updates: Partial<OrderFormState>) => {
    set((state) => ({
      orderForm: { ...state.orderForm, ...updates },
    }));
  },

  resetOrderForm: () => {
    const { selectedSymbol, orderForm } = get();
    set({
      orderForm: {
        ...defaultOrderForm,
        symbol: selectedSymbol,
        leverage: orderForm.leverage,
        marginType: orderForm.marginType,
      },
    });
  },

  setError: (error: string | null) => set({ error }),

  // Reset all state on logout - CRITICAL for multi-user data isolation
  resetState: () => {
    console.log('FuturesStore: Resetting all state for user logout');
    set({
      isLoading: false,
      error: null,
      tradingMode: {
        dryRun: true,
        mode: 'paper',
        modeLabel: 'PAPER',
        canSwitch: true,
      },
      accountInfo: null,
      positionMode: 'ONE_WAY',
      positions: [],
      openOrders: [],
      selectedSymbol: 'BTCUSDT',
      orderBook: null,
      markPrice: null,
      fundingRate: null,
      symbols: [],
      tradeHistory: [],
      fundingFees: [],
      transactions: [],
      accountSettings: {},
      metrics: null,
      orderForm: defaultOrderForm,
    });
  },

  // ==================== WEBSOCKET UPDATES ====================

  updateOrderBook: (orderBook: OrderBookDepth) => set({ orderBook }),

  updateMarkPrice: (markPrice: MarkPrice) => set({ markPrice }),

  updatePositions: (positions: FuturesPosition[]) => set({ positions }),
}));

// ==================== SELECTORS ====================

// Helper to safely parse numeric values that may come as strings from API
const safeNumber = (val: number | string | null | undefined): number => {
  if (val === null || val === undefined) return 0;
  const num = typeof val === 'string' ? parseFloat(val) : val;
  return isNaN(num) ? 0 : num;
};

export const selectActivePositions = (state: FuturesState) =>
  state.positions.filter((p) => safeNumber(p.positionAmt) !== 0);

export const selectTotalUnrealizedPnl = (state: FuturesState) =>
  state.positions.reduce((sum, p) => sum + safeNumber(p.unRealizedProfit), 0);

export const selectTotalMarginUsed = (state: FuturesState) =>
  safeNumber(state.accountInfo?.total_position_initial_margin);

export const selectAvailableBalance = (state: FuturesState) =>
  safeNumber(state.accountInfo?.available_balance);

export const selectOrderBookSpread = (state: FuturesState) => {
  if (!state.orderBook?.bids?.length || !state.orderBook?.asks?.length) {
    return { spread: 0, spreadPercent: 0 };
  }
  const bestBid = parseFloat(state.orderBook.bids[0][0]);
  const bestAsk = parseFloat(state.orderBook.asks[0][0]);
  const spread = bestAsk - bestBid;
  const spreadPercent = (spread / bestAsk) * 100;
  return { spread, spreadPercent, bestBid, bestAsk };
};
