import type { WSEvent } from '../types';

type EventCallback = (event: WSEvent) => void;
type ConnectionCallback = () => void;

const ACCESS_TOKEN_KEY = 'access_token';

class WebSocketService {
  private ws: WebSocket | null = null;
  private baseUrl: string;
  private reconnectTimer: number | null = null;
  private isConnecting = false;
  private eventCallbacks: Map<string, EventCallback[]> = new Map();
  private onConnectCallbacks: ConnectionCallback[] = [];
  private onDisconnectCallbacks: ConnectionCallback[] = [];
  private useAuthenticatedEndpoint = true;

  // Exponential backoff settings
  private reconnectAttempts = 0;
  private readonly initialReconnectDelay = 1000; // 1 second
  private readonly maxReconnectDelay = 30000; // 30 seconds max

  constructor() {
    // Determine WebSocket URL based on current location
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    this.baseUrl = `${protocol}//${host}`;
  }

  /**
   * Calculate reconnect delay using exponential backoff
   * Delay = min(initialDelay * 2^attempts, maxDelay)
   */
  private getReconnectDelay(): number {
    const delay = Math.min(
      this.initialReconnectDelay * Math.pow(2, this.reconnectAttempts),
      this.maxReconnectDelay
    );
    return delay;
  }

  /**
   * Reset all callbacks and state - MUST be called on logout
   * to prevent data leakage between users
   */
  reset(): void {
    console.log('WebSocket: Resetting all callbacks and state');
    this.eventCallbacks.clear();
    this.onConnectCallbacks = [];
    this.onDisconnectCallbacks = [];
  }

  /**
   * Get the WebSocket URL with auth token for authenticated endpoint
   */
  private getUrl(): string {
    if (this.useAuthenticatedEndpoint) {
      const token = localStorage.getItem(ACCESS_TOKEN_KEY);
      if (token) {
        // Use authenticated endpoint with token as query param
        return `${this.baseUrl}/ws/user?token=${encodeURIComponent(token)}`;
      }
    }
    // Fallback to public endpoint (only for market data)
    return `${this.baseUrl}/ws`;
  }

  connect(): void {
    if (this.ws || this.isConnecting) {
      return;
    }

    this.isConnecting = true;
    const url = this.getUrl();
    console.log('Connecting to WebSocket:', url.replace(/token=[^&]+/, 'token=***'));

    try {
      this.ws = new WebSocket(url);

      this.ws.onopen = () => {
        console.log('WebSocket connected');
        this.isConnecting = false;
        this.reconnectAttempts = 0; // Reset backoff on successful connection
        this.clearReconnectTimer();
        this.onConnectCallbacks.forEach((cb) => cb());
      };

      this.ws.onmessage = (event) => {
        try {
          const wsEvent: WSEvent = JSON.parse(event.data);
          this.handleEvent(wsEvent);
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error);
        }
      };

      this.ws.onerror = (error) => {
        console.error('WebSocket error:', error);
      };

      this.ws.onclose = () => {
        console.log('WebSocket disconnected');
        this.ws = null;
        this.isConnecting = false;
        this.onDisconnectCallbacks.forEach((cb) => cb());
        this.scheduleReconnect();
      };
    } catch (error) {
      console.error('Failed to create WebSocket:', error);
      this.isConnecting = false;
      this.scheduleReconnect();
    }
  }

  disconnect(): void {
    this.clearReconnectTimer();
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  subscribe(eventType: string, callback: EventCallback): void {
    if (!this.eventCallbacks.has(eventType)) {
      this.eventCallbacks.set(eventType, []);
    }
    this.eventCallbacks.get(eventType)!.push(callback);
  }

  subscribeAll(callback: EventCallback): void {
    this.subscribe('*', callback);
  }

  unsubscribe(eventType: string, callback: EventCallback): void {
    const callbacks = this.eventCallbacks.get(eventType);
    if (callbacks) {
      const index = callbacks.indexOf(callback);
      if (index > -1) {
        callbacks.splice(index, 1);
      }
    }
  }

  onConnect(callback: ConnectionCallback): void {
    this.onConnectCallbacks.push(callback);
  }

  offConnect(callback: ConnectionCallback): void {
    const index = this.onConnectCallbacks.indexOf(callback);
    if (index > -1) {
      this.onConnectCallbacks.splice(index, 1);
    }
  }

  onDisconnect(callback: ConnectionCallback): void {
    this.onDisconnectCallbacks.push(callback);
  }

  offDisconnect(callback: ConnectionCallback): void {
    const index = this.onDisconnectCallbacks.indexOf(callback);
    if (index > -1) {
      this.onDisconnectCallbacks.splice(index, 1);
    }
  }

  isConnected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN;
  }

  private handleEvent(event: WSEvent): void {
    // Notify specific event type subscribers
    const callbacks = this.eventCallbacks.get(event.type);
    if (callbacks) {
      callbacks.forEach((cb) => cb(event));
    }

    // Notify all-event subscribers
    const allCallbacks = this.eventCallbacks.get('*');
    if (allCallbacks) {
      allCallbacks.forEach((cb) => cb(event));
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer) {
      return;
    }

    const delay = this.getReconnectDelay();
    this.reconnectAttempts++;
    console.log(`[WebSocket] Reconnect attempt ${this.reconnectAttempts}, waiting ${delay / 1000}s (exponential backoff)`);

    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, delay);
  }

  /**
   * Get current reconnect attempts (for status display)
   */
  getReconnectAttempts(): number {
    return this.reconnectAttempts;
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }
}

export const wsService = new WebSocketService();
