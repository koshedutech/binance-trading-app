// Package settlement provides integration tests for the position snapshot service.
// These tests require a running PostgreSQL database and are skipped if DB is unavailable.
//
//go:build integration
// +build integration

package settlement

import (
	"context"
	"os"
	"testing"
	"time"

	"binance-trading-bot/internal/database"
)

// getTestDB returns a database connection for integration tests.
// Returns nil if DATABASE_URL is not set.
func getTestDB(t *testing.T) *database.Repository {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
		return nil
	}

	db, err := database.NewDB(dbURL)
	if err != nil {
		t.Skipf("Failed to connect to database: %v", err)
		return nil
	}

	return database.NewRepository(db)
}

// ============================================================================
// INTEGRATION TEST: SaveDailyPositionSnapshots
// ============================================================================

func TestIntegration_SaveDailyPositionSnapshots(t *testing.T) {
	repo := getTestDB(t)
	if repo == nil {
		return
	}
	ctx := context.Background()

	// Create test data
	testUserID := "test-user-integration-" + time.Now().Format("20060102150405")
	testDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	snapshots := []database.DailyPositionSnapshot{
		{
			UserID:        testUserID,
			SnapshotDate:  testDate,
			Symbol:        "BTCUSDT",
			PositionSide:  "LONG",
			Quantity:      0.5,
			EntryPrice:    50000.0,
			MarkPrice:     51000.0,
			UnrealizedPnL: 500.0,
			Mode:          ModeScalp,
			Leverage:      10,
			MarginType:    "CROSSED",
		},
		{
			UserID:        testUserID,
			SnapshotDate:  testDate,
			Symbol:        "ETHUSDT",
			PositionSide:  "SHORT",
			Quantity:      1.0,
			EntryPrice:    3000.0,
			MarkPrice:     2900.0,
			UnrealizedPnL: 100.0,
			Mode:          ModeSwing,
			Leverage:      5,
			MarginType:    "ISOLATED",
		},
	}

	// Clean up first (in case of previous failed test)
	_ = repo.DeleteDailySnapshotsForDate(ctx, testUserID, testDate)

	// Save snapshots
	err := repo.SaveDailyPositionSnapshots(ctx, snapshots)
	if err != nil {
		t.Fatalf("Failed to save snapshots: %v", err)
	}

	// Retrieve and verify
	retrieved, err := repo.GetDailyPositionSnapshots(ctx, testUserID, testDate)
	if err != nil {
		t.Fatalf("Failed to retrieve snapshots: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("Expected 2 snapshots, got %d", len(retrieved))
	}

	// Verify mode breakdown
	modeBreakdown, err := repo.GetModeBreakdownForDate(ctx, testUserID, testDate)
	if err != nil {
		t.Fatalf("Failed to get mode breakdown: %v", err)
	}

	if len(modeBreakdown) != 2 {
		t.Errorf("Expected 2 mode breakdowns (scalp, swing), got %d", len(modeBreakdown))
	}

	// Clean up
	err = repo.DeleteDailySnapshotsForDate(ctx, testUserID, testDate)
	if err != nil {
		t.Errorf("Failed to clean up test data: %v", err)
	}

	t.Log("Integration test passed: SaveDailyPositionSnapshots")
}

// ============================================================================
// INTEGRATION TEST: Upsert Behavior
// ============================================================================

func TestIntegration_UpsertBehavior(t *testing.T) {
	repo := getTestDB(t)
	if repo == nil {
		return
	}
	ctx := context.Background()

	testUserID := "test-user-upsert-" + time.Now().Format("20060102150405")
	testDate := time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)

	// Clean up first
	_ = repo.DeleteDailySnapshotsForDate(ctx, testUserID, testDate)

	// First insert
	snapshot1 := []database.DailyPositionSnapshot{{
		UserID:        testUserID,
		SnapshotDate:  testDate,
		Symbol:        "BTCUSDT",
		PositionSide:  "LONG",
		Quantity:      0.5,
		EntryPrice:    50000.0,
		MarkPrice:     51000.0,
		UnrealizedPnL: 500.0,
		Mode:          ModeScalp,
		Leverage:      10,
		MarginType:    "CROSSED",
	}}

	err := repo.SaveDailyPositionSnapshots(ctx, snapshot1)
	if err != nil {
		t.Fatalf("Failed to save initial snapshot: %v", err)
	}

	// Upsert with different values
	snapshot2 := []database.DailyPositionSnapshot{{
		UserID:        testUserID,
		SnapshotDate:  testDate,
		Symbol:        "BTCUSDT",
		PositionSide:  "LONG",
		Quantity:      0.75, // Changed
		EntryPrice:    50000.0,
		MarkPrice:     52000.0, // Changed
		UnrealizedPnL: 1500.0,  // Changed
		Mode:          ModeScalp,
		Leverage:      10,
		MarginType:    "CROSSED",
	}}

	err = repo.SaveDailyPositionSnapshots(ctx, snapshot2)
	if err != nil {
		t.Fatalf("Failed to upsert snapshot: %v", err)
	}

	// Verify only one record exists with updated values
	retrieved, err := repo.GetDailyPositionSnapshots(ctx, testUserID, testDate)
	if err != nil {
		t.Fatalf("Failed to retrieve snapshots: %v", err)
	}

	if len(retrieved) != 1 {
		t.Errorf("Expected 1 snapshot after upsert, got %d", len(retrieved))
	}

	if retrieved[0].UnrealizedPnL != 1500.0 {
		t.Errorf("Expected UnrealizedPnL = 1500.0, got %f", retrieved[0].UnrealizedPnL)
	}

	if retrieved[0].MarkPrice != 52000.0 {
		t.Errorf("Expected MarkPrice = 52000.0, got %f", retrieved[0].MarkPrice)
	}

	// Clean up
	_ = repo.DeleteDailySnapshotsForDate(ctx, testUserID, testDate)

	t.Log("Integration test passed: UpsertBehavior")
}

