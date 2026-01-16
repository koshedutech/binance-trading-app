// Package orders provides chain tracking testing for Binance futures trading.
// Epic 7: Client Order ID & Trade Lifecycle Tracking - Chain Tracker Tests
package orders

import (
	"sync"
	"testing"
	"time"
)

// ============================================================================
// TEST CASES: CREATE CHAIN
// ============================================================================

// TestCreateChain verifies that CreateChain creates a new chain correctly
func TestCreateChain(t *testing.T) {
	tracker := NewChainTracker()

	chain, err := tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)

	if err != nil {
		t.Fatalf("CreateChain failed: %v", err)
	}

	// Verify chain properties
	if chain.BaseID != "ULT-15JAN-00001" {
		t.Errorf("Expected BaseID ULT-15JAN-00001, got %s", chain.BaseID)
	}
	if chain.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %s", chain.Symbol)
	}
	if chain.Mode != ModeUltraFast {
		t.Errorf("Expected mode %s, got %s", ModeUltraFast, chain.Mode)
	}
	if chain.Direction != DirectionLong {
		t.Errorf("Expected direction %s, got %s", DirectionLong, chain.Direction)
	}
	if chain.Status != ChainStatusActive {
		t.Errorf("Expected status %s, got %s", ChainStatusActive, chain.Status)
	}
	if chain.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

// TestCreateChainWithDifferentModes verifies chain creation with all trading modes
func TestCreateChainWithDifferentModes(t *testing.T) {
	modes := []TradingMode{ModeUltraFast, ModeScalp, ModeSwing, ModePosition}

	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			tracker := NewChainTracker()
			chainID := ModeCode[mode] + "-15JAN-00001"

			chain, err := tracker.CreateChain(chainID, "ETHUSDT", mode, DirectionShort)

			if err != nil {
				t.Fatalf("CreateChain failed: %v", err)
			}
			if chain.Mode != mode {
				t.Errorf("Expected mode %s, got %s", mode, chain.Mode)
			}
		})
	}
}

// TestCreateChainWithDifferentDirections verifies chain creation with both directions
func TestCreateChainWithDifferentDirections(t *testing.T) {
	directions := []Direction{DirectionLong, DirectionShort}

	for _, direction := range directions {
		t.Run(string(direction), func(t *testing.T) {
			tracker := NewChainTracker()

			chain, err := tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, direction)

			if err != nil {
				t.Fatalf("CreateChain failed: %v", err)
			}
			if chain.Direction != direction {
				t.Errorf("Expected direction %s, got %s", direction, chain.Direction)
			}
		})
	}
}

// TestCreateChainInitializesMetadata verifies metadata is initialized
func TestCreateChainInitializesMetadata(t *testing.T) {
	tracker := NewChainTracker()

	chain, err := tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)

	if err != nil {
		t.Fatalf("CreateChain failed: %v", err)
	}
	if chain.Metadata == nil {
		t.Error("Metadata should be initialized, not nil")
	}
	if chain.FilledOrders == nil {
		t.Error("FilledOrders should be initialized, not nil")
	}
	if chain.PendingOrders == nil {
		t.Error("PendingOrders should be initialized, not nil")
	}
}

// TestCreateChainMarksEntryPending verifies entry order is marked pending on creation
func TestCreateChainMarksEntryPending(t *testing.T) {
	tracker := NewChainTracker()

	chain, err := tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)

	if err != nil {
		t.Fatalf("CreateChain failed: %v", err)
	}
	if !chain.IsOrderPending(OrderTypeEntry) {
		t.Error("Entry order should be marked as pending on chain creation")
	}
}

// TestCreateChainDuplicateError verifies error when creating duplicate chain
func TestCreateChainDuplicateError(t *testing.T) {
	tracker := NewChainTracker()

	// Create first chain
	_, err := tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)
	if err != nil {
		t.Fatalf("First CreateChain failed: %v", err)
	}

	// Try to create duplicate
	_, err = tracker.CreateChain("ULT-15JAN-00001", "ETHUSDT", ModeScalp, DirectionShort)
	if err != ErrChainAlreadyExists {
		t.Errorf("Expected ErrChainAlreadyExists, got: %v", err)
	}
}

// TestCreateChainEmptyBaseIDError verifies error for empty base ID
func TestCreateChainEmptyBaseIDError(t *testing.T) {
	tracker := NewChainTracker()

	_, err := tracker.CreateChain("", "BTCUSDT", ModeUltraFast, DirectionLong)

	if err != ErrEmptyBaseID {
		t.Errorf("Expected ErrEmptyBaseID, got: %v", err)
	}
}

