// Package database provides tests for CalibrationRepository.
// Epic 11: Position Decision Engine - Story 11.25: Calibration Data Storage
package database

import (
	"context"
	"testing"
	"time"
)

// TestCalibrationBucketData tests the CalibrationBucketData struct.
func TestCalibrationBucketData(t *testing.T) {
	data := &CalibrationBucketData{
		TotalTrades:     100,
		WinningTrades:   60,
		WinRate:         0.60,
		AvgPnLPercent:   1.5,
		TotalPnLPercent: 150.0,
		LastUpdated:     "2026-01-18T10:00:00Z",
	}

	if data.TotalTrades != 100 {
		t.Errorf("Expected TotalTrades=100, got %d", data.TotalTrades)
	}
	if data.WinRate != 0.60 {
		t.Errorf("Expected WinRate=0.60, got %f", data.WinRate)
	}
}

// TestCalibrationMetadata tests the CalibrationMetadata struct.
func TestCalibrationMetadata(t *testing.T) {
	meta := &CalibrationMetadata{
		LastUpdated:            "2026-01-18T10:00:00Z",
		FirstTradeDate:         "2026-01-01T00:00:00Z",
		TotalCalibrationTrades: 500,
	}

	if meta.TotalCalibrationTrades != 500 {
		t.Errorf("Expected TotalCalibrationTrades=500, got %d", meta.TotalCalibrationTrades)
	}
}

