import React, { createContext, useContext, useState, useEffect, useCallback, ReactNode } from 'react';
import { api } from '../services/api';

// Types
export interface User {
  id: string;
  email: string;
  name: string;
  email_verified: boolean;
  subscription_tier: 'free' | 'trader' | 'pro' | 'whale';
  profit_share_pct: number;
  api_key_mode: 'user_provided' | 'master_account';
  is_admin: boolean;
  created_at: string;
  last_login_at?: string;
}

interface LoginCredentials {
  email: string;
  password: string;
}

interface RegisterData {
  email: string;
  password: string;
  name: string;
  referral_code?: string;
}

interface AuthContextType {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (credentials: LoginCredentials) => Promise<void>;
  register: (data: RegisterData) => Promise<void>;
  logout: () => Promise<void>;
  refreshAuth: () => Promise<void>;
  refreshUser: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

// Token storage keys
const ACCESS_TOKEN_KEY = 'access_token';
const REFRESH_TOKEN_KEY = 'refresh_token';
const USER_KEY = 'user';

// Helper functions for token storage
const getStoredTokens = () => ({
  accessToken: localStorage.getItem(ACCESS_TOKEN_KEY),
  refreshToken: localStorage.getItem(REFRESH_TOKEN_KEY),
});

const setStoredTokens = (accessToken: string, refreshToken: string) => {
  localStorage.setItem(ACCESS_TOKEN_KEY, accessToken);
  localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken);
};

