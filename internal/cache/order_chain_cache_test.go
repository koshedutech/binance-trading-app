// Package cache provides Redis-based caching for order chains.
// Epic 7: Client Order ID & Trade Lifecycle Tracking
// Story 7.20: Order Chain Redis Cache Layer - Unit Tests
package cache

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestOrderChainKey tests the key generation function
func TestOrderChainKey(t *testing.T) {
	tests := []struct {
		name     string
		userID   string
		chainID  string
		expected string
	}{
		{
			name:     "standard key",
			userID:   "user-123",
			chainID:  "ULT-18JAN-00001",
			expected: "order_chain:user-123:ULT-18JAN-00001",
		},
		{
			name:     "empty user",
			userID:   "",
			chainID:  "ULT-18JAN-00001",
			expected: "order_chain::ULT-18JAN-00001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := OrderChainKey(tt.userID, tt.chainID)
			if result != tt.expected {
				t.Errorf("OrderChainKey(%s, %s) = %s, want %s", tt.userID, tt.chainID, result, tt.expected)
			}
		})
	}
}

// TestActiveChainsKey tests the active chains set key generation
func TestActiveChainsKey(t *testing.T) {
	tests := []struct {
		name     string
		userID   string
		expected string
	}{
		{
			name:     "standard key",
			userID:   "user-123",
			expected: "order_chains:active:user-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ActiveChainsKey(tt.userID)
			if result != tt.expected {
				t.Errorf("ActiveChainsKey(%s) = %s, want %s", tt.userID, result, tt.expected)
			}
		})
	}
}

// TestRecentEventsKey tests the recent events list key generation
func TestRecentEventsKey(t *testing.T) {
	tests := []struct {
		name     string
		userID   string
		expected string
	}{
		{
			name:     "standard key",
			userID:   "user-123",
			expected: "order_chain_events:recent:user-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RecentEventsKey(tt.userID)
			if result != tt.expected {
				t.Errorf("RecentEventsKey(%s) = %s, want %s", tt.userID, result, tt.expected)
			}
		})
	}
}

// TestOrderChainStateJSON tests JSON marshaling/unmarshaling
func TestOrderChainStateJSON(t *testing.T) {
	entryPrice := 97450.00
	entryQty := 0.01
	slPrice := 97100.00
	tpPrice := 98000.00

	state := &OrderChainState{
		ChainID:  "ULT-18JAN-00001",
		UserID:   "user-123",
		Symbol:   "BTCUSDT",
		Side:     "LONG",
		ModeCode: "ULT",
		Status:   "ACTIVE",
		Entry: &OrderChainEntry{
			Price:    entryPrice,
			Quantity: entryQty,
			Status:   "FILLED",
		},
		Position: &OrderChainPosition{
			Status:            "ACTIVE",
			EntryPrice:        entryPrice,
			Quantity:          entryQty,
			RemainingQuantity: entryQty,
		},
		StopLoss: &OrderChainStopLoss{
			CurrentPrice:      slPrice,
			ModificationCount: 3,
		},
		TakeProfit: &OrderChainTakeProfit{
			Mode:              "NORMAL",
			CurrentPrice:      tpPrice,
			ModificationCount: 1,
		},
		EventCount:   8,
		LastEventSeq: 8,
		CreatedAt:    "2026-01-18T09:00:00Z",
		UpdatedAt:    "2026-01-18T11:30:00Z",
	}

	// Marshal
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Failed to marshal OrderChainState: %v", err)
	}

	// Unmarshal
	var decoded OrderChainState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal OrderChainState: %v", err)
	}

	// Verify fields
	if decoded.ChainID != state.ChainID {
		t.Errorf("ChainID mismatch: got %s, want %s", decoded.ChainID, state.ChainID)
	}
	if decoded.Status != state.Status {
		t.Errorf("Status mismatch: got %s, want %s", decoded.Status, state.Status)
	}
	if decoded.Entry.Price != entryPrice {
		t.Errorf("Entry.Price mismatch: got %f, want %f", decoded.Entry.Price, entryPrice)
	}
	if decoded.StopLoss.ModificationCount != 3 {
		t.Errorf("SL ModificationCount mismatch: got %d, want 3", decoded.StopLoss.ModificationCount)
	}
}

// TestChainEventSummaryJSON tests event summary JSON
func TestChainEventSummaryJSON(t *testing.T) {
	price := 97200.00
	oldPrice := 97100.00
	orderType := "SL"

	event := &ChainEventSummary{
		ChainID:       "ULT-18JAN-00001",
		EventType:     "SL_MODIFIED",
		OrderType:     &orderType,
		Price:         &price,
		OldPrice:      &oldPrice,
		Timestamp:     "2026-01-18T11:30:00Z",
		EventSequence: 8,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal ChainEventSummary: %v", err)
	}

	var decoded ChainEventSummary
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ChainEventSummary: %v", err)
	}

	if decoded.ChainID != event.ChainID {
		t.Errorf("ChainID mismatch: got %s, want %s", decoded.ChainID, event.ChainID)
	}
	if decoded.EventType != event.EventType {
		t.Errorf("EventType mismatch: got %s, want %s", decoded.EventType, event.EventType)
	}
	if *decoded.Price != price {
		t.Errorf("Price mismatch: got %f, want %f", *decoded.Price, price)
	}
}

// TestFormatTimestamp tests timestamp formatting
func TestFormatTimestamp(t *testing.T) {
	testTime := time.Date(2026, 1, 18, 9, 15, 32, 0, time.UTC)
	result := FormatTimestamp(testTime)

	// Should be RFC3339 format
	if result != "2026-01-18T09:15:32Z" {
		t.Errorf("FormatTimestamp() = %s, want 2026-01-18T09:15:32Z", result)
	}
}

