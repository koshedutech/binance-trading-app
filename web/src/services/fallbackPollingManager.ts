type FetchFunction = () => Promise<void>;
type ChangeListener = (isActive: boolean) => void;

class FallbackPollingManager {
  private intervals: Map<string, NodeJS.Timeout> = new Map();
  private isActive = false;
  private fetchFunctions: Map<string, FetchFunction> = new Map();
  private changeListeners: Set<ChangeListener> = new Set();

  registerFetchFunction(key: string, fn: FetchFunction): void {
    this.fetchFunctions.set(key, fn);
  }

  unregisterFetchFunction(key: string): void {
    this.fetchFunctions.delete(key);
    // Also clear any running interval for this key
    const interval = this.intervals.get(key);
    if (interval) {
      clearInterval(interval);
      this.intervals.delete(key);
    }
  }

  clearAllFetchFunctions(): void {
    this.stop();
    this.fetchFunctions.clear();
  }

  onChange(listener: ChangeListener): void {
    this.changeListeners.add(listener);
  }

  offChange(listener: ChangeListener): void {
    this.changeListeners.delete(listener);
  }

  private notifyChange(): void {
    this.changeListeners.forEach(listener => listener(this.isActive));
  }

  start(): void {
    if (this.isActive) return;
    this.isActive = true;
    this.notifyChange();

    this.fetchFunctions.forEach((fetchFn, key) => {
      // Initial fetch with error handling
      fetchFn().catch(e => console.warn(`[FallbackManager] Initial fetch failed for ${key}:`, e));
      const interval = setInterval(() => {
        fetchFn().catch(e => console.warn(`[FallbackManager] Polling failed for ${key}:`, e));
      }, 60000);
      this.intervals.set(key, interval);
    });

    console.log('[FallbackManager] Started 60s polling for all critical data');
  }

  stop(): void {
    if (!this.isActive) return;
    this.isActive = false;
    this.notifyChange();

    this.intervals.forEach((interval) => clearInterval(interval));
    this.intervals.clear();

    console.log('[FallbackManager] Stopped fallback polling');
  }

  async syncAll(): Promise<void> {
    const promises = Array.from(this.fetchFunctions.values()).map(fn =>
      fn().catch(e => console.warn('[FallbackManager] Sync failed:', e))
    );
    await Promise.all(promises);
  }

  getIsActive(): boolean {
    return this.isActive;
  }
}

export const fallbackManager = new FallbackPollingManager();
