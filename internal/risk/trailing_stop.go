package risk

import (
	"log"
	"sync"
	"time"
)

// TrailingStopManager manages trailing stop losses for positions
type TrailingStopManager struct {
	positions map[string]*TrailingPosition
	config    *TrailingConfig
	mu        sync.RWMutex
}

// TrailingConfig holds trailing stop configuration
type TrailingConfig struct {
	Enabled            bool    // Enable trailing stops
	TrailingPercent    float64 // Distance from high water mark
	ActivationPercent  float64 // Profit % to activate trailing
	UseATRMultiplier   bool    // Use ATR-based trailing distance
	ATRMultiplier      float64 // ATR multiplier for trailing distance
}

// TrailingPosition tracks a position with trailing stop
type TrailingPosition struct {
	Symbol           string
	Side             string    // "BUY" or "SELL"
	EntryPrice       float64
	CurrentStopLoss  float64
	OriginalStopLoss float64
	HighWaterMark    float64   // Highest price since entry (for longs)
	LowWaterMark     float64   // Lowest price since entry (for shorts)
	IsActivated      bool      // Whether trailing stop is active
	LastUpdate       time.Time
}

// NewTrailingStopManager creates a new trailing stop manager
func NewTrailingStopManager(config *TrailingConfig) *TrailingStopManager {
	return &TrailingStopManager{
		positions: make(map[string]*TrailingPosition),
		config:    config,
	}
}

// AddPosition adds a new position to track
func (tsm *TrailingStopManager) AddPosition(symbol, side string, entryPrice, stopLoss float64) {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	tsm.positions[symbol] = &TrailingPosition{
		Symbol:           symbol,
		Side:             side,
		EntryPrice:       entryPrice,
		CurrentStopLoss:  stopLoss,
		OriginalStopLoss: stopLoss,
		HighWaterMark:    entryPrice,
		LowWaterMark:     entryPrice,
		IsActivated:      false,
		LastUpdate:       time.Now(),
	}

	log.Printf("[TrailingStop] Position added: %s %s @ %.4f, SL: %.4f", side, symbol, entryPrice, stopLoss)
}

// RemovePosition removes a position from tracking
func (tsm *TrailingStopManager) RemovePosition(symbol string) {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	delete(tsm.positions, symbol)
	log.Printf("[TrailingStop] Position removed: %s", symbol)
}

// UpdatePrice updates the current price and adjusts trailing stop if needed
func (tsm *TrailingStopManager) UpdatePrice(symbol string, currentPrice float64) *StopUpdate {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	pos, exists := tsm.positions[symbol]
	if !exists {
		return nil
	}

	var update *StopUpdate

	if pos.Side == "BUY" {
		update = tsm.updateLongPosition(pos, currentPrice)
	} else {
		update = tsm.updateShortPosition(pos, currentPrice)
	}

	pos.LastUpdate = time.Now()
	return update
}

// StopUpdate represents a stop loss update
type StopUpdate struct {
	Symbol       string
	OldStopLoss  float64
	NewStopLoss  float64
	IsTriggered  bool
	TriggerPrice float64
}

// updateLongPosition updates trailing stop for a long position
func (tsm *TrailingStopManager) updateLongPosition(pos *TrailingPosition, currentPrice float64) *StopUpdate {
	// Check if stop loss is triggered
	if currentPrice <= pos.CurrentStopLoss {
		return &StopUpdate{
			Symbol:       pos.Symbol,
			OldStopLoss:  pos.CurrentStopLoss,
			NewStopLoss:  pos.CurrentStopLoss,
			IsTriggered:  true,
			TriggerPrice: currentPrice,
		}
	}

	// Update high water mark
	if currentPrice > pos.HighWaterMark {
		pos.HighWaterMark = currentPrice
	}

	// Check if trailing should be activated
	profitPercent := ((currentPrice - pos.EntryPrice) / pos.EntryPrice) * 100
	if !pos.IsActivated && profitPercent >= tsm.config.ActivationPercent {
		pos.IsActivated = true
		log.Printf("[TrailingStop] Activated for %s at %.2f%% profit", pos.Symbol, profitPercent)
	}

	// Calculate new trailing stop if activated
	if pos.IsActivated && tsm.config.Enabled {
		trailingDistance := pos.HighWaterMark * (tsm.config.TrailingPercent / 100)
		newStopLoss := pos.HighWaterMark - trailingDistance

		// Only move stop loss up, never down
		if newStopLoss > pos.CurrentStopLoss {
			oldStop := pos.CurrentStopLoss
			pos.CurrentStopLoss = newStopLoss

			log.Printf("[TrailingStop] %s: SL moved up %.4f -> %.4f (HWM: %.4f, Profit: %.2f%%)",
				pos.Symbol, oldStop, newStopLoss, pos.HighWaterMark, profitPercent)

			return &StopUpdate{
				Symbol:      pos.Symbol,
				OldStopLoss: oldStop,
				NewStopLoss: newStopLoss,
				IsTriggered: false,
			}
		}
	}

	return nil
}