// TestFormatTimestampPtr tests pointer timestamp formatting
func TestFormatTimestampPtr(t *testing.T) {
	// Test with nil
	result := FormatTimestampPtr(nil)
	if result != nil {
		t.Errorf("FormatTimestampPtr(nil) = %v, want nil", result)
	}

	// Test with value
	testTime := time.Date(2026, 1, 18, 9, 15, 32, 0, time.UTC)
	result = FormatTimestampPtr(&testTime)
	if result == nil {
		t.Fatal("FormatTimestampPtr(&time) returned nil")
	}
	if *result != "2026-01-18T09:15:32Z" {
		t.Errorf("FormatTimestampPtr() = %s, want 2026-01-18T09:15:32Z", *result)
	}
}

// TestOrderChainCacheNilSafe tests that cache methods are nil-safe
func TestOrderChainCacheNilSafe(t *testing.T) {
	ctx := context.Background()

	// Test with nil cache service (using mock logger)
	mockLogger := NewMockLogger()
	cache := &OrderChainCache{cache: nil, logger: mockLogger}

	// All operations should gracefully handle nil
	err := cache.SetChainState(ctx, &OrderChainState{UserID: "user", ChainID: "chain"})
	if err != nil {
		t.Errorf("SetChainState with nil cache returned error: %v", err)
	}

	state, err := cache.GetChainState(ctx, "user", "chain")
	if err != nil {
		t.Errorf("GetChainState with nil cache returned error: %v", err)
	}
	if state != nil {
		t.Errorf("GetChainState with nil cache returned non-nil state")
	}

	err = cache.DeleteChainState(ctx, "user", "chain")
	if err != nil {
		t.Errorf("DeleteChainState with nil cache returned error: %v", err)
	}

	err = cache.AddToActiveChains(ctx, "user", "chain")
	if err != nil {
		t.Errorf("AddToActiveChains with nil cache returned error: %v", err)
	}

	ids, err := cache.GetActiveChainIDs(ctx, "user")
	if err != nil {
		t.Errorf("GetActiveChainIDs with nil cache returned error: %v", err)
	}
	if ids != nil {
		t.Errorf("GetActiveChainIDs with nil cache returned non-nil slice")
	}

	chains, err := cache.GetMultipleChains(ctx, "user", []string{"chain1", "chain2"})
	if err != nil {
		t.Errorf("GetMultipleChains with nil cache returned error: %v", err)
	}
	if chains != nil {
		t.Errorf("GetMultipleChains with nil cache returned non-nil slice")
	}

	err = cache.PushRecentEvent(ctx, "user", &ChainEventSummary{})
	if err != nil {
		t.Errorf("PushRecentEvent with nil cache returned error: %v", err)
	}

	events, err := cache.GetRecentEvents(ctx, "user", 10)
	if err != nil {
		t.Errorf("GetRecentEvents with nil cache returned error: %v", err)
	}
	if events != nil {
		t.Errorf("GetRecentEvents with nil cache returned non-nil slice")
	}

	err = cache.WarmCacheForUser(ctx, "user", []*OrderChainState{})
	if err != nil {
		t.Errorf("WarmCacheForUser with nil cache returned error: %v", err)
	}

	err = cache.InvalidateUserCache(ctx, "user")
	if err != nil {
		t.Errorf("InvalidateUserCache with nil cache returned error: %v", err)
	}
}

// TestKeyPrefixConstants tests that key prefix constants are defined correctly
func TestKeyPrefixConstants(t *testing.T) {
	if PrefixOrderChain != "order_chain:%s:%s" {
		t.Errorf("PrefixOrderChain = %s, want order_chain:%%s:%%s", PrefixOrderChain)
	}
	if PrefixActiveChains != "order_chains:active:%s" {
		t.Errorf("PrefixActiveChains = %s, want order_chains:active:%%s", PrefixActiveChains)
	}
	if PrefixRecentEvents != "order_chain_events:recent:%s" {
		t.Errorf("PrefixRecentEvents = %s, want order_chain_events:recent:%%s", PrefixRecentEvents)
	}
}

// TestRecentEventsTTL tests the TTL constant
func TestRecentEventsTTL(t *testing.T) {
	if RecentEventsTTL != time.Hour {
		t.Errorf("RecentEventsTTL = %v, want 1 hour", RecentEventsTTL)
	}
}

// TestRecentEventsMaxLength tests the max length constant
func TestRecentEventsMaxLength(t *testing.T) {
	if RecentEventsMaxLength != 100 {
		t.Errorf("RecentEventsMaxLength = %d, want 100", RecentEventsMaxLength)
	}
}

// TestIsHealthyNilCache tests IsHealthy with nil cache
func TestIsHealthyNilCache(t *testing.T) {
	mockLogger := NewMockLogger()
	cache := &OrderChainCache{cache: nil, logger: mockLogger}
	if cache.IsHealthy() {
		t.Error("IsHealthy() returned true for nil cache")
	}
}

// TestGetMultipleChainsEmptyList tests GetMultipleChains with empty list
func TestGetMultipleChainsEmptyList(t *testing.T) {
	mockLogger := NewMockLogger()
	cache := &OrderChainCache{cache: nil, logger: mockLogger}
	ctx := context.Background()

	chains, err := cache.GetMultipleChains(ctx, "user", []string{})
	if err != nil {
		t.Errorf("GetMultipleChains with empty list returned error: %v", err)
	}
	if chains != nil {
		t.Errorf("GetMultipleChains with empty list returned non-nil: %v", chains)
	}
}