// TestCreateChainEmptySymbolError verifies error for empty symbol
func TestCreateChainEmptySymbolError(t *testing.T) {
	tracker := NewChainTracker()

	_, err := tracker.CreateChain("ULT-15JAN-00001", "", ModeUltraFast, DirectionLong)

	if err != ErrEmptySymbol {
		t.Errorf("Expected ErrEmptySymbol, got: %v", err)
	}
}

// ============================================================================
// TEST CASES: UPDATE CHAIN STATUS
// ============================================================================

// TestUpdateChainStatus verifies status updates work correctly
func TestUpdateChainStatus(t *testing.T) {
	tracker := NewChainTracker()

	// Create chain first
	_, err := tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)
	if err != nil {
		t.Fatalf("CreateChain failed: %v", err)
	}

	// Update status
	err = tracker.UpdateChainStatus("ULT-15JAN-00001", ChainStatusPartial)
	if err != nil {
		t.Fatalf("UpdateChainStatus failed: %v", err)
	}

	// Verify status was updated
	chain, _ := tracker.GetChain("ULT-15JAN-00001")
	if chain.Status != ChainStatusPartial {
		t.Errorf("Expected status %s, got %s", ChainStatusPartial, chain.Status)
	}
}

// TestUpdateChainStatusUpdatesTimestamp verifies UpdatedAt is updated
func TestUpdateChainStatusUpdatesTimestamp(t *testing.T) {
	tracker := NewChainTracker()

	chain, _ := tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)
	originalUpdatedAt := chain.UpdatedAt

	// Small delay to ensure timestamp differs
	time.Sleep(time.Millisecond)

	_ = tracker.UpdateChainStatus("ULT-15JAN-00001", ChainStatusPartial)

	updatedChain, _ := tracker.GetChain("ULT-15JAN-00001")
	if !updatedChain.UpdatedAt.After(originalUpdatedAt) {
		t.Error("UpdatedAt should be updated after status change")
	}
}

// TestUpdateChainStatusAllStatuses tests all valid status transitions
func TestUpdateChainStatusAllStatuses(t *testing.T) {
	statuses := []ChainStatus{
		ChainStatusActive,
		ChainStatusPartial,
		ChainStatusCompleted,
		ChainStatusCancelled,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			tracker := NewChainTracker()
			_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)

			err := tracker.UpdateChainStatus("ULT-15JAN-00001", status)
			if err != nil {
				t.Fatalf("UpdateChainStatus to %s failed: %v", status, err)
			}

			chain, _ := tracker.GetChain("ULT-15JAN-00001")
			if chain.Status != status {
				t.Errorf("Expected status %s, got %s", status, chain.Status)
			}
		})
	}
}

// TestUpdateChainStatusNotFound verifies error for non-existent chain
func TestUpdateChainStatusNotFound(t *testing.T) {
	tracker := NewChainTracker()

	err := tracker.UpdateChainStatus("NON-EXISTENT-CHAIN", ChainStatusCompleted)

	if err != ErrChainNotFound {
		t.Errorf("Expected ErrChainNotFound, got: %v", err)
	}
}

// TestUpdateChainStatusEmptyBaseID verifies error for empty base ID
func TestUpdateChainStatusEmptyBaseID(t *testing.T) {
	tracker := NewChainTracker()

	err := tracker.UpdateChainStatus("", ChainStatusCompleted)

	if err != ErrEmptyBaseID {
		t.Errorf("Expected ErrEmptyBaseID, got: %v", err)
	}
}

// TestUpdateChainStatusInvalidStatus verifies error for invalid status
func TestUpdateChainStatusInvalidStatus(t *testing.T) {
	tracker := NewChainTracker()
	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)

	err := tracker.UpdateChainStatus("ULT-15JAN-00001", ChainStatus("invalid_status"))

	if err != ErrInvalidChainStatus {
		t.Errorf("Expected ErrInvalidChainStatus, got: %v", err)
	}
}

// ============================================================================
// TEST CASES: GET CHAIN
// ============================================================================

// TestGetChain verifies chain retrieval works correctly
func TestGetChain(t *testing.T) {
	tracker := NewChainTracker()

	// Create chain
	created, _ := tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)

	// Retrieve chain
	retrieved, err := tracker.GetChain("ULT-15JAN-00001")

	if err != nil {
		t.Fatalf("GetChain failed: %v", err)
	}
	if retrieved.BaseID != created.BaseID {
		t.Errorf("Retrieved chain BaseID mismatch: expected %s, got %s", created.BaseID, retrieved.BaseID)
	}
	if retrieved.Symbol != created.Symbol {
		t.Errorf("Retrieved chain Symbol mismatch: expected %s, got %s", created.Symbol, retrieved.Symbol)
	}
}

