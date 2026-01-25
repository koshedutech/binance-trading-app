package autopilot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"binance-trading-bot/internal/database"
)

// TradingStateCache interface for caching trading state
// This avoids import cycle with cache package
type TradingStateCache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	IsHealthy() bool
}

// ==================== STORY 14.14: Trading Controller ====================
// Trading ON/OFF toggle for Chain Trading System
// Controls Entry Decision and Exit Decision independently of Ginie Autopilot
// =========================================================================

// TradingState represents the current trading state for a user
type TradingState struct {
	Enabled        bool      `json:"enabled"`         // Master trading switch
	EntryActive    bool      `json:"entry_active"`    // Entry Decision active (new entries allowed)
	ExitActive     bool      `json:"exit_active"`     // Exit Decision active (always true when positions exist)
	LastChanged    time.Time `json:"last_changed"`    // When the state was last changed
	ChangedBy      string    `json:"changed_by"`      // "user" or "system"
	Reason         string    `json:"reason,omitempty"` // Optional reason for state change
}

// TradingController interface for managing trading ON/OFF state
type TradingController interface {
	// IsTradingEnabled returns true if trading is enabled for the user
	IsTradingEnabled(ctx context.Context, userID string) (bool, error)

	// SetTradingEnabled enables or disables trading for the user
	SetTradingEnabled(ctx context.Context, userID string, enabled bool, changedBy string, reason string) error

	// GetTradingState returns the complete trading state for the user
	GetTradingState(ctx context.Context, userID string) (*TradingState, error)

	// IsEntryAllowed returns true if new entries are allowed (trading enabled)
	IsEntryAllowed(ctx context.Context, userID string) (bool, error)

	// IsExitAllowed returns true if exits are allowed (always true when positions exist)
	IsExitAllowed(ctx context.Context, userID string) (bool, error)

	// InvalidateCache invalidates the cached trading state for the user
	InvalidateCache(ctx context.Context, userID string) error
}

// DefaultTradingController implements TradingController with database + Redis caching
type DefaultTradingController struct {
	repo      *database.Repository
	cache     TradingStateCache
	mu        sync.RWMutex

	// In-memory cache for fast access during trading loops
	memCache  map[string]*TradingState
	memTTL    time.Duration
}

// Redis cache key prefix and TTL
const (
	TradingStateCachePrefix = "user:%s:trading_state"
	TradingStateCacheTTL    = 5 * time.Minute
)

// NewDefaultTradingController creates a new DefaultTradingController
func NewDefaultTradingController(repo *database.Repository, cacheService TradingStateCache) *DefaultTradingController {
	return &DefaultTradingController{
		repo:     repo,
		cache:    cacheService,
		memCache: make(map[string]*TradingState),
		memTTL:   30 * time.Second, // Fast in-memory cache for hot path
	}
}

// IsTradingEnabled returns true if trading is enabled for the user
func (c *DefaultTradingController) IsTradingEnabled(ctx context.Context, userID string) (bool, error) {
	state, err := c.GetTradingState(ctx, userID)
	if err != nil {
		return false, err
	}
	return state.Enabled, nil
}

// SetTradingEnabled enables or disables trading for the user
func (c *DefaultTradingController) SetTradingEnabled(ctx context.Context, userID string, enabled bool, changedBy string, reason string) error {
	// Validate changedBy
	if changedBy != "user" && changedBy != "system" {
		changedBy = "user"
	}

	// Create the new state
	state := &TradingState{
		Enabled:     enabled,
		EntryActive: enabled, // Entry follows master switch
		ExitActive:  true,    // Exit is always active (managed by position existence)
		LastChanged: time.Now(),
		ChangedBy:   changedBy,
		Reason:      reason,
	}

	// Convert to database type
	dbState := &database.TradingStateData{
		Enabled:     state.Enabled,
		EntryActive: state.EntryActive,
		ExitActive:  state.ExitActive,
		LastChanged: state.LastChanged,
		ChangedBy:   state.ChangedBy,
		Reason:      state.Reason,
	}

	// Save to database
	if err := c.repo.SaveUserTradingState(ctx, userID, dbState); err != nil {
		return fmt.Errorf("failed to save trading state to database: %w", err)
	}

	// Update Redis cache
	if c.cache != nil && c.cache.IsHealthy() {
		cacheKey := fmt.Sprintf(TradingStateCachePrefix, userID)
		if err := c.cache.SetJSON(ctx, cacheKey, state, TradingStateCacheTTL); err != nil {
			log.Printf("[TRADING-CONTROLLER] Warning: Failed to update cache for user %s: %v", userID, err)
		}
	}

	// Update in-memory cache
	c.mu.Lock()
	c.memCache[userID] = state
	c.mu.Unlock()

	log.Printf("[TRADING-CONTROLLER] Trading state changed for user %s: enabled=%v, changedBy=%s, reason=%s",
		userID, enabled, changedBy, reason)

	return nil
}