// TestCalibrationDataRecord tests the CalibrationDataRecord struct.
func TestCalibrationDataRecord(t *testing.T) {
	record := &CalibrationDataRecord{
		ID:            1,
		UserID:        "test-user-123",
		Strategy:      "trend_following",
		IndicatorHash: "abc12345",
		IndicatorList: []string{"rsi", "ema_cross", "atr"},
		Bucket0_24: &CalibrationBucketData{
			TotalTrades:   100,
			WinningTrades: 23,
			WinRate:       0.23,
			AvgPnLPercent: -1.2,
		},
		Bucket25_49: &CalibrationBucketData{
			TotalTrades:   150,
			WinningTrades: 62,
			WinRate:       0.41,
			AvgPnLPercent: 0.3,
		},
		Bucket50_74: &CalibrationBucketData{
			TotalTrades:   200,
			WinningTrades: 116,
			WinRate:       0.58,
			AvgPnLPercent: 1.1,
		},
		Bucket75_100: &CalibrationBucketData{
			TotalTrades:   80,
			WinningTrades: 58,
			WinRate:       0.72,
			AvgPnLPercent: 2.4,
		},
		Metadata: &CalibrationMetadata{
			LastUpdated:            "2026-01-18T10:00:00Z",
			FirstTradeDate:         "2026-01-01T00:00:00Z",
			TotalCalibrationTrades: 530,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if record.Strategy != "trend_following" {
		t.Errorf("Expected Strategy='trend_following', got '%s'", record.Strategy)
	}
	if len(record.IndicatorList) != 3 {
		t.Errorf("Expected 3 indicators, got %d", len(record.IndicatorList))
	}
	if record.Bucket0_24.WinRate != 0.23 {
		t.Errorf("Expected Bucket0_24 WinRate=0.23, got %f", record.Bucket0_24.WinRate)
	}
	if record.Bucket75_100.WinRate != 0.72 {
		t.Errorf("Expected Bucket75_100 WinRate=0.72, got %f", record.Bucket75_100.WinRate)
	}
}

// TestCalibrationHistoryRecord tests the CalibrationHistoryRecord struct.
func TestCalibrationHistoryRecord(t *testing.T) {
	record := &CalibrationHistoryRecord{
		ID:            1,
		OriginalID:    42,
		UserID:        "test-user-123",
		Strategy:      "mean_reversion",
		IndicatorHash: "def67890",
		IndicatorList: []string{"bollinger", "rsi"},
		Bucket0_24:    &CalibrationBucketData{TotalTrades: 50},
		Bucket25_49:   &CalibrationBucketData{TotalTrades: 100},
		Bucket50_74:   &CalibrationBucketData{TotalTrades: 150},
		Bucket75_100:  &CalibrationBucketData{TotalTrades: 75},
		Metadata: &CalibrationMetadata{
			TotalCalibrationTrades: 375,
		},
		ArchivedAt:    time.Now(),
		ArchiveReason: "INDICATOR_CHANGE",
	}

	if record.OriginalID != 42 {
		t.Errorf("Expected OriginalID=42, got %d", record.OriginalID)
	}
	if record.ArchiveReason != "INDICATOR_CHANGE" {
		t.Errorf("Expected ArchiveReason='INDICATOR_CHANGE', got '%s'", record.ArchiveReason)
	}
}

// MockCalibrationRepository is a mock implementation for testing.
type MockCalibrationRepository struct {
	data    map[string]*CalibrationDataRecord
	history []*CalibrationHistoryRecord
}

// NewMockCalibrationRepository creates a new mock repository.
func NewMockCalibrationRepository() *MockCalibrationRepository {
	return &MockCalibrationRepository{
		data:    make(map[string]*CalibrationDataRecord),
		history: make([]*CalibrationHistoryRecord, 0),
	}
}

func (m *MockCalibrationRepository) key(userID, strategy, indicatorHash string) string {
	return userID + ":" + strategy + ":" + indicatorHash
}

func (m *MockCalibrationRepository) UpsertCalibrationData(ctx context.Context, data *CalibrationDataRecord) error {
	key := m.key(data.UserID, data.Strategy, data.IndicatorHash)
	existing, exists := m.data[key]
	if exists {
		data.ID = existing.ID
	} else {
		data.ID = int64(len(m.data) + 1)
	}
	data.UpdatedAt = time.Now()
	if data.CreatedAt.IsZero() {
		data.CreatedAt = time.Now()
	}
	m.data[key] = data
	return nil
}

func (m *MockCalibrationRepository) GetCalibrationData(ctx context.Context, userID, strategy, indicatorHash string) (*CalibrationDataRecord, error) {
	key := m.key(userID, strategy, indicatorHash)
	if data, ok := m.data[key]; ok {
		return data, nil
	}
	return nil, nil
}

func (m *MockCalibrationRepository) GetAllCalibrationDataForUser(ctx context.Context, userID string) ([]*CalibrationDataRecord, error) {
	var results []*CalibrationDataRecord
	for key, data := range m.data {
		if len(key) > len(userID) && key[:len(userID)+1] == userID+":" {
			results = append(results, data)
		}
	}
	return results, nil
}

func (m *MockCalibrationRepository) GetCalibrationDataByStrategy(ctx context.Context, userID, strategy string) ([]*CalibrationDataRecord, error) {
	var results []*CalibrationDataRecord
	prefix := userID + ":" + strategy + ":"
	for key, data := range m.data {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			results = append(results, data)
		}
	}
	return results, nil
}

func (m *MockCalibrationRepository) ArchiveCalibrationData(ctx context.Context, userID, strategy, indicatorHash, reason string) error {
	key := m.key(userID, strategy, indicatorHash)
	if data, ok := m.data[key]; ok {
		history := &CalibrationHistoryRecord{
			ID:            int64(len(m.history) + 1),
			OriginalID:    data.ID,
			UserID:        data.UserID,
			Strategy:      data.Strategy,
			IndicatorHash: data.IndicatorHash,
			IndicatorList: data.IndicatorList,
			Bucket0_24:    data.Bucket0_24,
			Bucket25_49:   data.Bucket25_49,
			Bucket50_74:   data.Bucket50_74,
			Bucket75_100:  data.Bucket75_100,
			Metadata:      data.Metadata,
			ArchivedAt:    time.Now(),
			ArchiveReason: reason,
		}
		m.history = append(m.history, history)
		delete(m.data, key)
	}
	return nil
}

func (m *MockCalibrationRepository) DeleteCalibrationData(ctx context.Context, userID, strategy, indicatorHash string) error {
	key := m.key(userID, strategy, indicatorHash)
	delete(m.data, key)
	return nil
}

func (m *MockCalibrationRepository) GetCalibrationHistory(ctx context.Context, userID, strategy string) ([]*CalibrationHistoryRecord, error) {
	var results []*CalibrationHistoryRecord
	for _, h := range m.history {
		if h.UserID == userID && h.Strategy == strategy {
			results = append(results, h)
		}
	}
	return results, nil
}

// TestMockCalibrationRepository_Upsert tests the mock repository upsert operation.
func TestMockCalibrationRepository_Upsert(t *testing.T) {
	repo := NewMockCalibrationRepository()
	ctx := context.Background()

	record := &CalibrationDataRecord{
		UserID:        "user-1",
		Strategy:      "trend_following",
		IndicatorHash: "abc12345",
		IndicatorList: []string{"rsi", "ema"},
		Bucket0_24:    &CalibrationBucketData{TotalTrades: 10},
		Bucket25_49:   &CalibrationBucketData{TotalTrades: 20},
		Bucket50_74:   &CalibrationBucketData{TotalTrades: 30},
		Bucket75_100:  &CalibrationBucketData{TotalTrades: 40},
		Metadata:      &CalibrationMetadata{TotalCalibrationTrades: 100},
	}

	// Insert
	err := repo.UpsertCalibrationData(ctx, record)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}
	if record.ID != 1 {
		t.Errorf("Expected ID=1, got %d", record.ID)
	}

	// Get
	retrieved, err := repo.GetCalibrationData(ctx, "user-1", "trend_following", "abc12345")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected record, got nil")
	}
	if retrieved.Bucket0_24.TotalTrades != 10 {
		t.Errorf("Expected TotalTrades=10, got %d", retrieved.Bucket0_24.TotalTrades)
	}

	// Update
	record.Bucket0_24.TotalTrades = 15
	err = repo.UpsertCalibrationData(ctx, record)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	retrieved, _ = repo.GetCalibrationData(ctx, "user-1", "trend_following", "abc12345")
	if retrieved.Bucket0_24.TotalTrades != 15 {
		t.Errorf("Expected TotalTrades=15 after update, got %d", retrieved.Bucket0_24.TotalTrades)
	}
}

