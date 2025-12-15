import { create } from 'zustand';
import type {
  BotStatus,
  Position,
  Order,
  Strategy,
  Signal,
  ScreenerResult,
  TradingMetrics,
} from '../types';

interface AppState {
  // Connection state
  isConnected: boolean;
  isWSConnected: boolean;

  // Bot state
  botStatus: BotStatus | null;

  // Trading data
  positions: Position[];
  activeOrders: Order[];
  strategies: Strategy[];
  recentSignals: Signal[];
  screenerResults: ScreenerResult[];
  metrics: TradingMetrics | null;

  // UI state
  selectedTab: string;
  isLoading: boolean;
  error: string | null;

  // Actions
  setConnected: (connected: boolean) => void;
  setWSConnected: (connected: boolean) => void;
  setBotStatus: (status: BotStatus) => void;
  setPositions: (positions: Position[]) => void;
  updatePosition: (symbol: string, updates: Partial<Position>) => void;
  setActiveOrders: (orders: Order[]) => void;
  setStrategies: (strategies: Strategy[]) => void;
  setRecentSignals: (signals: Signal[]) => void;
  setScreenerResults: (results: ScreenerResult[]) => void;
  setMetrics: (metrics: TradingMetrics) => void;
  setSelectedTab: (tab: string) => void;
  setLoading: (loading: boolean) => void;
  setError: (error: string | null) => void;
}

export const useStore = create<AppState>((set) => ({
  // Initial state
  isConnected: false,
  isWSConnected: false,
  botStatus: null,
  positions: [],
  activeOrders: [],
  strategies: [],
  recentSignals: [],
  screenerResults: [],
  metrics: null,
  selectedTab: 'dashboard',
  isLoading: false,
  error: null,

  // Actions
  setConnected: (connected) => set({ isConnected: connected }),
  setWSConnected: (connected) => set({ isWSConnected: connected }),
  setBotStatus: (status) => set({ botStatus: status }),
  setPositions: (positions) => set({ positions }),
  updatePosition: (symbol, updates) =>
    set((state) => ({
      positions: state.positions.map((pos) =>
        pos.symbol === symbol ? { ...pos, ...updates } : pos
      ),
    })),
  setActiveOrders: (orders) => set({ activeOrders: orders }),
  setStrategies: (strategies) => set({ strategies }),
  setRecentSignals: (signals) => set({ recentSignals: signals }),
  setScreenerResults: (results) => set({ screenerResults: results }),
  setMetrics: (metrics) => set({ metrics }),
  setSelectedTab: (tab) => set({ selectedTab: tab }),
  setLoading: (loading) => set({ isLoading: loading }),
  setError: (error) => set({ error }),
}));
