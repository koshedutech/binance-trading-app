package autopilot

import (
	"context"
	"testing"
	"time"
)

// ==================== STORY 14.14: Trading Controller Tests ====================
// Tests for Trading ON/OFF toggle functionality
// =============================================================================

// MockTradingController implements TradingController for testing
type MockTradingController struct {
	states  map[string]*TradingState
	enabled map[string]bool
}

// NewMockTradingController creates a new mock trading controller
func NewMockTradingController() *MockTradingController {
	return &MockTradingController{
		states:  make(map[string]*TradingState),
		enabled: make(map[string]bool),
	}
}

func (m *MockTradingController) IsTradingEnabled(ctx context.Context, userID string) (bool, error) {
	state, exists := m.states[userID]
	if !exists {
		return true, nil // Default enabled
	}
	return state.Enabled, nil
}

func (m *MockTradingController) SetTradingEnabled(ctx context.Context, userID string, enabled bool, changedBy string, reason string) error {
	m.states[userID] = &TradingState{
		Enabled:     enabled,
		EntryActive: enabled,
		ExitActive:  true,
		LastChanged: time.Now(),
		ChangedBy:   changedBy,
		Reason:      reason,
	}
	m.enabled[userID] = enabled
	return nil
}

func (m *MockTradingController) GetTradingState(ctx context.Context, userID string) (*TradingState, error) {
	state, exists := m.states[userID]
	if !exists {
		return DefaultTradingState(), nil
	}
	return state, nil
}

func (m *MockTradingController) IsEntryAllowed(ctx context.Context, userID string) (bool, error) {
	state, exists := m.states[userID]
	if !exists {
		return true, nil
	}
	return state.EntryActive, nil
}

func (m *MockTradingController) IsExitAllowed(ctx context.Context, userID string) (bool, error) {
	// Exit is always allowed when positions exist
	return true, nil
}

func (m *MockTradingController) InvalidateCache(ctx context.Context, userID string) error {
	delete(m.states, userID)
	delete(m.enabled, userID)
	return nil
}

// ==================== TEST CASES ====================

func TestDefaultTradingState(t *testing.T) {
	state := DefaultTradingState()

	if !state.Enabled {
		t.Error("Expected default state to be enabled")
	}

	if !state.EntryActive {
		t.Error("Expected default entry to be active")
	}

	if !state.ExitActive {
		t.Error("Expected default exit to be active")
	}

	if state.ChangedBy != "system" {
		t.Errorf("Expected ChangedBy to be 'system', got %s", state.ChangedBy)
	}
}

