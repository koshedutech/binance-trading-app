// Package database provides tests for ModeStrategy repository operations.
// Epic 11: Position Decision Engine - Story 11.29: Mode-Strategy Database Schema
package database

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// STRUCT TESTS
// ============================================================================

// TestModeStrategySettingsRow tests the ModeStrategySettingsRow struct.
func TestModeStrategySettingsRow(t *testing.T) {
	settings := json.RawMessage(`{"leverage": 10, "max_positions": 5}`)
	now := time.Now()

	row := &ModeStrategySettingsRow{
		ID:        1,
		UserID:    "123",
		Mode:      "scalp",
		Strategy:  "trend_following",
		Enabled:   true,
		Priority:  1,
		Settings:  settings,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if row.ID != 1 {
		t.Errorf("Expected ID=1, got %d", row.ID)
	}
	if row.UserID != "123" {
		t.Errorf("Expected UserID='123', got '%s'", row.UserID)
	}
	if row.Mode != "scalp" {
		t.Errorf("Expected Mode='scalp', got '%s'", row.Mode)
	}
	if row.Strategy != "trend_following" {
		t.Errorf("Expected Strategy='trend_following', got '%s'", row.Strategy)
	}
	if !row.Enabled {
		t.Error("Expected Enabled=true")
	}
	if row.Priority != 1 {
		t.Errorf("Expected Priority=1, got %d", row.Priority)
	}

	// Verify settings JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(row.Settings, &parsed); err != nil {
		t.Errorf("Failed to parse settings JSON: %v", err)
	}
	if parsed["leverage"].(float64) != 10 {
		t.Errorf("Expected leverage=10, got %v", parsed["leverage"])
	}
}

// TestModeStrategySettingsRow_AllModes tests all valid mode values.
func TestModeStrategySettingsRow_AllModes(t *testing.T) {
	modes := []string{"scalp", "swing", "position", "ultra_fast"}

	for _, mode := range modes {
		row := &ModeStrategySettingsRow{
			Mode:     mode,
			Strategy: "trend_following",
			Settings: json.RawMessage(`{}`),
		}
		if row.Mode != mode {
			t.Errorf("Expected Mode='%s', got '%s'", mode, row.Mode)
		}
	}
}

// TestModeStrategySettingsRow_AllStrategies tests all valid strategy values.
func TestModeStrategySettingsRow_AllStrategies(t *testing.T) {
	strategies := []string{"trend_following", "mean_reversion", "breakout", "range_trading"}

	for _, strategy := range strategies {
		row := &ModeStrategySettingsRow{
			Mode:     "scalp",
			Strategy: strategy,
			Settings: json.RawMessage(`{}`),
		}
		if row.Strategy != strategy {
			t.Errorf("Expected Strategy='%s', got '%s'", strategy, row.Strategy)
		}
	}
}

// ============================================================================
// MOCK REPOSITORY
// ============================================================================

// MockModeStrategyRepository is a mock implementation for testing.
type MockModeStrategyRepository struct {
	mu   sync.RWMutex
	data map[string]*ModeStrategySettingsRow
	seq  int
}

// NewMockModeStrategyRepository creates a new mock repository.
func NewMockModeStrategyRepository() *MockModeStrategyRepository {
	return &MockModeStrategyRepository{
		data: make(map[string]*ModeStrategySettingsRow),
		seq:  0,
	}
}

func (m *MockModeStrategyRepository) key(userID string, mode, strategy string) string {
	return userID + ":" + mode + ":" + strategy
}

func (m *MockModeStrategyRepository) keyStr(userID string, mode, strategy string) string {
	return fmt.Sprintf("%s:%s:%s", userID, mode, strategy)
}

