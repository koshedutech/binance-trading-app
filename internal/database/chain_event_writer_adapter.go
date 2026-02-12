package database

import (
	"binance-trading-bot/internal/orders"
	"context"
	"time"
)

// ChainEventWriterDBAdapter adapts DB to orders.ChainEventWriterDB interface
type ChainEventWriterDBAdapter struct {
	db *DB
}

// NewChainEventWriterDBAdapter creates a new adapter
func NewChainEventWriterDBAdapter(db *DB) *ChainEventWriterDBAdapter {
	return &ChainEventWriterDBAdapter{db: db}
}

// ========== Order Chain Operations ==========

// CreateOrderChain implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) CreateOrderChain(ctx context.Context, chain *orders.OrderChain) error {
	return a.db.CreateOrderChain(ctx, chain)
}

// UpdateOrderChain implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) UpdateOrderChain(ctx context.Context, chain *orders.OrderChain) error {
	return a.db.UpdateOrderChain(ctx, chain)
}

// GetOrderChainByID implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) GetOrderChainByID(ctx context.Context, userID, chainID string) (*orders.OrderChain, error) {
	return a.db.GetOrderChainByID(ctx, userID, chainID)
}

// GetOrderChainByChainIDOnly implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) GetOrderChainByChainIDOnly(ctx context.Context, chainID string) (*orders.OrderChain, error) {
	return a.db.GetOrderChainByChainIDOnly(ctx, chainID)
}

// GetActiveOrderChains implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) GetActiveOrderChains(ctx context.Context, userID string) ([]*orders.OrderChain, error) {
	return a.db.GetActiveOrderChains(ctx, userID)
}

// GetOpenOrderChains implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) GetOpenOrderChains(ctx context.Context, userID string) ([]*orders.OrderChain, error) {
	return a.db.GetOpenOrderChains(ctx, userID)
}

// CloseOrderChain implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) CloseOrderChain(ctx context.Context, chainID string, closeReason string, realizedPnL, totalFees float64, closePrice *float64) error {
	return a.db.CloseOrderChain(ctx, chainID, closeReason, realizedPnL, totalFees, closePrice)
}

// ReactivateOrderChain implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) ReactivateOrderChain(ctx context.Context, chainID string) error {
	return a.db.ReactivateOrderChain(ctx, chainID)
}

// UpdateOrderChainSLPrice implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) UpdateOrderChainSLPrice(ctx context.Context, chainID string, newPrice float64) error {
	return a.db.UpdateOrderChainSLPrice(ctx, chainID, newPrice)
}

// UpdateOrderChainTPPrice implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) UpdateOrderChainTPPrice(ctx context.Context, chainID string, newPrice float64) error {
	return a.db.UpdateOrderChainTPPrice(ctx, chainID, newPrice)
}

// LinkHedgeChain implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) LinkHedgeChain(ctx context.Context, primaryChainID, hedgeChainID string) error {
	return a.db.LinkHedgeChain(ctx, primaryChainID, hedgeChainID)
}

// IncrementOrderChainEventCount implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) IncrementOrderChainEventCount(ctx context.Context, chainID string, newSeq int) error {
	return a.db.IncrementOrderChainEventCount(ctx, chainID, newSeq)
}

// UpdateOrderChainSLDetails implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) UpdateOrderChainSLDetails(ctx context.Context, chainID string, binanceOrderID int64, limitPrice float64, quantity float64) error {
	return a.db.UpdateOrderChainSLDetails(ctx, chainID, binanceOrderID, limitPrice, quantity)
}

// UpdateOrderChainTPDetails implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) UpdateOrderChainTPDetails(ctx context.Context, chainID string, binanceOrderID int64, limitPrice float64, quantity float64) error {
	return a.db.UpdateOrderChainTPDetails(ctx, chainID, binanceOrderID, limitPrice, quantity)
}

// UpdateOrderChainSLFilled implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) UpdateOrderChainSLFilled(ctx context.Context, chainID string, fillPrice float64, fillTime time.Time) error {
	return a.db.UpdateOrderChainSLFilled(ctx, chainID, fillPrice, fillTime)
}

// UpdateOrderChainTPFilled implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) UpdateOrderChainTPFilled(ctx context.Context, chainID string, fillPrice float64, fillTime time.Time) error {
	return a.db.UpdateOrderChainTPFilled(ctx, chainID, fillPrice, fillTime)
}

// ========== PnL Correction Operations ==========

// UpdateClosedChainPnL implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) UpdateClosedChainPnL(ctx context.Context, chainID string, realizedPnL float64, totalFees float64) error {
	return a.db.UpdateClosedChainPnL(ctx, chainID, realizedPnL, totalFees)
}

// ========== Chain Event Operations ==========

// InsertChainEvent implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) InsertChainEvent(ctx context.Context, event *orders.ChainEvent) error {
	return a.db.InsertChainEvent(ctx, event)
}

// GetChainEvents implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) GetChainEvents(ctx context.Context, chainID string) ([]*orders.ChainEvent, error) {
	return a.db.GetChainEvents(ctx, chainID)
}

// GetNextEventSequence implements ChainEventWriterDB
func (a *ChainEventWriterDBAdapter) GetNextEventSequence(ctx context.Context, chainID string) (int, error) {
	return a.db.GetNextEventSequence(ctx, chainID)
}
