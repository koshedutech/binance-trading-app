// Package startup provides initialization routines for the application.
// Epic 7: Client Order ID & Trade Lifecycle Tracking
// Story 7.20: Order Chain Redis Cache Layer
package startup

import (
	"context"
	"time"

	"binance-trading-bot/internal/cache"
	"binance-trading-bot/internal/orders"
)

// OrderChainCacheDB defines the database interface for cache warming
type OrderChainCacheDB interface {
	GetUsersWithActiveChains(ctx context.Context) ([]string, error)
	GetActiveOrderChains(ctx context.Context, userID string) ([]*orders.OrderChain, error)
}

// Logger interface for dependency injection (matches cache.Logger)
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// CacheWarmer warms the Redis cache on application startup
type CacheWarmer struct {
	db     OrderChainCacheDB
	cache  *cache.OrderChainCache
	logger Logger
}

// NewCacheWarmer creates a new CacheWarmer instance
func NewCacheWarmer(db OrderChainCacheDB, cache *cache.OrderChainCache, logger Logger) *CacheWarmer {
	return &CacheWarmer{
		db:     db,
		cache:  cache,
		logger: logger,
	}
}

// WarmOrderChainCache loads all active order chains into Redis cache.
// Called during application startup to pre-populate the cache.
func (w *CacheWarmer) WarmOrderChainCache(ctx context.Context) error {
	startTime := time.Now()
	w.logger.Info("Starting order chain cache warm-up")

	if w.cache == nil || !w.cache.IsHealthy() {
		w.logger.Warn("Cache unavailable, skipping warm-up")
		return nil
	}

	// Get all users with active chains
	users, err := w.db.GetUsersWithActiveChains(ctx)
	if err != nil {
		w.logger.Error("Failed to get users with active chains", "error", err)
		return err
	}

	if len(users) == 0 {
		w.logger.Info("No users with active chains found, nothing to warm")
		return nil
	}

	totalChains := 0
	failedUsers := 0

	for _, userID := range users {
		// Get active chains for this user
		chains, err := w.db.GetActiveOrderChains(ctx, userID)
		if err != nil {
			w.logger.Warn("Failed to get active chains for user", "user_id", userID, "error", err)
			failedUsers++
			continue
		}

		if len(chains) == 0 {
			continue
		}

		// Convert to cache state format
		cacheStates := make([]*cache.OrderChainState, 0, len(chains))
		for _, chain := range chains {
			state := ConvertChainToCacheState(chain)
			cacheStates = append(cacheStates, state)
		}

		// Warm cache for this user
		if err := w.cache.WarmCacheForUser(ctx, userID, cacheStates); err != nil {
			w.logger.Warn("Failed to warm cache for user", "user_id", userID, "error", err)
			failedUsers++
			continue
		}

		totalChains += len(chains)
	}

	elapsed := time.Since(startTime)
	w.logger.Info("Order chain cache warm-up complete",
		"total_users", len(users),
		"total_chains", totalChains,
		"failed_users", failedUsers,
		"duration_ms", elapsed.Milliseconds())

	return nil
}

// ConvertChainToCacheState converts a database OrderChain to cache OrderChainState
func ConvertChainToCacheState(chain *orders.OrderChain) *cache.OrderChainState {
	state := &cache.OrderChainState{
		ChainID:            chain.ChainID,
		UserID:             chain.UserID,
		Symbol:             chain.Symbol,
		Side:               chain.Side,
		ModeCode:           chain.ModeCode,
		Status:             string(chain.Status),
		PositionOptEnabled: chain.PositionOptEnabled,
		IsHedge:            chain.IsHedge,
		HedgeChainID:       chain.HedgeChainID,
		ParentChainID:      chain.ParentChainID,
		EventCount:         chain.EventCount,
		LastEventSeq:       chain.LastEventSeq,
		RealizedPnL:        chain.RealizedPnL,
		TotalFees:          chain.TotalFees,
		CreatedAt:          cache.FormatTimestamp(chain.CreatedAt),
		UpdatedAt:          cache.FormatTimestamp(chain.UpdatedAt),
		ClosedAt:           cache.FormatTimestampPtr(chain.ClosedAt),
	}

	// Entry details
	if chain.EntryPrice != nil && chain.EntryQuantity != nil {
		entryStatus := "PENDING"
		if chain.EntryFilledAt != nil {
			entryStatus = "FILLED"
		}
		state.Entry = &cache.OrderChainEntry{
			Price:          *chain.EntryPrice,
			Quantity:       *chain.EntryQuantity,
			Status:         entryStatus,
			FilledAt:       cache.FormatTimestampPtr(chain.EntryFilledAt),
			BinanceOrderID: chain.EntryBinanceOrderID,
		}
	}

	// Position state
	if chain.Status == orders.OrderChainStatusActive || chain.Status == orders.OrderChainStatusPartial {
		if chain.EntryPrice != nil && chain.EntryQuantity != nil {
			remainingQty := float64(0)
			if chain.RemainingQuantity != nil {
				remainingQty = *chain.RemainingQuantity
			}
			state.Position = &cache.OrderChainPosition{
				Status:            string(chain.Status),
				EntryPrice:        *chain.EntryPrice,
				Quantity:          *chain.EntryQuantity,
				RemainingQuantity: remainingQty,
			}
		}
	}

	// Stop Loss state
	if chain.CurrentSLPrice != nil {
		state.StopLoss = &cache.OrderChainStopLoss{
			CurrentPrice:      *chain.CurrentSLPrice,
			ModificationCount: chain.SLModificationCount,
		}
	}

	// Take Profit state
	if chain.PositionOptEnabled {
		// Position optimization TPs
		if chain.CurrentTP1Price != nil {
			state.TP1 = &cache.OrderChainTPLevel{
				Price:  *chain.CurrentTP1Price,
				Status: "PLACED",
			}
		}
		if chain.CurrentTP2Price != nil {
			state.TP2 = &cache.OrderChainTPLevel{
				Price:  *chain.CurrentTP2Price,
				Status: "PLACED",
			}
		}
		if chain.CurrentTP3Price != nil {
			state.TP3 = &cache.OrderChainTPLevel{
				Price:  *chain.CurrentTP3Price,
				Status: "PLACED",
			}
		}
	} else if chain.CurrentTPPrice != nil {
		// Normal mode TP
		state.TakeProfit = &cache.OrderChainTakeProfit{
			Mode:              "NORMAL",
			CurrentPrice:      *chain.CurrentTPPrice,
			ModificationCount: chain.TPModificationCount,
		}
	}

	return state
}

// WarmOrderChainCacheFunc is a standalone function for warming the cache.
// Useful when you don't need the full CacheWarmer struct.
func WarmOrderChainCacheFunc(ctx context.Context, db OrderChainCacheDB, chainCache *cache.OrderChainCache, logger Logger) error {
	warmer := NewCacheWarmer(db, chainCache, logger)
	return warmer.WarmOrderChainCache(ctx)
}