// ============================================================================
// INTEGRATION TEST: Date Range Query
// ============================================================================

func TestIntegration_DateRangeQuery(t *testing.T) {
	repo := getTestDB(t)
	if repo == nil {
		return
	}
	ctx := context.Background()

	testUserID := "test-user-daterange-" + time.Now().Format("20060102150405")
	dates := []time.Time{
		time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 11, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
	}

	// Clean up first
	for _, d := range dates {
		_ = repo.DeleteDailySnapshotsForDate(ctx, testUserID, d)
	}

	// Insert snapshots for multiple dates
	for _, d := range dates {
		snapshot := []database.DailyPositionSnapshot{{
			UserID:        testUserID,
			SnapshotDate:  d,
			Symbol:        "BTCUSDT",
			PositionSide:  "LONG",
			Quantity:      0.5,
			EntryPrice:    50000.0,
			MarkPrice:     51000.0,
			UnrealizedPnL: 500.0,
			Mode:          ModeScalp,
			Leverage:      10,
			MarginType:    "CROSSED",
		}}
		if err := repo.SaveDailyPositionSnapshots(ctx, snapshot); err != nil {
			t.Fatalf("Failed to save snapshot for %v: %v", d, err)
		}
	}

	// Query date range
	startDate := dates[0]
	endDate := dates[2]
	retrieved, err := repo.GetDailyPositionSnapshotsDateRange(ctx, testUserID, startDate, endDate)
	if err != nil {
		t.Fatalf("Failed to query date range: %v", err)
	}

	if len(retrieved) != 3 {
		t.Errorf("Expected 3 snapshots in date range, got %d", len(retrieved))
	}

	// Clean up
	for _, d := range dates {
		_ = repo.DeleteDailySnapshotsForDate(ctx, testUserID, d)
	}

	t.Log("Integration test passed: DateRangeQuery")
}

// ============================================================================
// INTEGRATION TEST: HasDailySnapshotForDate
// ============================================================================

func TestIntegration_HasDailySnapshotForDate(t *testing.T) {
	repo := getTestDB(t)
	if repo == nil {
		return
	}
	ctx := context.Background()

	testUserID := "test-user-exists-" + time.Now().Format("20060102150405")
	testDate := time.Date(2026, 1, 13, 0, 0, 0, 0, time.UTC)

	// Clean up first
	_ = repo.DeleteDailySnapshotsForDate(ctx, testUserID, testDate)

	// Check non-existent
	exists, err := repo.HasDailySnapshotForDate(ctx, testUserID, testDate)
	if err != nil {
		t.Fatalf("Failed to check existence: %v", err)
	}
	if exists {
		t.Error("Expected snapshot not to exist before insert")
	}

	// Insert
	snapshot := []database.DailyPositionSnapshot{{
		UserID:        testUserID,
		SnapshotDate:  testDate,
		Symbol:        "BTCUSDT",
		PositionSide:  "LONG",
		Quantity:      0.5,
		EntryPrice:    50000.0,
		MarkPrice:     51000.0,
		UnrealizedPnL: 500.0,
		Mode:          ModeScalp,
		Leverage:      10,
		MarginType:    "CROSSED",
	}}
	if err := repo.SaveDailyPositionSnapshots(ctx, snapshot); err != nil {
		t.Fatalf("Failed to save snapshot: %v", err)
	}

	// Check exists now
	exists, err = repo.HasDailySnapshotForDate(ctx, testUserID, testDate)
	if err != nil {
		t.Fatalf("Failed to check existence: %v", err)
	}
	if !exists {
		t.Error("Expected snapshot to exist after insert")
	}

	// Clean up
	_ = repo.DeleteDailySnapshotsForDate(ctx, testUserID, testDate)

	t.Log("Integration test passed: HasDailySnapshotForDate")
}
