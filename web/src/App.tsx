import { useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { useStore } from './store';
import { apiService } from './services/api';
import { wsService } from './services/websocket';
import Dashboard from './pages/Dashboard';
import VisualStrategyDemoEnhanced from './pages/VisualStrategyDemoEnhanced';
import PatternScannerPage from './pages/PatternScannerPage';
import Header from './components/Header';
import ConnectionIndicator from './components/ConnectionIndicator';

function App() {
  const {
    setConnected,
    setWSConnected,
    setBotStatus,
    setPositions,
    setActiveOrders,
    setStrategies,
    setRecentSignals,
    setScreenerResults,
    setMetrics,
    updatePosition,
  } = useStore();

  useEffect(() => {
    // Initialize data fetching
    const initializeData = async () => {
      try {
        // Fetch initial data
        const [status, positions, orders, strategies, signals, screener, metrics] = await Promise.all([
          apiService.getBotStatus(),
          apiService.getPositions(),
          apiService.getActiveOrders(),
          apiService.getStrategies(),
          apiService.getSignals(20),
          apiService.getScreenerResults(20),
          apiService.getMetrics(),
        ]);

        setBotStatus(status);
        setPositions(positions);
        setActiveOrders(orders);
        setStrategies(strategies);
        setRecentSignals(signals);
        setScreenerResults(screener);
        setMetrics(metrics);
        setConnected(true);
      } catch (error) {
        console.error('Failed to initialize data:', error);
        setConnected(false);
      }
    };

    // Connect to WebSocket
    wsService.connect();

    // WebSocket event handlers
    wsService.onConnect(() => {
      console.log('Connected to WebSocket');
      setWSConnected(true);
    });

    wsService.onDisconnect(() => {
      console.log('Disconnected from WebSocket');
      setWSConnected(false);
    });

    // Subscribe to WebSocket events
    wsService.subscribe('POSITION_UPDATE', (event) => {
      const { symbol, entry_price, current_price, quantity, pnl, pnl_percent } = event.data;
      updatePosition(symbol, {
        entry_price,
        current_price,
        quantity,
        pnl,
        pnl_percent,
      });
    });

    wsService.subscribe('TRADE_OPENED', async () => {
      // Refresh positions when a new trade opens
      const positions = await apiService.getPositions();
      setPositions(positions);
    });

    wsService.subscribe('TRADE_CLOSED', async () => {
      // Refresh positions and metrics when a trade closes
      const [positions, metrics] = await Promise.all([
        apiService.getPositions(),
        apiService.getMetrics(),
      ]);
      setPositions(positions);
      setMetrics(metrics);
    });

    wsService.subscribe('ORDER_PLACED', async () => {
      // Refresh orders when a new order is placed
      const orders = await apiService.getActiveOrders();
      setActiveOrders(orders);
    });

    wsService.subscribe('SCREENER_UPDATE', async () => {
      // Refresh screener results
      const results = await apiService.getScreenerResults(20);
      setScreenerResults(results);
    });

    // Initialize
    initializeData();

    // Set up polling for data that doesn't come through WebSocket
    const pollInterval = setInterval(async () => {
      try {
        const [positions, orders, signals, screener, metrics] = await Promise.all([
          apiService.getPositions(),
          apiService.getActiveOrders(),
          apiService.getSignals(20),
          apiService.getScreenerResults(20),
          apiService.getMetrics(),
        ]);
        setPositions(positions);
        setActiveOrders(orders);
        setRecentSignals(signals);
        setScreenerResults(screener);
        setMetrics(metrics);
      } catch (error) {
        console.error('Polling error:', error);
      }
    }, 1000); // Poll every 1 second

    // Cleanup
    return () => {
      clearInterval(pollInterval);
      wsService.disconnect();
    };
  }, []);

  return (
    <BrowserRouter>
      <div className="min-h-screen bg-dark-900">
        <Header />
        <ConnectionIndicator />
        <main className="container mx-auto px-4 py-6">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/visual-strategy-advanced" element={<VisualStrategyDemoEnhanced />} />
            <Route path="/pattern-scanner" element={<PatternScannerPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </main>
      </div>
    </BrowserRouter>
  );
}

export default App;