// TestMockCalibrationRepository_GetAll tests getting all calibrations for a user.
func TestMockCalibrationRepository_GetAll(t *testing.T) {
	repo := NewMockCalibrationRepository()
	ctx := context.Background()

	// Add multiple records for the same user
	records := []*CalibrationDataRecord{
		{
			UserID:        "user-1",
			Strategy:      "trend_following",
			IndicatorHash: "hash1",
			Bucket0_24:    &CalibrationBucketData{},
			Bucket25_49:   &CalibrationBucketData{},
			Bucket50_74:   &CalibrationBucketData{},
			Bucket75_100:  &CalibrationBucketData{},
			Metadata:      &CalibrationMetadata{},
		},
		{
			UserID:        "user-1",
			Strategy:      "mean_reversion",
			IndicatorHash: "hash2",
			Bucket0_24:    &CalibrationBucketData{},
			Bucket25_49:   &CalibrationBucketData{},
			Bucket50_74:   &CalibrationBucketData{},
			Bucket75_100:  &CalibrationBucketData{},
			Metadata:      &CalibrationMetadata{},
		},
		{
			UserID:        "user-2",
			Strategy:      "trend_following",
			IndicatorHash: "hash3",
			Bucket0_24:    &CalibrationBucketData{},
			Bucket25_49:   &CalibrationBucketData{},
			Bucket50_74:   &CalibrationBucketData{},
			Bucket75_100:  &CalibrationBucketData{},
			Metadata:      &CalibrationMetadata{},
		},
	}

	for _, r := range records {
		repo.UpsertCalibrationData(ctx, r)
	}

	// Get all for user-1
	user1Records, err := repo.GetAllCalibrationDataForUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetAllForUser failed: %v", err)
	}
	if len(user1Records) != 2 {
		t.Errorf("Expected 2 records for user-1, got %d", len(user1Records))
	}

	// Get by strategy
	byStrategy, err := repo.GetCalibrationDataByStrategy(ctx, "user-1", "trend_following")
	if err != nil {
		t.Fatalf("GetByStrategy failed: %v", err)
	}
	if len(byStrategy) != 1 {
		t.Errorf("Expected 1 record for trend_following, got %d", len(byStrategy))
	}
}