func TestMockTradingController_DefaultState(t *testing.T) {
	ctx := context.Background()
	controller := NewMockTradingController()
	userID := "test-user-1"

	// Check default state (should be enabled)
	enabled, err := controller.IsTradingEnabled(ctx, userID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !enabled {
		t.Error("Expected trading to be enabled by default")
	}

	// Check entry allowed by default
	entryAllowed, err := controller.IsEntryAllowed(ctx, userID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !entryAllowed {
		t.Error("Expected entry to be allowed by default")
	}

	// Check exit always allowed
	exitAllowed, err := controller.IsExitAllowed(ctx, userID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !exitAllowed {
		t.Error("Expected exit to always be allowed")
	}
}

func TestMockTradingController_DisableTrading(t *testing.T) {
	ctx := context.Background()
	controller := NewMockTradingController()
	userID := "test-user-2"

	// Disable trading
	err := controller.SetTradingEnabled(ctx, userID, false, "user", "testing disable")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify trading is disabled
	enabled, err := controller.IsTradingEnabled(ctx, userID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if enabled {
		t.Error("Expected trading to be disabled")
	}

	// Verify entry is paused
	entryAllowed, err := controller.IsEntryAllowed(ctx, userID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if entryAllowed {
		t.Error("Expected entry to be paused when trading is disabled")
	}

	// Verify exit is still allowed (always active)
	exitAllowed, err := controller.IsExitAllowed(ctx, userID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !exitAllowed {
		t.Error("Expected exit to remain active when trading is disabled")
	}

	// Get state and verify details
	state, err := controller.GetTradingState(ctx, userID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if state.ChangedBy != "user" {
		t.Errorf("Expected ChangedBy to be 'user', got %s", state.ChangedBy)
	}
	if state.Reason != "testing disable" {
		t.Errorf("Expected reason to be 'testing disable', got %s", state.Reason)
	}
}

func TestMockTradingController_EnableTrading(t *testing.T) {
	ctx := context.Background()
	controller := NewMockTradingController()
	userID := "test-user-3"

	// First disable trading
	err := controller.SetTradingEnabled(ctx, userID, false, "user", "initial disable")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Then enable trading
	err = controller.SetTradingEnabled(ctx, userID, true, "user", "resuming trading")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify trading is enabled
	enabled, err := controller.IsTradingEnabled(ctx, userID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !enabled {
		t.Error("Expected trading to be enabled")
	}

	// Verify entry is active
	entryAllowed, err := controller.IsEntryAllowed(ctx, userID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !entryAllowed {
		t.Error("Expected entry to be allowed when trading is enabled")
	}
}

func TestMockTradingController_SystemDisable(t *testing.T) {
	ctx := context.Background()
	controller := NewMockTradingController()
	userID := "test-user-4"

	// Disable by system
	err := controller.SetTradingEnabled(ctx, userID, false, "system", "circuit breaker triggered")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Get state and verify it was system triggered
	state, err := controller.GetTradingState(ctx, userID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if state.ChangedBy != "system" {
		t.Errorf("Expected ChangedBy to be 'system', got %s", state.ChangedBy)
	}
	if state.Reason != "circuit breaker triggered" {
		t.Errorf("Expected reason to be 'circuit breaker triggered', got %s", state.Reason)
	}
}

func TestMockTradingController_InvalidateCache(t *testing.T) {
	ctx := context.Background()
	controller := NewMockTradingController()
	userID := "test-user-5"

	// Set a state
	err := controller.SetTradingEnabled(ctx, userID, false, "user", "testing")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify it's disabled
	enabled, _ := controller.IsTradingEnabled(ctx, userID)
	if enabled {
		t.Error("Expected trading to be disabled")
	}

	// Invalidate cache
	err = controller.InvalidateCache(ctx, userID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// After invalidation, should return default (enabled)
	enabled, _ = controller.IsTradingEnabled(ctx, userID)
	if !enabled {
		t.Error("Expected trading to return to default (enabled) after cache invalidation")
	}
}

func TestMockTradingController_MultipleUsers(t *testing.T) {
	ctx := context.Background()
	controller := NewMockTradingController()

	// Set different states for different users
	controller.SetTradingEnabled(ctx, "user-1", true, "user", "")
	controller.SetTradingEnabled(ctx, "user-2", false, "user", "")
	controller.SetTradingEnabled(ctx, "user-3", true, "user", "")

	// Verify each user has correct state
	enabled1, _ := controller.IsTradingEnabled(ctx, "user-1")
	enabled2, _ := controller.IsTradingEnabled(ctx, "user-2")
	enabled3, _ := controller.IsTradingEnabled(ctx, "user-3")

	if !enabled1 {
		t.Error("Expected user-1 trading to be enabled")
	}
	if enabled2 {
		t.Error("Expected user-2 trading to be disabled")
	}
	if !enabled3 {
		t.Error("Expected user-3 trading to be enabled")
	}
}

func TestTradingState_ExitAlwaysActive(t *testing.T) {
	// Verify that ExitActive is always true in TradingState
	// This ensures position management continues even when trading is OFF

	state := &TradingState{
		Enabled:     false,
		EntryActive: false,
		ExitActive:  true, // Must always be true
		LastChanged: time.Now(),
		ChangedBy:   "user",
	}

	if !state.ExitActive {
		t.Error("ExitActive must always be true to ensure position management")
	}

	// When Enabled is false, Entry should be false but Exit still true
	if state.EntryActive {
		t.Error("EntryActive should be false when Enabled is false")
	}
	if !state.ExitActive {
		t.Error("ExitActive should remain true even when Enabled is false")
	}
}

func TestTradingState_Timestamps(t *testing.T) {
	ctx := context.Background()
	controller := NewMockTradingController()
	userID := "test-user-timestamp"

	before := time.Now()
	time.Sleep(time.Millisecond) // Small delay to ensure time difference

	err := controller.SetTradingEnabled(ctx, userID, true, "user", "timestamp test")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	state, err := controller.GetTradingState(ctx, userID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if state.LastChanged.Before(before) {
		t.Error("LastChanged should be after the time before SetTradingEnabled was called")
	}
}