// CreateModeStrategySettings creates a new mode+strategy configuration.
func (m *MockModeStrategyRepository) CreateModeStrategySettings(ctx context.Context, userID string, mode, strategy string, enabled bool, priority int, settings json.RawMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.seq++
	key := m.keyStr(userID, mode, strategy)

	m.data[key] = &ModeStrategySettingsRow{
		ID:        m.seq,
		UserID:    userID,
		Mode:      mode,
		Strategy:  strategy,
		Enabled:   enabled,
		Priority:  priority,
		Settings:  settings,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return nil
}

// GetModeStrategySettings retrieves a specific mode+strategy configuration.
func (m *MockModeStrategyRepository) GetModeStrategySettings(ctx context.Context, userID string, mode, strategy string) (*ModeStrategySettingsRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := m.keyStr(userID, mode, strategy)
	if row, ok := m.data[key]; ok {
		return row, nil
	}
	return nil, nil // Not found
}

// GetAllModeStrategySettings retrieves all mode+strategy configurations for a user.
func (m *MockModeStrategyRepository) GetAllModeStrategySettings(ctx context.Context, userID string) ([]*ModeStrategySettingsRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ModeStrategySettingsRow
	prefix := userID + ":"

	for key, row := range m.data {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			result = append(result, row)
		}
	}

	return result, nil
}

// GetModeStrategies retrieves all strategies for a specific mode.
func (m *MockModeStrategyRepository) GetModeStrategies(ctx context.Context, userID string, mode string) ([]*ModeStrategySettingsRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ModeStrategySettingsRow
	prefix := fmt.Sprintf("%s:%s:", userID, mode)

	for key, row := range m.data {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			result = append(result, row)
		}
	}

	return result, nil
}

// UpdateModeStrategySettings updates the settings for a mode+strategy.
func (m *MockModeStrategyRepository) UpdateModeStrategySettings(ctx context.Context, userID string, mode, strategy string, settings json.RawMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.keyStr(userID, mode, strategy)
	if row, ok := m.data[key]; ok {
		row.Settings = settings
		row.UpdatedAt = time.Now()
		return nil
	}

	return fmt.Errorf("mode strategy settings not found for user %s, mode %s, strategy %s", userID, mode, strategy)
}

// UpdateModeStrategyEnabled updates the enabled status.
func (m *MockModeStrategyRepository) UpdateModeStrategyEnabled(ctx context.Context, userID string, mode, strategy string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.keyStr(userID, mode, strategy)
	if row, ok := m.data[key]; ok {
		row.Enabled = enabled
		row.UpdatedAt = time.Now()
		return nil
	}

	return fmt.Errorf("mode strategy settings not found for user %s, mode %s, strategy %s", userID, mode, strategy)
}

// DeleteModeStrategySettings deletes a mode+strategy configuration.
func (m *MockModeStrategyRepository) DeleteModeStrategySettings(ctx context.Context, userID string, mode, strategy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.keyStr(userID, mode, strategy)
	if _, ok := m.data[key]; ok {
		delete(m.data, key)
		return nil
	}

	return fmt.Errorf("mode strategy settings not found for user %s, mode %s, strategy %s", userID, mode, strategy)
}

// BulkCreateModeStrategySettings creates multiple mode+strategy configurations.
func (m *MockModeStrategyRepository) BulkCreateModeStrategySettings(ctx context.Context, userID string, configs []ModeStrategySettingsRow) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, config := range configs {
		m.seq++
		key := m.keyStr(userID, config.Mode, config.Strategy)

		// Upsert behavior (ON CONFLICT DO UPDATE)
		if existing, ok := m.data[key]; ok {
			existing.Enabled = config.Enabled
			existing.Priority = config.Priority
			existing.Settings = config.Settings
			existing.UpdatedAt = time.Now()
		} else {
			m.data[key] = &ModeStrategySettingsRow{
				ID:        m.seq,
				UserID:    userID,
				Mode:      config.Mode,
				Strategy:  config.Strategy,
				Enabled:   config.Enabled,
				Priority:  config.Priority,
				Settings:  config.Settings,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
		}
	}

	return nil
}

// ============================================================================
// MOCK REPOSITORY TESTS
// ============================================================================