const clearStoredTokens = () => {
  localStorage.removeItem(ACCESS_TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
  localStorage.removeItem(USER_KEY);
};

const getStoredUser = (): User | null => {
  const userStr = localStorage.getItem(USER_KEY);
  if (userStr) {
    try {
      return JSON.parse(userStr);
    } catch {
      return null;
    }
  }
  return null;
};

const setStoredUser = (user: User) => {
  localStorage.setItem(USER_KEY, JSON.stringify(user));
};

// Auth Provider Props
interface AuthProviderProps {
  children: ReactNode;
}

// Default local user when auth is disabled
const LOCAL_USER: User = {
  id: 'local-user',
  email: 'local@localhost',
  name: 'Local User',
  email_verified: true,
  subscription_tier: 'whale',
  profit_share_pct: 0,
  api_key_mode: 'master_account',
  is_admin: true,
  created_at: new Date().toISOString(),
};

export const AuthProvider: React.FC<AuthProviderProps> = ({ children }) => {
  const [user, setUser] = useState<User | null>(getStoredUser);
  const [isLoading, setIsLoading] = useState(true);
  const [authDisabled, setAuthDisabled] = useState(false);

  // Check if user is authenticated (or auth is disabled)
  const isAuthenticated = !!user || authDisabled;

  // Refresh authentication on mount
  useEffect(() => {
    const initAuth = async () => {
      // First check if auth is disabled on backend
      try {
        const statusResponse = await api.get('/auth/status');
        if (statusResponse.data?.auth_enabled === false) {
          // Auth disabled - use local user
          setAuthDisabled(true);
          setUser(LOCAL_USER);
          setIsLoading(false);
          return;
        }
      } catch {
        // If /auth/status fails, continue with normal auth flow
      }

      const { accessToken } = getStoredTokens();
      if (accessToken) {
        try {
          // Verify token is still valid
          const response = await api.get('/auth/me');
          // /auth/me returns user data directly
          const userData = response.data as User;
          setUser(userData);
          setStoredUser(userData);
        } catch (error) {
          // Token invalid, try refresh
          try {
            await refreshAuth();
          } catch {
            // Refresh failed, clear tokens
            clearStoredTokens();
            setUser(null);
          }
        }
      }
      setIsLoading(false);
    };

    initAuth();
  }, []);

  // Login function
  const login = useCallback(async (credentials: LoginCredentials) => {
    const response = await api.post('/auth/login', credentials);
    // Auth endpoints return data directly, not wrapped in a 'data' field
    const { user: userData, access_token, refresh_token } = response.data as {
      user: User;
      access_token: string;
      refresh_token: string;
      expires_in: number;
    };

    setStoredTokens(access_token, refresh_token);
    setStoredUser(userData);
    setUser(userData);
  }, []);

  // Register function
  const register = useCallback(async (data: RegisterData) => {
    // First register the user
    await api.post('/auth/register', data);

    // After registration, automatically log them in
    const loginResponse = await api.post('/auth/login', { email: data.email, password: data.password });
    const { user: userData, access_token, refresh_token } = loginResponse.data as {
      user: User;
      access_token: string;
      refresh_token: string;
      expires_in: number;
    };

    setStoredTokens(access_token, refresh_token);
    setStoredUser(userData);
    setUser(userData);
  }, []);

  // Logout function
  const logout = useCallback(async () => {
    const { refreshToken } = getStoredTokens();
    try {
      if (refreshToken) {
        await api.post('/auth/logout', { refresh_token: refreshToken });
      }
    } catch {
      // Ignore logout errors
    } finally {
      clearStoredTokens();
      setUser(null);
    }
  }, []);

  // Refresh authentication
  const refreshAuth = useCallback(async () => {
    const { refreshToken } = getStoredTokens();
    if (!refreshToken) {
      throw new Error('No refresh token');
    }

    const response = await api.post('/auth/refresh', { refresh_token: refreshToken });
    // Refresh returns tokens directly
    const { access_token, refresh_token: newRefreshToken } = response.data as {
      access_token: string;
      refresh_token: string;
      expires_in: number;
    };

    setStoredTokens(access_token, newRefreshToken);

    // Fetch updated user data
    const userResponse = await api.get('/auth/me');
    const userData = userResponse.data as User;
    setStoredUser(userData);
    setUser(userData);
  }, []);

  // Refresh user data only (without token refresh)
  const refreshUser = useCallback(async () => {
    const userResponse = await api.get('/auth/me');
    const userData = userResponse.data as User;
    setStoredUser(userData);
    setUser(userData);
  }, []);

  const value = {
    user,
    isAuthenticated,
    isLoading,
    login,
    register,
    logout,
    refreshAuth,
    refreshUser,
  };

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
};

// Hook to use auth context
export const useAuth = (): AuthContextType => {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};

// HOC for protected routes
interface ProtectedRouteProps {
  children: ReactNode;
  requiredTier?: User['subscription_tier'][];
  requireAdmin?: boolean;
}

export const ProtectedRoute: React.FC<ProtectedRouteProps> = ({
  children,
  requiredTier,
  requireAdmin = false,
}) => {
  const { user, isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-indigo-600"></div>
      </div>
    );
  }

  if (!isAuthenticated) {
    // Redirect to login
    window.location.href = '/login';
    return null;
  }

  if (requireAdmin && !user?.is_admin) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-red-600">Access Denied</h1>
          <p className="text-gray-600 mt-2">You do not have permission to access this page.</p>
        </div>
      </div>
    );
  }

  if (requiredTier && user && !requiredTier.includes(user.subscription_tier)) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-yellow-600">Upgrade Required</h1>
          <p className="text-gray-600 mt-2">
            This feature requires a {requiredTier.join(' or ')} subscription.
          </p>
          <a href="/billing" className="mt-4 inline-block bg-indigo-600 text-white px-4 py-2 rounded">
            Upgrade Now
          </a>
        </div>
      </div>
    );
  }

  return <>{children}</>;
};

// Export tier information
export const TIER_INFO = {
  free: { name: 'Free', maxPositions: 3, profitShare: 30, monthlyFee: 0, features: ['Spot trading only'] },
  trader: { name: 'Trader', maxPositions: 10, profitShare: 20, monthlyFee: 49, features: ['Spot + Futures'] },
  pro: { name: 'Pro', maxPositions: 25, profitShare: 12, monthlyFee: 149, features: ['Priority support'] },
  whale: { name: 'Whale', maxPositions: -1, profitShare: 5, monthlyFee: 499, features: ['Dedicated agent', 'Unlimited'] },
};

export default AuthContext;
