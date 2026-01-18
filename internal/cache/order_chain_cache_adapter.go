// Package cache provides Redis-based caching for order chains.
// Epic 7: Client Order ID & Trade Lifecycle Tracking
// Story 7.20: Order Chain Redis Cache Layer
//
// This file provides an adapter that implements the orders.ChainCacheInterface
// using the OrderChainCache service, bridging the orders and cache packages.
package cache

import (
	"context"

	"binance-trading-bot/internal/orders"
)

// OrderChainCacheAdapter adapts OrderChainCache to implement orders.ChainCacheInterface
type OrderChainCacheAdapter struct {
	cache *OrderChainCache
}

// NewOrderChainCacheAdapter creates a new adapter wrapping the OrderChainCache
func NewOrderChainCacheAdapter(cache *OrderChainCache) *OrderChainCacheAdapter {
	return &OrderChainCacheAdapter{cache: cache}
}

// SetChainState implements orders.ChainCacheInterface
func (a *OrderChainCacheAdapter) SetChainState(ctx context.Context, chain *orders.ChainCacheState) error {
	// Convert orders.ChainCacheState to cache.OrderChainState
	cacheState := convertToCacheOrderChainState(chain)
	return a.cache.SetChainState(ctx, cacheState)
}

// DeleteChainState implements orders.ChainCacheInterface
func (a *OrderChainCacheAdapter) DeleteChainState(ctx context.Context, userID, chainID string) error {
	return a.cache.DeleteChainState(ctx, userID, chainID)
}

// AddToActiveChains implements orders.ChainCacheInterface
func (a *OrderChainCacheAdapter) AddToActiveChains(ctx context.Context, userID, chainID string) error {
	return a.cache.AddToActiveChains(ctx, userID, chainID)
}

// RemoveFromActiveChains implements orders.ChainCacheInterface
func (a *OrderChainCacheAdapter) RemoveFromActiveChains(ctx context.Context, userID, chainID string) error {
	return a.cache.RemoveFromActiveChains(ctx, userID, chainID)
}

// PushRecentEvent implements orders.ChainCacheInterface
func (a *OrderChainCacheAdapter) PushRecentEvent(ctx context.Context, userID string, event *orders.ChainCacheEventSummary) error {
	// Convert orders.ChainCacheEventSummary to cache.ChainEventSummary
	cacheEvent := &ChainEventSummary{
		ChainID:       event.ChainID,
		EventType:     event.EventType,
		OrderType:     event.OrderType,
		Price:         event.Price,
		OldPrice:      event.OldPrice,
		Quantity:      event.Quantity,
		PnL:           event.PnL,
		Timestamp:     event.Timestamp,
		EventSequence: event.EventSequence,
	}
	return a.cache.PushRecentEvent(ctx, userID, cacheEvent)
}

// convertToCacheOrderChainState converts orders.ChainCacheState to cache.OrderChainState
func convertToCacheOrderChainState(src *orders.ChainCacheState) *OrderChainState {
	state := &OrderChainState{
		ChainID:            src.ChainID,
		UserID:             src.UserID,
		Symbol:             src.Symbol,
		Side:               src.Side,
		ModeCode:           src.ModeCode,
		Status:             src.Status,
		PositionOptEnabled: src.PositionOptEnabled,
		IsHedge:            src.IsHedge,
		HedgeChainID:       src.HedgeChainID,
		ParentChainID:      src.ParentChainID,
		EventCount:         src.EventCount,
		LastEventSeq:       src.LastEventSeq,
		RealizedPnL:        src.RealizedPnL,
		TotalFees:          src.TotalFees,
		CreatedAt:          src.CreatedAt,
		UpdatedAt:          src.UpdatedAt,
		ClosedAt:           src.ClosedAt,
	}

	// Entry
	if src.EntryPrice != nil {
		state.Entry = &OrderChainEntry{
			Price:          *src.EntryPrice,
			Status:         src.EntryStatus,
			FilledAt:       src.EntryFilledAt,
			BinanceOrderID: src.EntryBinanceOrderID,
		}
		if src.EntryQuantity != nil {
			state.Entry.Quantity = *src.EntryQuantity
		}
	}

	// Position
	if src.PositionStatus != "" && src.PositionStatus != "CLOSED" {
		state.Position = &OrderChainPosition{
			Status:            src.PositionStatus,
			EntryPrice:        src.PositionEntryPrice,
			Quantity:          src.PositionQuantity,
			RemainingQuantity: src.PositionRemainingQuantity,
		}
	}

	// Stop Loss
	if src.SLCurrentPrice != nil {
		state.StopLoss = &OrderChainStopLoss{
			CurrentPrice:      *src.SLCurrentPrice,
			BinanceOrderID:    src.SLBinanceOrderID,
			ModificationCount: src.SLModificationCount,
			LastModified:      src.SLLastModified,
		}
	}

	// Take Profit
	if src.TPMode == "POSITION_OPT" {
		if src.TP1Price != nil {
			state.TP1 = &OrderChainTPLevel{
				Price:          *src.TP1Price,
				Status:         src.TP1Status,
				BinanceOrderID: src.TP1BinanceOrderID,
			}
			if src.TP1Quantity != nil {
				state.TP1.Quantity = *src.TP1Quantity
			}
		}
		if src.TP2Price != nil {
			state.TP2 = &OrderChainTPLevel{
				Price:          *src.TP2Price,
				Status:         src.TP2Status,
				BinanceOrderID: src.TP2BinanceOrderID,
			}
			if src.TP2Quantity != nil {
				state.TP2.Quantity = *src.TP2Quantity
			}
		}
		if src.TP3Price != nil {
			state.TP3 = &OrderChainTPLevel{
				Price:          *src.TP3Price,
				Status:         src.TP3Status,
				BinanceOrderID: src.TP3BinanceOrderID,
			}
			if src.TP3Quantity != nil {
				state.TP3.Quantity = *src.TP3Quantity
			}
		}
	} else if src.TPCurrentPrice != nil {
		state.TakeProfit = &OrderChainTakeProfit{
			Mode:              "NORMAL",
			CurrentPrice:      *src.TPCurrentPrice,
			BinanceOrderID:    src.TPBinanceOrderID,
			ModificationCount: src.TPModificationCount,
			LastModified:      src.TPLastModified,
		}
	}

	return state
}