// TestMockModeStrategyRepository_Create tests creating a mode+strategy configuration.
func TestMockModeStrategyRepository_Create(t *testing.T) {
	repo := NewMockModeStrategyRepository()
	ctx := context.Background()

	settings := json.RawMessage(`{"leverage": 10}`)

	err := repo.CreateModeStrategySettings(ctx, "1", "scalp", "trend_following", true, 1, settings)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify it was created
	row, err := repo.GetModeStrategySettings(ctx, "1", "scalp", "trend_following")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if row == nil {
		t.Fatal("Expected row, got nil")
	}
	if row.Mode != "scalp" {
		t.Errorf("Expected Mode='scalp', got '%s'", row.Mode)
	}
	if row.Strategy != "trend_following" {
		t.Errorf("Expected Strategy='trend_following', got '%s'", row.Strategy)
	}
	if !row.Enabled {
		t.Error("Expected Enabled=true")
	}
}

// TestMockModeStrategyRepository_GetNotFound tests getting a non-existent configuration.
func TestMockModeStrategyRepository_GetNotFound(t *testing.T) {
	repo := NewMockModeStrategyRepository()
	ctx := context.Background()

	row, err := repo.GetModeStrategySettings(ctx, "999", "scalp", "trend_following")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if row != nil {
		t.Error("Expected nil for non-existent record")
	}
}

// TestMockModeStrategyRepository_GetAll tests getting all configurations for a user.
func TestMockModeStrategyRepository_GetAll(t *testing.T) {
	repo := NewMockModeStrategyRepository()
	ctx := context.Background()

	settings := json.RawMessage(`{}`)

	// Create multiple configs for user 1
	repo.CreateModeStrategySettings(ctx, "1", "scalp", "trend_following", true, 1, settings)
	repo.CreateModeStrategySettings(ctx, "1", "scalp", "mean_reversion", false, 2, settings)
	repo.CreateModeStrategySettings(ctx, "1", "swing", "trend_following", true, 1, settings)

	// Create config for user 2
	repo.CreateModeStrategySettings(ctx, "2", "scalp", "trend_following", true, 1, settings)

	// Get all for user 1
	rows, err := repo.GetAllModeStrategySettings(ctx, "1")
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("Expected 3 rows for user 1, got %d", len(rows))
	}

	// Get all for user 2
	rows, err = repo.GetAllModeStrategySettings(ctx, "2")
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("Expected 1 row for user 2, got %d", len(rows))
	}
}