// updateShortPosition updates trailing stop for a short position
func (tsm *TrailingStopManager) updateShortPosition(pos *TrailingPosition, currentPrice float64) *StopUpdate {
	// Check if stop loss is triggered
	if currentPrice >= pos.CurrentStopLoss {
		return &StopUpdate{
			Symbol:       pos.Symbol,
			OldStopLoss:  pos.CurrentStopLoss,
			NewStopLoss:  pos.CurrentStopLoss,
			IsTriggered:  true,
			TriggerPrice: currentPrice,
		}
	}

	// Update low water mark
	if currentPrice < pos.LowWaterMark {
		pos.LowWaterMark = currentPrice
	}

	// Check if trailing should be activated
	profitPercent := ((pos.EntryPrice - currentPrice) / pos.EntryPrice) * 100
	if !pos.IsActivated && profitPercent >= tsm.config.ActivationPercent {
		pos.IsActivated = true
		log.Printf("[TrailingStop] Activated for %s SHORT at %.2f%% profit", pos.Symbol, profitPercent)
	}

	// Calculate new trailing stop if activated
	if pos.IsActivated && tsm.config.Enabled {
		trailingDistance := pos.LowWaterMark * (tsm.config.TrailingPercent / 100)
		newStopLoss := pos.LowWaterMark + trailingDistance

		// Only move stop loss down for shorts
		if newStopLoss < pos.CurrentStopLoss {
			oldStop := pos.CurrentStopLoss
			pos.CurrentStopLoss = newStopLoss

			log.Printf("[TrailingStop] %s SHORT: SL moved down %.4f -> %.4f (LWM: %.4f, Profit: %.2f%%)",
				pos.Symbol, oldStop, newStopLoss, pos.LowWaterMark, profitPercent)

			return &StopUpdate{
				Symbol:      pos.Symbol,
				OldStopLoss: oldStop,
				NewStopLoss: newStopLoss,
				IsTriggered: false,
			}
		}
	}

	return nil
}

// GetPosition returns a position's trailing stop info
func (tsm *TrailingStopManager) GetPosition(symbol string) *TrailingPosition {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()

	if pos, exists := tsm.positions[symbol]; exists {
		// Return a copy
		return &TrailingPosition{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			EntryPrice:       pos.EntryPrice,
			CurrentStopLoss:  pos.CurrentStopLoss,
			OriginalStopLoss: pos.OriginalStopLoss,
			HighWaterMark:    pos.HighWaterMark,
			LowWaterMark:     pos.LowWaterMark,
			IsActivated:      pos.IsActivated,
			LastUpdate:       pos.LastUpdate,
		}
	}
	return nil
}

// GetAllPositions returns all tracked positions
func (tsm *TrailingStopManager) GetAllPositions() []*TrailingPosition {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()

	positions := make([]*TrailingPosition, 0, len(tsm.positions))
	for _, pos := range tsm.positions {
		positions = append(positions, &TrailingPosition{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			EntryPrice:       pos.EntryPrice,
			CurrentStopLoss:  pos.CurrentStopLoss,
			OriginalStopLoss: pos.OriginalStopLoss,
			HighWaterMark:    pos.HighWaterMark,
			LowWaterMark:     pos.LowWaterMark,
			IsActivated:      pos.IsActivated,
			LastUpdate:       pos.LastUpdate,
		})
	}
	return positions
}

// GetCurrentStopLoss returns the current stop loss for a symbol
func (tsm *TrailingStopManager) GetCurrentStopLoss(symbol string) (float64, bool) {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()

	if pos, exists := tsm.positions[symbol]; exists {
		return pos.CurrentStopLoss, true
	}
	return 0, false
}