// TestGetChainNotFound verifies error for non-existent chain
func TestGetChainNotFound(t *testing.T) {
	tracker := NewChainTracker()

	chain, err := tracker.GetChain("NON-EXISTENT-CHAIN")

	if err != ErrChainNotFound {
		t.Errorf("Expected ErrChainNotFound, got: %v", err)
	}
	if chain != nil {
		t.Error("Expected nil chain for non-existent ID")
	}
}

// TestGetChainEmptyBaseID verifies error for empty base ID
func TestGetChainEmptyBaseID(t *testing.T) {
	tracker := NewChainTracker()

	_, err := tracker.GetChain("")

	if err != ErrEmptyBaseID {
		t.Errorf("Expected ErrEmptyBaseID, got: %v", err)
	}
}

// TestGetChainReturnsCorrectData verifies all chain fields are returned correctly
func TestGetChainReturnsCorrectData(t *testing.T) {
	tracker := NewChainTracker()

	// Create chain with specific data
	_, _ = tracker.CreateChain("SCA-20JAN-00042", "ETHUSDT", ModeScalp, DirectionShort)

	// Mark some orders as filled/pending
	_ = tracker.MarkOrderFilled("SCA-20JAN-00042", OrderTypeEntry)
	_ = tracker.MarkOrderPending("SCA-20JAN-00042", OrderTypeSL)
	_ = tracker.MarkOrderPending("SCA-20JAN-00042", OrderTypeTP1)

	// Retrieve and verify
	chain, err := tracker.GetChain("SCA-20JAN-00042")

	if err != nil {
		t.Fatalf("GetChain failed: %v", err)
	}
	if chain.Symbol != "ETHUSDT" {
		t.Errorf("Expected Symbol ETHUSDT, got %s", chain.Symbol)
	}
	if chain.Mode != ModeScalp {
		t.Errorf("Expected Mode %s, got %s", ModeScalp, chain.Mode)
	}
	if chain.Direction != DirectionShort {
		t.Errorf("Expected Direction %s, got %s", DirectionShort, chain.Direction)
	}
	if !chain.IsOrderFilled(OrderTypeEntry) {
		t.Error("Entry order should be marked as filled")
	}
	if !chain.IsOrderPending(OrderTypeSL) {
		t.Error("SL order should be marked as pending")
	}
	if !chain.IsOrderPending(OrderTypeTP1) {
		t.Error("TP1 order should be marked as pending")
	}
}

// ============================================================================
// TEST CASES: GET ACTIVE CHAINS
// ============================================================================

// TestGetActiveChains verifies only active chains are returned
func TestGetActiveChains(t *testing.T) {
	tracker := NewChainTracker()

	// Create multiple chains
	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)
	_, _ = tracker.CreateChain("ULT-15JAN-00002", "ETHUSDT", ModeUltraFast, DirectionShort)
	_, _ = tracker.CreateChain("ULT-15JAN-00003", "SOLUSDT", ModeUltraFast, DirectionLong)

	// Close one chain
	_ = tracker.CloseChain("ULT-15JAN-00002")

	// Cancel another chain
	_ = tracker.CancelChain("ULT-15JAN-00003")

	// Get active chains
	activeChains := tracker.GetActiveChains()

	if len(activeChains) != 1 {
		t.Errorf("Expected 1 active chain, got %d", len(activeChains))
	}
	if activeChains[0].BaseID != "ULT-15JAN-00001" {
		t.Errorf("Expected active chain ULT-15JAN-00001, got %s", activeChains[0].BaseID)
	}
}

// TestGetActiveChainsIncludesPartial verifies partial status chains are returned
func TestGetActiveChainsIncludesPartial(t *testing.T) {
	tracker := NewChainTracker()

	// Create chains
	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)
	_, _ = tracker.CreateChain("ULT-15JAN-00002", "ETHUSDT", ModeUltraFast, DirectionShort)

	// Set one to partial
	_ = tracker.UpdateChainStatus("ULT-15JAN-00002", ChainStatusPartial)

	// Get active chains - should include both active and partial
	activeChains := tracker.GetActiveChains()

	if len(activeChains) != 2 {
		t.Errorf("Expected 2 active chains (including partial), got %d", len(activeChains))
	}
}