// TestMockModeStrategyRepository_GetModeStrategies tests getting strategies for a mode.
func TestMockModeStrategyRepository_GetModeStrategies(t *testing.T) {
	repo := NewMockModeStrategyRepository()
	ctx := context.Background()

	settings := json.RawMessage(`{}`)

	// Create all strategies for scalp mode
	repo.CreateModeStrategySettings(ctx, "1", "scalp", "trend_following", true, 1, settings)
	repo.CreateModeStrategySettings(ctx, "1", "scalp", "mean_reversion", false, 2, settings)
	repo.CreateModeStrategySettings(ctx, "1", "scalp", "breakout", false, 3, settings)
	repo.CreateModeStrategySettings(ctx, "1", "scalp", "range_trading", false, 4, settings)

	// Create for swing mode
	repo.CreateModeStrategySettings(ctx, "1", "swing", "trend_following", true, 1, settings)

	// Get scalp strategies
	rows, err := repo.GetModeStrategies(ctx, "1", "scalp")
	if err != nil {
		t.Fatalf("GetModeStrategies failed: %v", err)
	}
	if len(rows) != 4 {
		t.Errorf("Expected 4 strategies for scalp, got %d", len(rows))
	}

	// Get swing strategies
	rows, err = repo.GetModeStrategies(ctx, "1", "swing")
	if err != nil {
		t.Fatalf("GetModeStrategies failed: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("Expected 1 strategy for swing, got %d", len(rows))
	}
}

// TestMockModeStrategyRepository_Update tests updating settings.
func TestMockModeStrategyRepository_Update(t *testing.T) {
	repo := NewMockModeStrategyRepository()
	ctx := context.Background()

	initialSettings := json.RawMessage(`{"leverage": 10}`)
	updatedSettings := json.RawMessage(`{"leverage": 20}`)

	// Create
	repo.CreateModeStrategySettings(ctx, "1", "scalp", "trend_following", true, 1, initialSettings)

	// Update
	err := repo.UpdateModeStrategySettings(ctx, "1", "scalp", "trend_following", updatedSettings)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify update
	row, _ := repo.GetModeStrategySettings(ctx, "1", "scalp", "trend_following")
	var parsed map[string]interface{}
	json.Unmarshal(row.Settings, &parsed)
	if parsed["leverage"].(float64) != 20 {
		t.Errorf("Expected leverage=20 after update, got %v", parsed["leverage"])
	}
}

// TestMockModeStrategyRepository_UpdateNotFound tests updating non-existent record.
func TestMockModeStrategyRepository_UpdateNotFound(t *testing.T) {
	repo := NewMockModeStrategyRepository()
	ctx := context.Background()

	err := repo.UpdateModeStrategySettings(ctx, "999", "scalp", "trend_following", json.RawMessage(`{}`))
	if err == nil {
		t.Error("Expected error for updating non-existent record")
	}
}

// TestMockModeStrategyRepository_UpdateEnabled tests enabling/disabling.
func TestMockModeStrategyRepository_UpdateEnabled(t *testing.T) {
	repo := NewMockModeStrategyRepository()
	ctx := context.Background()

	settings := json.RawMessage(`{}`)

	// Create enabled
	repo.CreateModeStrategySettings(ctx, "1", "scalp", "trend_following", true, 1, settings)

	// Disable
	err := repo.UpdateModeStrategyEnabled(ctx, "1", "scalp", "trend_following", false)
	if err != nil {
		t.Fatalf("UpdateEnabled failed: %v", err)
	}

	// Verify
	row, _ := repo.GetModeStrategySettings(ctx, "1", "scalp", "trend_following")
	if row.Enabled {
		t.Error("Expected Enabled=false after update")
	}

	// Re-enable
	err = repo.UpdateModeStrategyEnabled(ctx, "1", "scalp", "trend_following", true)
	if err != nil {
		t.Fatalf("UpdateEnabled failed: %v", err)
	}

	row, _ = repo.GetModeStrategySettings(ctx, "1", "scalp", "trend_following")
	if !row.Enabled {
		t.Error("Expected Enabled=true after re-enable")
	}
}

// TestMockModeStrategyRepository_Delete tests deleting a configuration.
func TestMockModeStrategyRepository_Delete(t *testing.T) {
	repo := NewMockModeStrategyRepository()
	ctx := context.Background()

	settings := json.RawMessage(`{}`)

	// Create
	repo.CreateModeStrategySettings(ctx, "1", "scalp", "trend_following", true, 1, settings)

	// Delete
	err := repo.DeleteModeStrategySettings(ctx, "1", "scalp", "trend_following")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	row, _ := repo.GetModeStrategySettings(ctx, "1", "scalp", "trend_following")
	if row != nil {
		t.Error("Expected nil after delete")
	}
}

// TestMockModeStrategyRepository_DeleteNotFound tests deleting non-existent record.
func TestMockModeStrategyRepository_DeleteNotFound(t *testing.T) {
	repo := NewMockModeStrategyRepository()
	ctx := context.Background()

	err := repo.DeleteModeStrategySettings(ctx, "999", "scalp", "trend_following")
	if err == nil {
		t.Error("Expected error for deleting non-existent record")
	}
}

// TestMockModeStrategyRepository_BulkCreate tests creating multiple configurations.
func TestMockModeStrategyRepository_BulkCreate(t *testing.T) {
	repo := NewMockModeStrategyRepository()
	ctx := context.Background()

	configs := []ModeStrategySettingsRow{
		{Mode: "scalp", Strategy: "trend_following", Enabled: true, Priority: 1, Settings: json.RawMessage(`{"leverage": 10}`)},
		{Mode: "scalp", Strategy: "mean_reversion", Enabled: false, Priority: 2, Settings: json.RawMessage(`{"leverage": 5}`)},
		{Mode: "swing", Strategy: "trend_following", Enabled: true, Priority: 1, Settings: json.RawMessage(`{"leverage": 5}`)},
	}

	err := repo.BulkCreateModeStrategySettings(ctx, "1", configs)
	if err != nil {
		t.Fatalf("BulkCreate failed: %v", err)
	}

	// Verify all created
	rows, _ := repo.GetAllModeStrategySettings(ctx, "1")
	if len(rows) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(rows))
	}
}

