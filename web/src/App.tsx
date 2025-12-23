import { useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { useStore } from './store';
import { apiService } from './services/api';
import { wsService } from './services/websocket';
import { AuthProvider, useAuth, ProtectedRoute } from './contexts/AuthContext';
import Dashboard from './pages/Dashboard';
import VisualStrategyDemoEnhanced from './pages/VisualStrategyDemoEnhanced';
import PatternScannerPage from './pages/PatternScannerPage';
import FuturesDashboard from './pages/FuturesDashboard';
import Investigate from './pages/Investigate';
import EnhancedStrategyBuilder from './pages/EnhancedStrategyBuilder';
import Login from './pages/Login';
import Register from './pages/Register';
import Profile from './pages/Profile';
import APIKeys from './pages/APIKeys';
import Billing from './pages/Billing';
import Header from './components/Header';
import ConnectionIndicator from './components/ConnectionIndicator';

// Main app content that requires authentication context
function AppContent() {
  const { isAuthenticated, isLoading } = useAuth();
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
    // Only initialize data and websocket when authenticated
    if (!isAuthenticated || isLoading) {
      return;
    }

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
    }, 15000); // Poll every 15 seconds (reduced from 1s to avoid rate limits)

    // Cleanup
    return () => {
      clearInterval(pollInterval);
      wsService.disconnect();
    };
  }, [isAuthenticated, isLoading]);

  // Show loading spinner while checking auth
  if (isLoading) {
    return (
      <div className="min-h-screen bg-dark-900 flex items-center justify-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-indigo-600"></div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-dark-900">
      {/* Always show header, connection indicator only when authenticated */}
      <Header />
      {isAuthenticated && <ConnectionIndicator />}
      <main className={isAuthenticated ? "container mx-auto px-4 py-6" : ""}>
        <Routes>
          {/* Public routes */}
          <Route path="/login" element={
            isAuthenticated ? <Navigate to="/" replace /> : <Login />
          } />
          <Route path="/register" element={
            isAuthenticated ? <Navigate to="/" replace /> : <Register />
          } />

          {/* Protected routes */}
          <Route path="/" element={
            <ProtectedRoute>
              <Dashboard />
            </ProtectedRoute>
          } />
          <Route path="/dashboard" element={
            <ProtectedRoute>
              <Dashboard />
            </ProtectedRoute>
          } />
          <Route path="/futures" element={
            <ProtectedRoute requiredTier={['trader', 'pro', 'whale']}>
              <FuturesDashboard />
            </ProtectedRoute>
          } />
          <Route path="/investigate" element={
            <ProtectedRoute requiredTier={['trader', 'pro', 'whale']}>
              <Investigate />
            </ProtectedRoute>
          } />
          <Route path="/strategy-builder" element={
            <ProtectedRoute>
              <EnhancedStrategyBuilder />
            </ProtectedRoute>
          } />
          <Route path="/visual-strategy-advanced" element={
            <ProtectedRoute>
              <VisualStrategyDemoEnhanced />
            </ProtectedRoute>
          } />
          <Route path="/pattern-scanner" element={
            <ProtectedRoute>
              <PatternScannerPage />
            </ProtectedRoute>
          } />
          <Route path="/profile" element={
            <ProtectedRoute>
              <Profile />
            </ProtectedRoute>
          } />
          <Route path="/api-keys" element={
            <ProtectedRoute>
              <APIKeys />
            </ProtectedRoute>
          } />
          <Route path="/billing" element={
            <ProtectedRoute>
              <Billing />
            </ProtectedRoute>
          } />

          {/* Catch all - redirect to login if not authenticated, dashboard if authenticated */}
          <Route path="*" element={
            isAuthenticated ? <Navigate to="/" replace /> : <Navigate to="/login" replace />
          } />
        </Routes>
      </main>
    </div>
  );
}

// Main App component wrapped with providers
function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <AppContent />
      </AuthProvider>
    </BrowserRouter>
  );
}

export default App;