// TestGetActiveChainsEmptyResult verifies empty result for no active chains
func TestGetActiveChainsEmptyResult(t *testing.T) {
	tracker := NewChainTracker()

	// Create and close a chain
	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)
	_ = tracker.CloseChain("ULT-15JAN-00001")

	activeChains := tracker.GetActiveChains()

	if len(activeChains) != 0 {
		t.Errorf("Expected 0 active chains, got %d", len(activeChains))
	}
}

// TestGetActiveChainsEmptyTracker verifies empty result for new tracker
func TestGetActiveChainsEmptyTracker(t *testing.T) {
	tracker := NewChainTracker()

	activeChains := tracker.GetActiveChains()

	if len(activeChains) != 0 {
		t.Errorf("Expected 0 active chains for empty tracker, got %d", len(activeChains))
	}
}

// ============================================================================
// TEST CASES: CLOSE CHAIN
// ============================================================================

// TestCloseChain verifies chain closure works correctly
func TestCloseChain(t *testing.T) {
	tracker := NewChainTracker()

	// Create chain
	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)

	// Close chain
	err := tracker.CloseChain("ULT-15JAN-00001")

	if err != nil {
		t.Fatalf("CloseChain failed: %v", err)
	}
}

// TestCloseChainMarksCompleted verifies status is set to completed
func TestCloseChainMarksCompleted(t *testing.T) {
	tracker := NewChainTracker()

	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)
	_ = tracker.CloseChain("ULT-15JAN-00001")

	chain, _ := tracker.GetChain("ULT-15JAN-00001")

	if chain.Status != ChainStatusCompleted {
		t.Errorf("Expected status %s, got %s", ChainStatusCompleted, chain.Status)
	}
}

// TestCloseChainUpdatesTimestamp verifies UpdatedAt is updated
func TestCloseChainUpdatesTimestamp(t *testing.T) {
	tracker := NewChainTracker()

	chain, _ := tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)
	originalUpdatedAt := chain.UpdatedAt

	time.Sleep(time.Millisecond)
	_ = tracker.CloseChain("ULT-15JAN-00001")

	closedChain, _ := tracker.GetChain("ULT-15JAN-00001")
	if !closedChain.UpdatedAt.After(originalUpdatedAt) {
		t.Error("UpdatedAt should be updated after closing chain")
	}
}

// TestCloseChainNotFound verifies error for non-existent chain
func TestCloseChainNotFound(t *testing.T) {
	tracker := NewChainTracker()

	err := tracker.CloseChain("NON-EXISTENT-CHAIN")

	if err != ErrChainNotFound {
		t.Errorf("Expected ErrChainNotFound, got: %v", err)
	}
}

// TestCloseChainRemovesFromActive verifies closed chain is not in active list
func TestCloseChainRemovesFromActive(t *testing.T) {
	tracker := NewChainTracker()

	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)

	// Verify chain is active
	activeChains := tracker.GetActiveChains()
	if len(activeChains) != 1 {
		t.Fatalf("Expected 1 active chain before close, got %d", len(activeChains))
	}

	// Close chain
	_ = tracker.CloseChain("ULT-15JAN-00001")

	// Verify chain is no longer active
	activeChains = tracker.GetActiveChains()
	if len(activeChains) != 0 {
		t.Errorf("Expected 0 active chains after close, got %d", len(activeChains))
	}
}

// ============================================================================
// TEST CASES: CANCEL CHAIN
// ============================================================================

// TestCancelChain verifies chain cancellation works correctly
func TestCancelChain(t *testing.T) {
	tracker := NewChainTracker()

	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)

	err := tracker.CancelChain("ULT-15JAN-00001")

	if err != nil {
		t.Fatalf("CancelChain failed: %v", err)
	}

	chain, _ := tracker.GetChain("ULT-15JAN-00001")
	if chain.Status != ChainStatusCancelled {
		t.Errorf("Expected status %s, got %s", ChainStatusCancelled, chain.Status)
	}
}

// ============================================================================
// TEST CASES: GET CHAIN BY SYMBOL
// ============================================================================

// TestGetChainBySymbol verifies symbol lookup works correctly
func TestGetChainBySymbol(t *testing.T) {
	tracker := NewChainTracker()

	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)
	_, _ = tracker.CreateChain("ULT-15JAN-00002", "ETHUSDT", ModeUltraFast, DirectionShort)

	chain := tracker.GetChainBySymbol("BTCUSDT")

	if chain == nil {
		t.Fatal("Expected to find chain for BTCUSDT")
	}
	if chain.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %s", chain.Symbol)
	}
}