// TestMockModeStrategyRepository_BulkCreate_Upsert tests upsert behavior.
func TestMockModeStrategyRepository_BulkCreate_Upsert(t *testing.T) {
	repo := NewMockModeStrategyRepository()
	ctx := context.Background()

	// Initial create
	initialConfigs := []ModeStrategySettingsRow{
		{Mode: "scalp", Strategy: "trend_following", Enabled: true, Priority: 1, Settings: json.RawMessage(`{"leverage": 10}`)},
	}
	repo.BulkCreateModeStrategySettings(ctx, "1", initialConfigs)

	// Upsert with updated settings
	updatedConfigs := []ModeStrategySettingsRow{
		{Mode: "scalp", Strategy: "trend_following", Enabled: false, Priority: 2, Settings: json.RawMessage(`{"leverage": 20}`)},
	}
	err := repo.BulkCreateModeStrategySettings(ctx, "1", updatedConfigs)
	if err != nil {
		t.Fatalf("BulkCreate upsert failed: %v", err)
	}

	// Verify upsert (should have updated, not added new)
	rows, _ := repo.GetAllModeStrategySettings(ctx, "1")
	if len(rows) != 1 {
		t.Errorf("Expected 1 row after upsert, got %d", len(rows))
	}

	row, _ := repo.GetModeStrategySettings(ctx, "1", "scalp", "trend_following")
	if row.Enabled {
		t.Error("Expected Enabled=false after upsert")
	}
	if row.Priority != 2 {
		t.Errorf("Expected Priority=2 after upsert, got %d", row.Priority)
	}
}

// TestMockModeStrategyRepository_AllModeStrategyCombinations tests 4x4=16 combinations.
func TestMockModeStrategyRepository_AllModeStrategyCombinations(t *testing.T) {
	repo := NewMockModeStrategyRepository()
	ctx := context.Background()

	modes := []string{"scalp", "swing", "position", "ultra_fast"}
	strategies := []string{"trend_following", "mean_reversion", "breakout", "range_trading"}

	// Create all 16 combinations
	var configs []ModeStrategySettingsRow
	for _, mode := range modes {
		for i, strategy := range strategies {
			enabled := strategy == "trend_following" // Only trend_following enabled by default
			configs = append(configs, ModeStrategySettingsRow{
				Mode:     mode,
				Strategy: strategy,
				Enabled:  enabled,
				Priority: i + 1,
				Settings: json.RawMessage(`{}`),
			})
		}
	}

	err := repo.BulkCreateModeStrategySettings(ctx, "1", configs)
	if err != nil {
		t.Fatalf("BulkCreate failed: %v", err)
	}

	// Verify 16 total
	rows, _ := repo.GetAllModeStrategySettings(ctx, "1")
	if len(rows) != 16 {
		t.Errorf("Expected 16 rows (4 modes x 4 strategies), got %d", len(rows))
	}

	// Verify 4 strategies per mode
	for _, mode := range modes {
		modeRows, _ := repo.GetModeStrategies(ctx, "1", mode)
		if len(modeRows) != 4 {
			t.Errorf("Expected 4 strategies for mode '%s', got %d", mode, len(modeRows))
		}
	}
}

// TestModeStrategySettingsRow_JSONMarshaling tests JSON serialization of settings
func TestModeStrategySettingsRow_JSONMarshaling(t *testing.T) {
	settings := json.RawMessage(`{"leverage": 10, "max_positions": 5, "sltp": {"sl_percent": 1.0}}`)
	row := &ModeStrategySettingsRow{
		ID:       1,
		UserID:   "123",
		Mode:     "scalp",
		Strategy: "trend_following",
		Enabled:  true,
		Priority: 1,
		Settings: settings,
	}

	// Test marshaling
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Test unmarshaling
	var parsed ModeStrategySettingsRow
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if parsed.Mode != row.Mode {
		t.Errorf("Expected Mode=%s, got %s", row.Mode, parsed.Mode)
	}
	if parsed.Strategy != row.Strategy {
		t.Errorf("Expected Strategy=%s, got %s", row.Strategy, parsed.Strategy)
	}
}
