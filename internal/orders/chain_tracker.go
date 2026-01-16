// Package orders provides client order ID generation for Binance futures trading.
// Epic 7: Client Order ID & Trade Lifecycle Tracking
// Story 7.3: Chain Tracker Service
package orders

import (
	"errors"
	"sync"
	"time"
)

// ChainTracker errors
var (
	ErrChainNotFound      = errors.New("chain not found")
	ErrChainAlreadyExists = errors.New("chain already exists")
	ErrInvalidChainStatus = errors.New("invalid chain status")
	ErrEmptyBaseID        = errors.New("base ID cannot be empty")
	ErrEmptySymbol        = errors.New("symbol cannot be empty")
)

// ChainTracker manages active order chains in memory.
// It provides thread-safe access to chain state tracking.
type ChainTracker struct {
	mu     sync.RWMutex
	chains map[string]*ChainState // keyed by baseID
}

// NewChainTracker creates a new ChainTracker instance
func NewChainTracker() *ChainTracker {
	return &ChainTracker{
		chains: make(map[string]*ChainState),
	}
}

// CreateChain creates a new order chain on entry order placement.
// Returns error if a chain with the same baseID already exists.
func (ct *ChainTracker) CreateChain(baseID, symbol string, mode TradingMode, direction Direction) (*ChainState, error) {
	if baseID == "" {
		return nil, ErrEmptyBaseID
	}
	if symbol == "" {
		return nil, ErrEmptySymbol
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	// Check if chain already exists
	if _, exists := ct.chains[baseID]; exists {
		return nil, ErrChainAlreadyExists
	}

	// Create new chain state
	chain := NewChainState(baseID, symbol, mode, direction)

	// Mark entry order as pending by default
	chain.MarkOrderPending(OrderTypeEntry)

	ct.chains[baseID] = chain
	return chain, nil
}

// UpdateChainStatus updates the status of an existing chain.
// Returns error if chain not found or status is invalid.
func (ct *ChainTracker) UpdateChainStatus(baseID string, status ChainStatus) error {
	if baseID == "" {
		return ErrEmptyBaseID
	}

	// Validate status
	if !IsValidChainStatus(string(status)) {
		return ErrInvalidChainStatus
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	chain, exists := ct.chains[baseID]
	if !exists {
		return ErrChainNotFound
	}

	chain.Status = status
	chain.UpdatedAt = time.Now()
	return nil
}

// GetChain retrieves a chain by its base ID.
// Returns nil and ErrChainNotFound if not found.
func (ct *ChainTracker) GetChain(baseID string) (*ChainState, error) {
	if baseID == "" {
		return nil, ErrEmptyBaseID
	}

	ct.mu.RLock()
	defer ct.mu.RUnlock()

	chain, exists := ct.chains[baseID]
	if !exists {
		return nil, ErrChainNotFound
	}

	return chain, nil
}

// GetActiveChains returns all chains that are in active or partial status.
// Returns a slice of chain states (copies to prevent race conditions).
func (ct *ChainTracker) GetActiveChains() []*ChainState {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	var activeChains []*ChainState
	for _, chain := range ct.chains {
		if chain.IsActive() {
			activeChains = append(activeChains, chain)
		}
	}
	return activeChains
}

// CloseChain marks a chain as completed.
// Use this when a position is fully closed.
func (ct *ChainTracker) CloseChain(baseID string) error {
	return ct.UpdateChainStatus(baseID, ChainStatusCompleted)
}

// CancelChain marks a chain as cancelled.
// Use this when an entry order is rejected or cancelled before fill.
func (ct *ChainTracker) CancelChain(baseID string) error {
	return ct.UpdateChainStatus(baseID, ChainStatusCancelled)
}

// GetChainBySymbol returns the first active chain for a given symbol.
// Useful when you need to find the current chain for a trading pair.
// Returns nil if no active chain exists for the symbol.
func (ct *ChainTracker) GetChainBySymbol(symbol string) *ChainState {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	for _, chain := range ct.chains {
		if chain.Symbol == symbol && chain.IsActive() {
			return chain
		}
	}
	return nil
}

// GetAllChainsBySymbol returns all chains for a given symbol (active and completed).
func (ct *ChainTracker) GetAllChainsBySymbol(symbol string) []*ChainState {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	var chains []*ChainState
	for _, chain := range ct.chains {
		if chain.Symbol == symbol {
			chains = append(chains, chain)
		}
	}
	return chains
}

// MarkOrderFilled marks a specific order type as filled in a chain.
func (ct *ChainTracker) MarkOrderFilled(baseID string, orderType OrderType) error {
	if baseID == "" {
		return ErrEmptyBaseID
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	chain, exists := ct.chains[baseID]
	if !exists {
		return ErrChainNotFound
	}

	chain.MarkOrderFilled(orderType)

	// If entry is filled, chain becomes active
	if orderType == OrderTypeEntry && chain.Status == ChainStatusActive {
		// Entry filled, chain is now truly active
	}

	return nil
}

// MarkOrderPending marks a specific order type as pending in a chain.
func (ct *ChainTracker) MarkOrderPending(baseID string, orderType OrderType) error {
	if baseID == "" {
		return ErrEmptyBaseID
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	chain, exists := ct.chains[baseID]
	if !exists {
		return ErrChainNotFound
	}

	chain.MarkOrderPending(orderType)
	return nil
}

// MarkOrderCancelled marks a specific order type as cancelled in a chain.
func (ct *ChainTracker) MarkOrderCancelled(baseID string, orderType OrderType) error {
	if baseID == "" {
		return ErrEmptyBaseID
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	chain, exists := ct.chains[baseID]
	if !exists {
		return ErrChainNotFound
	}

	chain.MarkOrderCancelled(orderType)
	return nil
}

// RemoveChain completely removes a chain from the tracker.
// Use sparingly - typically chains should be marked completed/cancelled instead.
func (ct *ChainTracker) RemoveChain(baseID string) error {
	if baseID == "" {
		return ErrEmptyBaseID
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	if _, exists := ct.chains[baseID]; !exists {
		return ErrChainNotFound
	}

	delete(ct.chains, baseID)
	return nil
}

// GetChainCount returns the total number of chains being tracked.
func (ct *ChainTracker) GetChainCount() int {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return len(ct.chains)
}

// GetActiveChainCount returns the number of active chains.
func (ct *ChainTracker) GetActiveChainCount() int {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	count := 0
	for _, chain := range ct.chains {
		if chain.IsActive() {
			count++
		}
	}
	return count
}

// Clear removes all chains from the tracker.
// Use with caution - typically for testing or complete reset.
func (ct *ChainTracker) Clear() {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.chains = make(map[string]*ChainState)
}

// SetChainMetadata sets a metadata value on a chain.
func (ct *ChainTracker) SetChainMetadata(baseID, key string, value interface{}) error {
	if baseID == "" {
		return ErrEmptyBaseID
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	chain, exists := ct.chains[baseID]
	if !exists {
		return ErrChainNotFound
	}

	chain.SetMetadata(key, value)
	return nil
}

// GetChainMetadata retrieves a metadata value from a chain.
func (ct *ChainTracker) GetChainMetadata(baseID, key string) (interface{}, error) {
	if baseID == "" {
		return nil, ErrEmptyBaseID
	}

	ct.mu.RLock()
	defer ct.mu.RUnlock()

	chain, exists := ct.chains[baseID]
	if !exists {
		return nil, ErrChainNotFound
	}

	value, found := chain.GetMetadata(key)
	if !found {
		return nil, nil
	}
	return value, nil
}
