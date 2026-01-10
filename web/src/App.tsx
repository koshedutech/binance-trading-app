import { useEffect, useRef } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { useStore } from './store';
import { useFuturesStore } from './store/futuresStore';
import { apiService } from './services/api';
import { wsService } from './services/websocket';
import { futuresApi } from './services/futuresApi';
import { AuthProvider, useAuth, ProtectedRoute } from './contexts/AuthContext';
import Dashboard from './pages/Dashboard';
// Temporarily disabled due to @xyflow/react type export issues
// import VisualStrategyDemoEnhanced from './pages/VisualStrategyDemoEnhanced';
import PatternScannerPage from './pages/PatternScannerPage';
import FuturesDashboard from './pages/FuturesDashboard';
import Investigate from './pages/Investigate';
import EnhancedStrategyBuilder from './pages/EnhancedStrategyBuilder';
import Login from './pages/Login';
import Register from './pages/Register';
import VerifyEmail from './pages/VerifyEmail';
import Profile from './pages/Profile';
import Settings from './pages/Settings';
import APIKeys from './pages/APIKeys';
import AIKeys from './pages/AIKeys';
import Billing from './pages/Billing';
import AdminSettings from './pages/AdminSettings';
import ResetSettings from './pages/ResetSettings';
import Header from './components/Header';
import ConnectionIndicator from './components/ConnectionIndicator';

// Main app content that requires authentication context
function AppContent() {
  const { isAuthenticated, isLoading, subscriptionEnabled } = useAuth();
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
    resetState,
  } = useStore();

  // Track if we've already auto-started Ginie for this session
  const ginieAutoStarted = useRef(false);

  useEffect(() => {
    // Only initialize data and websocket when authenticated
    if (!isAuthenticated || isLoading) {
      return;
    }

    // Auto-start Ginie autopilot on login (runs once per session)
    const autoStartGinie = async () => {
      if (ginieAutoStarted.current) {
        return; // Already started this session
      }

      try {
        // Check current Ginie status
        const status = await futuresApi.getGinieAutopilotStatus();

        if (!status.stats?.running) {
          // Ginie not running - auto-start it
          console.log('[AUTO-START] Starting Ginie autopilot...');
          await futuresApi.toggleGinie(true);
          console.log('[AUTO-START] Ginie autopilot started successfully');
        } else {
          console.log('[AUTO-START] Ginie already running, skipping auto-start');
        }

        ginieAutoStarted.current = true;
      } catch (error) {
        console.error('[AUTO-START] Failed to auto-start Ginie:', error);
        // Don't retry - let user manually start if needed
        ginieAutoStarted.current = true;
      }
    };

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

        // Auto-start Ginie after successful data initialization
        autoStartGinie();
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

    // Subscribe to trading mode changes - keeps UI in sync across tabs/users
    wsService.subscribe('TRADING_MODE_CHANGED', (event) => {
      console.log('WebSocket: TRADING_MODE_CHANGED event received', event.data);
      const { dry_run, mode, mode_label } = event.data;
      useFuturesStore.getState().setTradingMode({
        dryRun: dry_run,
        mode: mode,
        modeLabel: mode_label,
        canSwitch: true,
      });
    });

    // Subscribe to autopilot toggle events - keeps UI in sync
    wsService.subscribe('AUTOPILOT_TOGGLED', (event) => {
      console.log('WebSocket: AUTOPILOT_TOGGLED event received', event.data);
      // Update trading mode if dry_run changed
      const { dry_run } = event.data;
      const currentMode = useFuturesStore.getState().tradingMode;
      if (currentMode.dryRun !== dry_run) {
        useFuturesStore.getState().setTradingMode({
          ...currentMode,
          dryRun: dry_run,
          mode: dry_run ? 'paper' : 'live',
          modeLabel: dry_run ? 'Paper Trading' : 'Live Trading',
        });
      }
      // Note: Autopilot status is handled by GiniePanel/FuturesAutopilotPanel polling
      // This event is mainly for mode sync
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
      } catch (error: any) {
        // Don't log auth errors (expected when not logged in)
        if (error?.response?.status !== 401 && error?.response?.status !== 403) {
          console.error('Polling error:', error);
        }
      }
    }, 15000); // Poll every 15 seconds (reduced from 1s to avoid rate limits)

    // Cleanup - reset all state when user logs out
    // CRITICAL: Reset ALL state to prevent data leakage between users
    return () => {
      clearInterval(pollInterval);
      wsService.reset();  // Clear all WebSocket callbacks first - prevents cross-user data leakage
      wsService.disconnect();
      resetState();  // Reset main store
      useFuturesStore.getState().resetState();  // Reset futures store - CRITICAL for multi-user isolation
      ginieAutoStarted.current = false;  // Reset auto-start flag for next login
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

          {/* Semi-protected route (requires auth but not verification) */}
          <Route path="/verify-email" element={
            <ProtectedRoute>
              <VerifyEmail />
            </ProtectedRoute>
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
          {/* Temporarily disabled due to @xyflow/react type export issues
          <Route path="/visual-strategy-advanced" element={
            <ProtectedRoute>
              <VisualStrategyDemoEnhanced />
            </ProtectedRoute>
          } />
          */}
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
          <Route path="/settings" element={
            <ProtectedRoute>
              <Settings />
            </ProtectedRoute>
          } />
          <Route path="/api-keys" element={
            <ProtectedRoute>
              <APIKeys />
            </ProtectedRoute>
          } />
          <Route path="/ai-keys" element={
            <ProtectedRoute>
              <AIKeys />
            </ProtectedRoute>
          } />
          <Route path="/billing" element={
            subscriptionEnabled ? (
              <ProtectedRoute>
                <Billing />
              </ProtectedRoute>
            ) : (
              <Navigate to="/dashboard" replace />
            )
          } />
          <Route path="/admin" element={
            <ProtectedRoute requireAdmin>
              <AdminSettings />
            </ProtectedRoute>
          } />
          <Route path="/reset-settings" element={
            <ProtectedRoute>
              <ResetSettings />
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