// TestMockCalibrationRepository_Archive tests archiving calibration data.
func TestMockCalibrationRepository_Archive(t *testing.T) {
	repo := NewMockCalibrationRepository()
	ctx := context.Background()

	record := &CalibrationDataRecord{
		UserID:        "user-1",
		Strategy:      "trend_following",
		IndicatorHash: "abc12345",
		IndicatorList: []string{"rsi"},
		Bucket0_24:    &CalibrationBucketData{TotalTrades: 100},
		Bucket25_49:   &CalibrationBucketData{},
		Bucket50_74:   &CalibrationBucketData{},
		Bucket75_100:  &CalibrationBucketData{},
		Metadata:      &CalibrationMetadata{TotalCalibrationTrades: 100},
	}
	repo.UpsertCalibrationData(ctx, record)

	// Archive
	err := repo.ArchiveCalibrationData(ctx, "user-1", "trend_following", "abc12345", "INDICATOR_CHANGE")
	if err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	// Original should be deleted
	retrieved, _ := repo.GetCalibrationData(ctx, "user-1", "trend_following", "abc12345")
	if retrieved != nil {
		t.Error("Expected original to be deleted after archive")
	}

	// Should be in history
	history, err := repo.GetCalibrationHistory(ctx, "user-1", "trend_following")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("Expected 1 history record, got %d", len(history))
	}
	if history[0].ArchiveReason != "INDICATOR_CHANGE" {
		t.Errorf("Expected reason='INDICATOR_CHANGE', got '%s'", history[0].ArchiveReason)
	}
}

// TestMockCalibrationRepository_Delete tests deleting calibration data.
func TestMockCalibrationRepository_Delete(t *testing.T) {
	repo := NewMockCalibrationRepository()
	ctx := context.Background()

	record := &CalibrationDataRecord{
		UserID:        "user-1",
		Strategy:      "trend_following",
		IndicatorHash: "abc12345",
		Bucket0_24:    &CalibrationBucketData{},
		Bucket25_49:   &CalibrationBucketData{},
		Bucket50_74:   &CalibrationBucketData{},
		Bucket75_100:  &CalibrationBucketData{},
		Metadata:      &CalibrationMetadata{},
	}
	repo.UpsertCalibrationData(ctx, record)

	// Delete
	err := repo.DeleteCalibrationData(ctx, "user-1", "trend_following", "abc12345")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Should be gone
	retrieved, _ := repo.GetCalibrationData(ctx, "user-1", "trend_following", "abc12345")
	if retrieved != nil {
		t.Error("Expected record to be deleted")
	}
}

// TestCalibrationRepository_NilPoolErrors tests that methods return errors when Pool is nil.
// This verifies the error handling behavior for database pool nil checks.
func TestCalibrationRepository_NilPoolErrors(t *testing.T) {
	// Create DB struct with nil Pool to test error handling
	db := &DB{Pool: nil}
	ctx := context.Background()

	// Test UpsertCalibrationData
	err := db.UpsertCalibrationData(ctx, &CalibrationDataRecord{})
	if err == nil {
		t.Error("UpsertCalibrationData: expected error for nil pool, got nil")
	}

	// Test GetCalibrationData
	_, err = db.GetCalibrationData(ctx, "user-1", "strategy", "hash")
	if err == nil {
		t.Error("GetCalibrationData: expected error for nil pool, got nil")
	}

	// Test GetAllCalibrationDataForUser
	_, err = db.GetAllCalibrationDataForUser(ctx, "user-1")
	if err == nil {
		t.Error("GetAllCalibrationDataForUser: expected error for nil pool, got nil")
	}

	// Test GetCalibrationDataByStrategy
	_, err = db.GetCalibrationDataByStrategy(ctx, "user-1", "strategy")
	if err == nil {
		t.Error("GetCalibrationDataByStrategy: expected error for nil pool, got nil")
	}

	// Test GetCalibrationHistory
	_, err = db.GetCalibrationHistory(ctx, "user-1", "strategy")
	if err == nil {
		t.Error("GetCalibrationHistory: expected error for nil pool, got nil")
	}

	// Test ArchiveCalibrationData
	err = db.ArchiveCalibrationData(ctx, "user-1", "strategy", "hash", "reason")
	if err == nil {
		t.Error("ArchiveCalibrationData: expected error for nil pool, got nil")
	}

	// Test DeleteCalibrationData
	err = db.DeleteCalibrationData(ctx, "user-1", "strategy", "hash")
	if err == nil {
		t.Error("DeleteCalibrationData: expected error for nil pool, got nil")
	}
}