// GetTradingState returns the complete trading state for the user
func (c *DefaultTradingController) GetTradingState(ctx context.Context, userID string) (*TradingState, error) {
	// Try in-memory cache first (fastest)
	c.mu.RLock()
	if state, ok := c.memCache[userID]; ok {
		c.mu.RUnlock()
		return state, nil
	}
	c.mu.RUnlock()

	// Try Redis cache
	if c.cache != nil && c.cache.IsHealthy() {
		cacheKey := fmt.Sprintf(TradingStateCachePrefix, userID)
		data, err := c.cache.Get(ctx, cacheKey)
		if err == nil && data != "" {
			var state TradingState
			if err := json.Unmarshal([]byte(data), &state); err == nil {
				// Update in-memory cache
				c.mu.Lock()
				c.memCache[userID] = &state
				c.mu.Unlock()
				return &state, nil
			}
		}
	}

	// Fall back to database
	dbState, err := c.repo.GetUserTradingState(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get trading state from database: %w", err)
	}

	// If not found, return default (enabled)
	var state *TradingState
	if dbState == nil {
		state = DefaultTradingState()
	} else {
		// Convert from database type
		state = &TradingState{
			Enabled:     dbState.Enabled,
			EntryActive: dbState.EntryActive,
			ExitActive:  dbState.ExitActive,
			LastChanged: dbState.LastChanged,
			ChangedBy:   dbState.ChangedBy,
			Reason:      dbState.Reason,
		}
	}

	// Update caches
	c.mu.Lock()
	c.memCache[userID] = state
	c.mu.Unlock()

	if c.cache != nil && c.cache.IsHealthy() {
		cacheKey := fmt.Sprintf(TradingStateCachePrefix, userID)
		if err := c.cache.SetJSON(ctx, cacheKey, state, TradingStateCacheTTL); err != nil {
			log.Printf("[TRADING-CONTROLLER] Warning: Failed to cache trading state for user %s: %v", userID, err)
		}
	}

	return state, nil
}

// IsEntryAllowed returns true if new entries are allowed
// When trading is OFF: Entry Decision is paused (no new entries)
// Coin Profiler and pattern calculations continue to run
func (c *DefaultTradingController) IsEntryAllowed(ctx context.Context, userID string) (bool, error) {
	state, err := c.GetTradingState(ctx, userID)
	if err != nil {
		return false, err
	}
	return state.EntryActive, nil
}

// IsExitAllowed returns true if exits are allowed
// Exit Decision is always active when positions exist
// This ensures positions are managed even when trading is OFF
func (c *DefaultTradingController) IsExitAllowed(ctx context.Context, userID string) (bool, error) {
	state, err := c.GetTradingState(ctx, userID)
	if err != nil {
		return false, err
	}
	return state.ExitActive, nil
}

// InvalidateCache invalidates the cached trading state for the user
func (c *DefaultTradingController) InvalidateCache(ctx context.Context, userID string) error {
	// Clear in-memory cache
	c.mu.Lock()
	delete(c.memCache, userID)
	c.mu.Unlock()

	// Clear Redis cache
	if c.cache != nil && c.cache.IsHealthy() {
		cacheKey := fmt.Sprintf(TradingStateCachePrefix, userID)
		if err := c.cache.Delete(ctx, cacheKey); err != nil {
			return fmt.Errorf("failed to invalidate cache: %w", err)
		}
	}

	return nil
}

// DefaultTradingState returns the default trading state (enabled)
func DefaultTradingState() *TradingState {
	return &TradingState{
		Enabled:     true,  // Trading ON by default
		EntryActive: true,  // Entries allowed
		ExitActive:  true,  // Exits always active
		LastChanged: time.Now(),
		ChangedBy:   "system",
		Reason:      "default",
	}
}

// ==================== GLOBAL INSTANCE MANAGEMENT ====================

var (
	globalTradingController TradingController
	tradingControllerMu     sync.RWMutex
)

// InitTradingController initializes the global trading controller
func InitTradingController(repo *database.Repository, cacheService TradingStateCache) {
	tradingControllerMu.Lock()
	defer tradingControllerMu.Unlock()

	globalTradingController = NewDefaultTradingController(repo, cacheService)
	log.Printf("[TRADING-CONTROLLER] Initialized global trading controller")
}

// GetTradingController returns the global trading controller
func GetTradingController() TradingController {
	tradingControllerMu.RLock()
	defer tradingControllerMu.RUnlock()
	return globalTradingController
}

// SetTradingController sets a custom trading controller (for testing)
func SetTradingController(controller TradingController) {
	tradingControllerMu.Lock()
	defer tradingControllerMu.Unlock()
	globalTradingController = controller
}