// TestGetChainBySymbolNotFound verifies nil return for non-existent symbol
func TestGetChainBySymbolNotFound(t *testing.T) {
	tracker := NewChainTracker()

	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)

	chain := tracker.GetChainBySymbol("XRPUSDT")

	if chain != nil {
		t.Error("Expected nil chain for non-existent symbol")
	}
}

// TestGetChainBySymbolReturnsActiveOnly verifies only active chains are returned
func TestGetChainBySymbolReturnsActiveOnly(t *testing.T) {
	tracker := NewChainTracker()

	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)
	_ = tracker.CloseChain("ULT-15JAN-00001")

	chain := tracker.GetChainBySymbol("BTCUSDT")

	if chain != nil {
		t.Error("Expected nil chain for closed chain symbol")
	}
}

// ============================================================================
// TEST CASES: MARK ORDER OPERATIONS
// ============================================================================

// TestMarkOrderFilled verifies marking orders as filled
func TestMarkOrderFilled(t *testing.T) {
	tracker := NewChainTracker()

	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)

	err := tracker.MarkOrderFilled("ULT-15JAN-00001", OrderTypeEntry)
	if err != nil {
		t.Fatalf("MarkOrderFilled failed: %v", err)
	}

	chain, _ := tracker.GetChain("ULT-15JAN-00001")
	if !chain.IsOrderFilled(OrderTypeEntry) {
		t.Error("Entry order should be marked as filled")
	}
	if chain.IsOrderPending(OrderTypeEntry) {
		t.Error("Entry order should no longer be pending")
	}
}

// TestMarkOrderPending verifies marking orders as pending
func TestMarkOrderPending(t *testing.T) {
	tracker := NewChainTracker()

	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)

	err := tracker.MarkOrderPending("ULT-15JAN-00001", OrderTypeSL)
	if err != nil {
		t.Fatalf("MarkOrderPending failed: %v", err)
	}

	chain, _ := tracker.GetChain("ULT-15JAN-00001")
	if !chain.IsOrderPending(OrderTypeSL) {
		t.Error("SL order should be marked as pending")
	}
}

// TestMarkOrderCancelled verifies marking orders as cancelled
func TestMarkOrderCancelled(t *testing.T) {
	tracker := NewChainTracker()

	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)
	_ = tracker.MarkOrderPending("ULT-15JAN-00001", OrderTypeTP1)

	err := tracker.MarkOrderCancelled("ULT-15JAN-00001", OrderTypeTP1)
	if err != nil {
		t.Fatalf("MarkOrderCancelled failed: %v", err)
	}

	chain, _ := tracker.GetChain("ULT-15JAN-00001")
	if chain.IsOrderPending(OrderTypeTP1) {
		t.Error("TP1 order should no longer be pending after cancellation")
	}
}

// ============================================================================
// TEST CASES: CHAIN COUNTS
// ============================================================================

// TestGetChainCount verifies total chain count
func TestGetChainCount(t *testing.T) {
	tracker := NewChainTracker()

	if tracker.GetChainCount() != 0 {
		t.Errorf("Expected 0 chains, got %d", tracker.GetChainCount())
	}

	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)
	_, _ = tracker.CreateChain("ULT-15JAN-00002", "ETHUSDT", ModeUltraFast, DirectionShort)
	_ = tracker.CloseChain("ULT-15JAN-00002")

	// Count includes all chains (active and closed)
	if tracker.GetChainCount() != 2 {
		t.Errorf("Expected 2 total chains, got %d", tracker.GetChainCount())
	}
}

// TestGetActiveChainCount verifies active chain count
func TestGetActiveChainCount(t *testing.T) {
	tracker := NewChainTracker()

	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)
	_, _ = tracker.CreateChain("ULT-15JAN-00002", "ETHUSDT", ModeUltraFast, DirectionShort)
	_ = tracker.CloseChain("ULT-15JAN-00002")

	if tracker.GetActiveChainCount() != 1 {
		t.Errorf("Expected 1 active chain, got %d", tracker.GetActiveChainCount())
	}
}

// ============================================================================
// TEST CASES: REMOVE AND CLEAR
// ============================================================================

// TestRemoveChain verifies chain removal
func TestRemoveChain(t *testing.T) {
	tracker := NewChainTracker()

	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)

	err := tracker.RemoveChain("ULT-15JAN-00001")
	if err != nil {
		t.Fatalf("RemoveChain failed: %v", err)
	}

	_, err = tracker.GetChain("ULT-15JAN-00001")
	if err != ErrChainNotFound {
		t.Errorf("Expected ErrChainNotFound after removal, got: %v", err)
	}
}

// TestClear verifies clearing all chains
func TestClear(t *testing.T) {
	tracker := NewChainTracker()

	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)
	_, _ = tracker.CreateChain("ULT-15JAN-00002", "ETHUSDT", ModeUltraFast, DirectionShort)

	tracker.Clear()

	if tracker.GetChainCount() != 0 {
		t.Errorf("Expected 0 chains after Clear, got %d", tracker.GetChainCount())
	}
}

// ============================================================================
// TEST CASES: METADATA
// ============================================================================

// TestSetAndGetChainMetadata verifies metadata operations
func TestSetAndGetChainMetadata(t *testing.T) {
	tracker := NewChainTracker()

	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)

	err := tracker.SetChainMetadata("ULT-15JAN-00001", "entryPrice", 42000.50)
	if err != nil {
		t.Fatalf("SetChainMetadata failed: %v", err)
	}

	value, err := tracker.GetChainMetadata("ULT-15JAN-00001", "entryPrice")
	if err != nil {
		t.Fatalf("GetChainMetadata failed: %v", err)
	}

	if value.(float64) != 42000.50 {
		t.Errorf("Expected metadata value 42000.50, got %v", value)
	}
}

// TestGetChainMetadataNotFound verifies nil return for missing key
func TestGetChainMetadataNotFound(t *testing.T) {
	tracker := NewChainTracker()

	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)

	value, err := tracker.GetChainMetadata("ULT-15JAN-00001", "nonExistentKey")
	if err != nil {
		t.Fatalf("GetChainMetadata failed: %v", err)
	}

	if value != nil {
		t.Errorf("Expected nil for non-existent key, got %v", value)
	}
}

// ============================================================================
// TEST CASES: CONCURRENT ACCESS
// ============================================================================

// TestConcurrentChainOperations verifies thread safety
func TestConcurrentChainOperations(t *testing.T) {
	tracker := NewChainTracker()

	// Create initial chain
	_, _ = tracker.CreateChain("ULT-15JAN-00001", "BTCUSDT", ModeUltraFast, DirectionLong)

	var wg sync.WaitGroup
	const goroutines = 50

	// Concurrent reads
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = tracker.GetChain("ULT-15JAN-00001")
		}()
	}

	// Concurrent status updates
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = tracker.UpdateChainStatus("ULT-15JAN-00001", ChainStatusPartial)
		}()
	}

	wg.Wait()

	// Verify chain is still accessible
	chain, err := tracker.GetChain("ULT-15JAN-00001")
	if err != nil {
		t.Fatalf("Chain should be accessible after concurrent operations: %v", err)
	}
	if chain == nil {
		t.Fatal("Chain should not be nil")
	}
}

// TestConcurrentChainCreation verifies concurrent chain creation
func TestConcurrentChainCreation(t *testing.T) {
	tracker := NewChainTracker()

	var wg sync.WaitGroup
	const goroutines = 20

	results := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			chainID := "ULT-15JAN-" + string(rune('A'+idx%26)) + string(rune('0'+idx/26))
			_, err := tracker.CreateChain(chainID, "BTCUSDT", ModeUltraFast, DirectionLong)
			results <- err
		}(i)
	}

	wg.Wait()
	close(results)

	// Count successful creations
	successCount := 0
	for err := range results {
		if err == nil {
			successCount++
		}
	}

	// Verify count matches successful creations
	if tracker.GetChainCount() != successCount {
		t.Errorf("Expected %d chains, got %d", successCount, tracker.GetChainCount())
	}
}

// TestConcurrentGetActiveChains verifies concurrent active chains retrieval
func TestConcurrentGetActiveChains(t *testing.T) {
	tracker := NewChainTracker()

	// Create multiple chains
	for i := 0; i < 10; i++ {
		chainID := "ULT-15JAN-" + string(rune('0'+i))
		_, _ = tracker.CreateChain(chainID, "BTCUSDT", ModeUltraFast, DirectionLong)
	}

	var wg sync.WaitGroup
	const goroutines = 50

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			chains := tracker.GetActiveChains()
			if chains == nil {
				t.Error("GetActiveChains should not return nil")
			}
		}()
	}

	wg.Wait()
}
