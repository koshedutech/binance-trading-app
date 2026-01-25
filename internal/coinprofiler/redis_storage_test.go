// Package coinprofiler provides Redis storage tests for Coin Profiler data.
// Epic 14: Chain Trading System
// Story 14.5: Redis Storage for Coin Profiler data - Unit Tests
package coinprofiler

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// MockLogger implements Logger interface for testing
type MockLogger struct {
	mu     sync.Mutex
	debugs []LogEntry
	infos  []LogEntry
	warns  []LogEntry
	errors []LogEntry
}

type LogEntry struct {
	Msg           string
	KeysAndValues []interface{}
}

func NewMockLogger() *MockLogger {
	return &MockLogger{}
}

func (m *MockLogger) Debug(msg string, keysAndValues ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.debugs = append(m.debugs, LogEntry{Msg: msg, KeysAndValues: keysAndValues})
}

func (m *MockLogger) Info(msg string, keysAndValues ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.infos = append(m.infos, LogEntry{Msg: msg, KeysAndValues: keysAndValues})
}

func (m *MockLogger) Warn(msg string, keysAndValues ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.warns = append(m.warns, LogEntry{Msg: msg, KeysAndValues: keysAndValues})
}

func (m *MockLogger) Error(msg string, keysAndValues ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors = append(m.errors, LogEntry{Msg: msg, KeysAndValues: keysAndValues})
}

// MockRedisClient implements RedisClient interface for testing
type MockRedisClient struct {
	data        map[string]string
	sets        map[string]map[string]struct{}
	mu          sync.RWMutex
	failPing    bool
	failGet     bool
	failSet     bool
	failDel     bool
	failMGet    bool
	failSAdd    bool
	failSRem    bool
	failSMembers bool
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string]string),
		sets: make(map[string]map[string]struct{}),
	}
}

func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cmd := redis.NewStringCmd(ctx, "get", key)
	if m.failGet {
		cmd.SetErr(redis.Nil)
		return cmd
	}

	if val, ok := m.data[key]; ok {
		cmd.SetVal(val)
	} else {
		cmd.SetErr(redis.Nil)
	}
	return cmd
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd := redis.NewStatusCmd(ctx, "set", key, value)
	if m.failSet {
		cmd.SetErr(redis.ErrClosed)
		return cmd
	}

	switch v := value.(type) {
	case string:
		m.data[key] = v
	case []byte:
		m.data[key] = string(v)
	default:
		m.data[key] = ""
	}
	cmd.SetVal("OK")
	return cmd
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd := redis.NewIntCmd(ctx, "del", keys)
	if m.failDel {
		cmd.SetErr(redis.ErrClosed)
		return cmd
	}

	count := int64(0)
	for _, key := range keys {
		if _, ok := m.data[key]; ok {
			delete(m.data, key)
			count++
		}
	}
	cmd.SetVal(count)
	return cmd
}

func (m *MockRedisClient) MGet(ctx context.Context, keys ...string) *redis.SliceCmd {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cmd := redis.NewSliceCmd(ctx, "mget", keys)
	if m.failMGet {
		cmd.SetErr(redis.ErrClosed)
		return cmd
	}

	result := make([]interface{}, len(keys))
	for i, key := range keys {
		if val, ok := m.data[key]; ok {
			result[i] = val
		} else {
			result[i] = nil
		}
	}
	cmd.SetVal(result)
	return cmd
}

func (m *MockRedisClient) SAdd(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd := redis.NewIntCmd(ctx, "sadd", key)
	if m.failSAdd {
		cmd.SetErr(redis.ErrClosed)
		return cmd
	}

	if m.sets[key] == nil {
		m.sets[key] = make(map[string]struct{})
	}
	count := int64(0)
	for _, member := range members {
		memberStr, ok := member.(string)
		if !ok {
			continue
		}
		if _, exists := m.sets[key][memberStr]; !exists {
			m.sets[key][memberStr] = struct{}{}
			count++
		}
	}
	cmd.SetVal(count)
	return cmd
}

func (m *MockRedisClient) SRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd := redis.NewIntCmd(ctx, "srem", key)
	if m.failSRem {
		cmd.SetErr(redis.ErrClosed)
		return cmd
	}

	count := int64(0)
	if m.sets[key] != nil {
		for _, member := range members {
			memberStr, ok := member.(string)
			if !ok {
				continue
			}
			if _, exists := m.sets[key][memberStr]; exists {
				delete(m.sets[key], memberStr)
				count++
			}
		}
	}
	cmd.SetVal(count)
	return cmd
}

func (m *MockRedisClient) SMembers(ctx context.Context, key string) *redis.StringSliceCmd {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cmd := redis.NewStringSliceCmd(ctx, "smembers", key)
	if m.failSMembers {
		cmd.SetErr(redis.ErrClosed)
		return cmd
	}

	var members []string
	if m.sets[key] != nil {
		for member := range m.sets[key] {
			members = append(members, member)
		}
	}
	cmd.SetVal(members)
	return cmd
}

func (m *MockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx, "ping")
	if m.failPing {
		cmd.SetErr(redis.ErrClosed)
		return cmd
	}
	cmd.SetVal("PONG")
	return cmd
}

// TestCoinDataKey tests the key generation function
func TestCoinDataKey(t *testing.T) {
	tests := []struct {
		name     string
		symbol   string
		expected string
	}{
		{
			name:     "standard symbol",
			symbol:   "BTCUSDT",
			expected: "coinprofiler:BTCUSDT:data",
		},
		{
			name:     "another symbol",
			symbol:   "ETHUSDT",
			expected: "coinprofiler:ETHUSDT:data",
		},
		{
			name:     "empty symbol",
			symbol:   "",
			expected: "coinprofiler::data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CoinDataKey(tt.symbol)
			if result != tt.expected {
				t.Errorf("CoinDataKey(%s) = %s, want %s", tt.symbol, result, tt.expected)
			}
		})
	}
}

// TestTimeframeDataKey tests the timeframe key generation function
func TestTimeframeDataKey(t *testing.T) {
	tests := []struct {
		name      string
		symbol    string
		timeframe string
		expected  string
	}{
		{
			name:      "5m timeframe",
			symbol:    "BTCUSDT",
			timeframe: "5m",
			expected:  "coinprofiler:BTCUSDT:5m",
		},
		{
			name:      "15m timeframe",
			symbol:    "ETHUSDT",
			timeframe: "15m",
			expected:  "coinprofiler:ETHUSDT:15m",
		},
		{
			name:      "1h timeframe",
			symbol:    "XRPUSDT",
			timeframe: "1h",
			expected:  "coinprofiler:XRPUSDT:1h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TimeframeDataKey(tt.symbol, tt.timeframe)
			if result != tt.expected {
				t.Errorf("TimeframeDataKey(%s, %s) = %s, want %s", tt.symbol, tt.timeframe, result, tt.expected)
			}
		})
	}
}

// TestCoinDataJSONMarshal tests JSON serialization of CoinData
func TestCoinDataJSONMarshal(t *testing.T) {
	now := time.Now()
	data := &CoinData{
		Symbol:     "BTCUSDT",
		Price:      97500.50,
		Volume24h:  1500000.0,
		Volatility: 2.5,
		Timeframes: map[string]*TimeframeData{
			"5m": {
				Timeframe:    "5m",
				Open:         97400.0,
				High:         97600.0,
				Low:          97350.0,
				Close:        97500.0,
				Volume:       500.0,
				TakerBuyVol:  250.0,
				TakerSellVol: 250.0,
				TradeCount:   1000,
				IsClosedBar:  true,
				UpdatedAt:    now,
			},
		},
		Source:     DataSourceStrategy,
		Strategies: []string{"ultra_scalp", "swing_trade"},
		UpdatedAt:  now,
	}

	// Marshal
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal CoinData: %v", err)
	}

	// Unmarshal
	var decoded CoinData
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal CoinData: %v", err)
	}

	// Verify fields
	if decoded.Symbol != data.Symbol {
		t.Errorf("Symbol mismatch: got %s, want %s", decoded.Symbol, data.Symbol)
	}
	if decoded.Price != data.Price {
		t.Errorf("Price mismatch: got %f, want %f", decoded.Price, data.Price)
	}
	if decoded.Volume24h != data.Volume24h {
		t.Errorf("Volume24h mismatch: got %f, want %f", decoded.Volume24h, data.Volume24h)
	}
	if decoded.Volatility != data.Volatility {
		t.Errorf("Volatility mismatch: got %f, want %f", decoded.Volatility, data.Volatility)
	}
	if len(decoded.Timeframes) != 1 {
		t.Errorf("Timeframes count mismatch: got %d, want 1", len(decoded.Timeframes))
	}
	if len(decoded.Strategies) != 2 {
		t.Errorf("Strategies count mismatch: got %d, want 2", len(decoded.Strategies))
	}
}

// TestTimeframeDataJSONMarshal tests JSON serialization of TimeframeData
func TestTimeframeDataJSONMarshal(t *testing.T) {
	now := time.Now()
	data := &TimeframeData{
		Timeframe:    "15m",
		Open:         97400.0,
		High:         97700.0,
		Low:          97300.0,
		Close:        97550.0,
		Volume:       1200.5,
		TakerBuyVol:  600.0,
		TakerSellVol: 600.5,
		QuoteVolume:  117000000.0,
		TradeCount:   5000,
		IsClosedBar:  false,
		OpenTime:     now.Add(-15 * time.Minute),
		CloseTime:    now,
		UpdatedAt:    now,
	}

	// Marshal
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal TimeframeData: %v", err)
	}

	// Unmarshal
	var decoded TimeframeData
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal TimeframeData: %v", err)
	}

	// Verify fields
	if decoded.Timeframe != data.Timeframe {
		t.Errorf("Timeframe mismatch: got %s, want %s", decoded.Timeframe, data.Timeframe)
	}
	if decoded.Open != data.Open {
		t.Errorf("Open mismatch: got %f, want %f", decoded.Open, data.Open)
	}
	if decoded.High != data.High {
		t.Errorf("High mismatch: got %f, want %f", decoded.High, data.High)
	}
	if decoded.Low != data.Low {
		t.Errorf("Low mismatch: got %f, want %f", decoded.Low, data.Low)
	}
	if decoded.Close != data.Close {
		t.Errorf("Close mismatch: got %f, want %f", decoded.Close, data.Close)
	}
	if decoded.Volume != data.Volume {
		t.Errorf("Volume mismatch: got %f, want %f", decoded.Volume, data.Volume)
	}
	if decoded.TradeCount != data.TradeCount {
		t.Errorf("TradeCount mismatch: got %d, want %d", decoded.TradeCount, data.TradeCount)
	}
	if decoded.IsClosedBar != data.IsClosedBar {
		t.Errorf("IsClosedBar mismatch: got %v, want %v", decoded.IsClosedBar, data.IsClosedBar)
	}
}

// TestDefaultStorageConfig tests the default configuration
func TestDefaultStorageConfig(t *testing.T) {
	config := DefaultStorageConfig()

	if config.TTL != DefaultCoinDataTTL {
		t.Errorf("Default TTL = %v, want %v", config.TTL, DefaultCoinDataTTL)
	}
}

// TestDefaultCoinDataTTL tests the default TTL constant
func TestDefaultCoinDataTTL(t *testing.T) {
	if DefaultCoinDataTTL != 5*time.Minute {
		t.Errorf("DefaultCoinDataTTL = %v, want 5 minutes", DefaultCoinDataTTL)
	}
}

// TestDefaultTimeframeTTL tests the default timeframe TTL constant
func TestDefaultTimeframeTTL(t *testing.T) {
	if DefaultTimeframeTTL != 5*time.Minute {
		t.Errorf("DefaultTimeframeTTL = %v, want 5 minutes", DefaultTimeframeTTL)
	}
}

// TestKeyTrackedSymbols tests the tracked symbols key constant
func TestKeyTrackedSymbols(t *testing.T) {
	if KeyTrackedSymbols != "coinprofiler:symbols" {
		t.Errorf("KeyTrackedSymbols = %s, want coinprofiler:symbols", KeyTrackedSymbols)
	}
}

// TestRedisCoinDataStorageNilClient tests behavior with nil Redis client
func TestRedisCoinDataStorageNilClient(t *testing.T) {
	logger := NewMockLogger()
	storage := NewRedisCoinDataStorage(nil, nil, logger)

	ctx := context.Background()

	// IsHealthy should return false
	if storage.IsHealthy() {
		t.Error("IsHealthy() should return false for nil client")
	}

	// SaveCoinData should return error
	err := storage.SaveCoinData(ctx, &CoinData{Symbol: "BTCUSDT"})
	if err == nil {
		t.Error("SaveCoinData should return error for nil client")
	}

	// GetCoinData should return error
	_, err = storage.GetCoinData(ctx, "BTCUSDT")
	if err == nil {
		t.Error("GetCoinData should return error for nil client")
	}

	// GetAllCoinData should return error
	_, err = storage.GetAllCoinData(ctx)
	if err == nil {
		t.Error("GetAllCoinData should return error for nil client")
	}

	// GetTrackedSymbols should return error
	_, err = storage.GetTrackedSymbols(ctx)
	if err == nil {
		t.Error("GetTrackedSymbols should return error for nil client")
	}

	// SaveBatch should return error
	err = storage.SaveBatch(ctx, map[string]*CoinData{"BTCUSDT": {Symbol: "BTCUSDT"}})
	if err == nil {
		t.Error("SaveBatch should return error for nil client")
	}

	// GetBatch should return error
	_, err = storage.GetBatch(ctx, []string{"BTCUSDT"})
	if err == nil {
		t.Error("GetBatch should return error for nil client")
	}

	// SaveTimeframeData should return error
	err = storage.SaveTimeframeData(ctx, "BTCUSDT", &TimeframeData{Timeframe: "5m"})
	if err == nil {
		t.Error("SaveTimeframeData should return error for nil client")
	}

	// GetTimeframeData should return error
	_, err = storage.GetTimeframeData(ctx, "BTCUSDT", "5m")
	if err == nil {
		t.Error("GetTimeframeData should return error for nil client")
	}

	// UpdateCoinDataDelta should return error
	err = storage.UpdateCoinDataDelta(ctx, "BTCUSDT", map[string]interface{}{"price": 100.0})
	if err == nil {
		t.Error("UpdateCoinDataDelta should return error for nil client")
	}
}

// TestRedisCoinDataStorageSaveAndGet tests save and get operations
func TestRedisCoinDataStorageSaveAndGet(t *testing.T) {
	mockClient := NewMockRedisClient()
	logger := NewMockLogger()
	storage := NewRedisCoinDataStorage(mockClient, nil, logger)

	ctx := context.Background()

	// Create test data
	data := &CoinData{
		Symbol:     "BTCUSDT",
		Price:      97500.50,
		Volume24h:  1500000.0,
		Volatility: 2.5,
		Timeframes: map[string]*TimeframeData{
			"5m": {
				Timeframe: "5m",
				Close:     97500.0,
			},
		},
		Source:    DataSourceStrategy,
		UpdatedAt: time.Now(),
	}

	// Save
	err := storage.SaveCoinData(ctx, data)
	if err != nil {
		t.Fatalf("SaveCoinData failed: %v", err)
	}

	// Get
	retrieved, err := storage.GetCoinData(ctx, "BTCUSDT")
	if err != nil {
		t.Fatalf("GetCoinData failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("GetCoinData returned nil")
	}

	if retrieved.Symbol != data.Symbol {
		t.Errorf("Symbol mismatch: got %s, want %s", retrieved.Symbol, data.Symbol)
	}
	if retrieved.Price != data.Price {
		t.Errorf("Price mismatch: got %f, want %f", retrieved.Price, data.Price)
	}
}

// TestRedisCoinDataStorageGetMiss tests cache miss behavior
func TestRedisCoinDataStorageGetMiss(t *testing.T) {
	mockClient := NewMockRedisClient()
	logger := NewMockLogger()
	storage := NewRedisCoinDataStorage(mockClient, nil, logger)

	ctx := context.Background()

	// Get non-existent data
	retrieved, err := storage.GetCoinData(ctx, "NONEXISTENT")
	if err != nil {
		t.Fatalf("GetCoinData should not return error for miss: %v", err)
	}

	if retrieved != nil {
		t.Error("GetCoinData should return nil for cache miss")
	}
}

// TestRedisCoinDataStorageDelete tests delete operation
func TestRedisCoinDataStorageDelete(t *testing.T) {
	mockClient := NewMockRedisClient()
	logger := NewMockLogger()
	storage := NewRedisCoinDataStorage(mockClient, nil, logger)

	ctx := context.Background()

	// Save data first
	data := &CoinData{
		Symbol: "BTCUSDT",
		Price:  97500.0,
	}
	_ = storage.SaveCoinData(ctx, data)

	// Delete
	err := storage.DeleteCoinData(ctx, "BTCUSDT")
	if err != nil {
		t.Fatalf("DeleteCoinData failed: %v", err)
	}

	// Verify deleted
	retrieved, _ := storage.GetCoinData(ctx, "BTCUSDT")
	if retrieved != nil {
		t.Error("Data should be deleted")
	}
}

// TestRedisCoinDataStorageTrackedSymbols tests tracked symbols set operations
func TestRedisCoinDataStorageTrackedSymbols(t *testing.T) {
	mockClient := NewMockRedisClient()
	logger := NewMockLogger()
	storage := NewRedisCoinDataStorage(mockClient, nil, logger)

	ctx := context.Background()

	// Save multiple symbols
	symbols := []string{"BTCUSDT", "ETHUSDT", "XRPUSDT"}
	for _, symbol := range symbols {
		err := storage.SaveCoinData(ctx, &CoinData{Symbol: symbol, Price: 100.0})
		if err != nil {
			t.Fatalf("SaveCoinData failed for %s: %v", symbol, err)
		}
	}

	// Get tracked symbols
	tracked, err := storage.GetTrackedSymbols(ctx)
	if err != nil {
		t.Fatalf("GetTrackedSymbols failed: %v", err)
	}

	if len(tracked) != len(symbols) {
		t.Errorf("Expected %d tracked symbols, got %d", len(symbols), len(tracked))
	}
}

// TestRedisCoinDataStorageBatchOperations tests batch save and get
func TestRedisCoinDataStorageBatchOperations(t *testing.T) {
	mockClient := NewMockRedisClient()
	logger := NewMockLogger()
	storage := NewRedisCoinDataStorage(mockClient, nil, logger)

	ctx := context.Background()

	// Create batch data
	batchData := map[string]*CoinData{
		"BTCUSDT": {Symbol: "BTCUSDT", Price: 97500.0},
		"ETHUSDT": {Symbol: "ETHUSDT", Price: 3500.0},
		"XRPUSDT": {Symbol: "XRPUSDT", Price: 0.5},
	}

	// Save batch
	err := storage.SaveBatch(ctx, batchData)
	if err != nil {
		t.Fatalf("SaveBatch failed: %v", err)
	}

	// Get batch
	symbols := []string{"BTCUSDT", "ETHUSDT", "XRPUSDT"}
	retrieved, err := storage.GetBatch(ctx, symbols)
	if err != nil {
		t.Fatalf("GetBatch failed: %v", err)
	}

	if len(retrieved) != len(symbols) {
		t.Errorf("Expected %d results, got %d", len(symbols), len(retrieved))
	}

	for symbol, expected := range batchData {
		if actual, ok := retrieved[symbol]; ok {
			if actual.Price != expected.Price {
				t.Errorf("%s price mismatch: got %f, want %f", symbol, actual.Price, expected.Price)
			}
		} else {
			t.Errorf("%s not found in batch results", symbol)
		}
	}
}

// TestRedisCoinDataStorageGetTTL tests the GetTTL method
func TestRedisCoinDataStorageGetTTL(t *testing.T) {
	mockClient := NewMockRedisClient()
	logger := NewMockLogger()
	storage := NewRedisCoinDataStorage(mockClient, nil, logger)

	ttl := storage.GetTTL()
	if ttl != DefaultCoinDataTTL {
		t.Errorf("GetTTL() = %v, want %v", ttl, DefaultCoinDataTTL)
	}
}

// TestRedisCoinDataStorageSetTTL tests the SetTTL method
func TestRedisCoinDataStorageSetTTL(t *testing.T) {
	mockClient := NewMockRedisClient()
	logger := NewMockLogger()
	storage := NewRedisCoinDataStorage(mockClient, nil, logger)

	// Set a new TTL
	newTTL := 10 * time.Minute
	storage.SetTTL(newTTL)

	if storage.GetTTL() != newTTL {
		t.Errorf("GetTTL() after SetTTL = %v, want %v", storage.GetTTL(), newTTL)
	}

	// Test that zero TTL is not set
	storage.SetTTL(0)
	if storage.GetTTL() != newTTL {
		t.Errorf("SetTTL(0) should not change TTL, got %v, want %v", storage.GetTTL(), newTTL)
	}

	// Test that negative TTL is not set
	storage.SetTTL(-1 * time.Minute)
	if storage.GetTTL() != newTTL {
		t.Errorf("SetTTL(negative) should not change TTL, got %v, want %v", storage.GetTTL(), newTTL)
	}
}

// TestNewRedisCoinDataStorageWithConfig tests creating storage with custom config
func TestNewRedisCoinDataStorageWithConfig(t *testing.T) {
	mockClient := NewMockRedisClient()
	logger := NewMockLogger()

	// Test with custom TTL
	config := &StorageConfig{
		TTL: 10 * time.Minute,
	}
	storage := NewRedisCoinDataStorage(mockClient, config, logger)

	if storage.GetTTL() != 10*time.Minute {
		t.Errorf("Storage TTL = %v, want 10 minutes", storage.GetTTL())
	}

	// Test with zero TTL (should default)
	configZero := &StorageConfig{
		TTL: 0,
	}
	storageZero := NewRedisCoinDataStorage(mockClient, configZero, logger)

	if storageZero.GetTTL() != DefaultCoinDataTTL {
		t.Errorf("Storage TTL with zero config = %v, want %v", storageZero.GetTTL(), DefaultCoinDataTTL)
	}
}

// TestRedisCoinDataStorageEmptyBatch tests batch operations with empty inputs
func TestRedisCoinDataStorageEmptyBatch(t *testing.T) {
	mockClient := NewMockRedisClient()
	logger := NewMockLogger()
	storage := NewRedisCoinDataStorage(mockClient, nil, logger)

	ctx := context.Background()

	// SaveBatch with empty map should succeed
	err := storage.SaveBatch(ctx, map[string]*CoinData{})
	if err != nil {
		t.Errorf("SaveBatch with empty map should not return error: %v", err)
	}

	// GetBatch with empty slice should return empty map
	result, err := storage.GetBatch(ctx, []string{})
	if err != nil {
		t.Errorf("GetBatch with empty slice should not return error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("GetBatch with empty slice should return empty map, got %d items", len(result))
	}
}

// TestMockLoggerCapture tests that the mock logger captures log entries
func TestMockLoggerCapture(t *testing.T) {
	logger := NewMockLogger()

	logger.Debug("debug message", "key", "value")
	logger.Info("info message", "count", 5)
	logger.Warn("warn message")
	logger.Error("error message", "err", "some error")

	if len(logger.debugs) != 1 {
		t.Errorf("Expected 1 debug entry, got %d", len(logger.debugs))
	}
	if len(logger.infos) != 1 {
		t.Errorf("Expected 1 info entry, got %d", len(logger.infos))
	}
	if len(logger.warns) != 1 {
		t.Errorf("Expected 1 warn entry, got %d", len(logger.warns))
	}
	if len(logger.errors) != 1 {
		t.Errorf("Expected 1 error entry, got %d", len(logger.errors))
	}

	if logger.debugs[0].Msg != "debug message" {
		t.Errorf("Debug message mismatch: got %s", logger.debugs[0].Msg)
	}
	if logger.infos[0].Msg != "info message" {
		t.Errorf("Info message mismatch: got %s", logger.infos[0].Msg)
	}
	if logger.warns[0].Msg != "warn message" {
		t.Errorf("Warn message mismatch: got %s", logger.warns[0].Msg)
	}
	if logger.errors[0].Msg != "error message" {
		t.Errorf("Error message mismatch: got %s", logger.errors[0].Msg)
	}
}

// TestCoinDataStorageInterfaceCompliance tests that RedisCoinDataStorage implements CoinDataStorage
func TestCoinDataStorageInterfaceCompliance(t *testing.T) {
	// This is a compile-time check
	var _ CoinDataStorage = (*RedisCoinDataStorage)(nil)
}

// TestRedisCoinDataStorageNilLogger tests that operations work without a logger
func TestRedisCoinDataStorageNilLogger(t *testing.T) {
	mockClient := NewMockRedisClient()
	storage := NewRedisCoinDataStorage(mockClient, nil, nil)

	ctx := context.Background()

	// These should not panic even with nil logger
	_ = storage.SaveCoinData(ctx, &CoinData{Symbol: "BTCUSDT"})
	_, _ = storage.GetCoinData(ctx, "BTCUSDT")
	_, _ = storage.GetAllCoinData(ctx)
	_ = storage.DeleteCoinData(ctx, "BTCUSDT")

	// Verify no panic occurred
	t.Log("Operations with nil logger completed without panic")
}

// TestKeyPrefixConstants tests that key prefix constants are correctly defined
func TestKeyPrefixConstants(t *testing.T) {
	if PrefixCoinData != "coinprofiler:%s:data" {
		t.Errorf("PrefixCoinData = %s, want coinprofiler:%%s:data", PrefixCoinData)
	}
	if PrefixTimeframeData != "coinprofiler:%s:%s" {
		t.Errorf("PrefixTimeframeData = %s, want coinprofiler:%%s:%%s", PrefixTimeframeData)
	}
}

// TestMultipleTimeframes tests CoinData with multiple timeframes
func TestMultipleTimeframes(t *testing.T) {
	now := time.Now()
	data := &CoinData{
		Symbol: "BTCUSDT",
		Price:  97500.0,
		Timeframes: map[string]*TimeframeData{
			"5m": {
				Timeframe: "5m",
				Close:     97500.0,
				UpdatedAt: now,
			},
			"15m": {
				Timeframe: "15m",
				Close:     97480.0,
				UpdatedAt: now,
			},
			"1h": {
				Timeframe: "1h",
				Close:     97400.0,
				UpdatedAt: now,
			},
		},
		UpdatedAt: now,
	}

	// Marshal and unmarshal
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded CoinData
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify all timeframes are present
	if len(decoded.Timeframes) != 3 {
		t.Errorf("Expected 3 timeframes, got %d", len(decoded.Timeframes))
	}

	for _, tf := range []string{"5m", "15m", "1h"} {
		if _, exists := decoded.Timeframes[tf]; !exists {
			t.Errorf("Missing timeframe: %s", tf)
		}
	}
}

// TestDataSourceConstants tests DataSource constants
func TestDataSourceConstants(t *testing.T) {
	tests := []struct {
		source   DataSource
		expected string
	}{
		{DataSourceStrategy, "strategy"},
		{DataSourcePosition, "position"},
		{DataSourceBoth, "both"},
	}

	for _, tt := range tests {
		t.Run(string(tt.source), func(t *testing.T) {
			if string(tt.source) != tt.expected {
				t.Errorf("DataSource constant %v = %s, want %s", tt.source, string(tt.source), tt.expected)
			}
		})
	}
}

// TestRedisCoinDataStorageTimeframeData tests timeframe data operations
func TestRedisCoinDataStorageTimeframeData(t *testing.T) {
	mockClient := NewMockRedisClient()
	logger := NewMockLogger()
	storage := NewRedisCoinDataStorage(mockClient, nil, logger)

	ctx := context.Background()

	// Save timeframe data
	tfData := &TimeframeData{
		Timeframe: "5m",
		Open:      97400.0,
		High:      97600.0,
		Low:       97350.0,
		Close:     97500.0,
		Volume:    500.0,
	}

	err := storage.SaveTimeframeData(ctx, "BTCUSDT", tfData)
	if err != nil {
		t.Fatalf("SaveTimeframeData failed: %v", err)
	}

	// Get timeframe data
	retrieved, err := storage.GetTimeframeData(ctx, "BTCUSDT", "5m")
	if err != nil {
		t.Fatalf("GetTimeframeData failed: %v", err)
	}

	if retrieved == nil {
		t.Fatal("GetTimeframeData returned nil")
	}

	if retrieved.Close != tfData.Close {
		t.Errorf("Close price mismatch: got %f, want %f", retrieved.Close, tfData.Close)
	}
}

// TestRedisCoinDataStorageIsHealthy tests health check behavior
func TestRedisCoinDataStorageIsHealthy(t *testing.T) {
	mockClient := NewMockRedisClient()
	logger := NewMockLogger()
	storage := NewRedisCoinDataStorage(mockClient, nil, logger)

	// Should be healthy with working mock client
	if !storage.IsHealthy() {
		t.Error("Storage should be healthy with working client")
	}

	// Make ping fail
	mockClient.failPing = true
	if storage.IsHealthy() {
		t.Error("Storage should not be healthy when ping fails")
	}
}

// TestRedisCoinDataStorageGetClient tests GetClient method
func TestRedisCoinDataStorageGetClient(t *testing.T) {
	mockClient := NewMockRedisClient()
	logger := NewMockLogger()
	storage := NewRedisCoinDataStorage(mockClient, nil, logger)

	client := storage.GetClient()
	if client != mockClient {
		t.Error("GetClient should return the injected client")
	}
}

// TestRedisCoinDataStorageInvalidInputs tests handling of invalid inputs
func TestRedisCoinDataStorageInvalidInputs(t *testing.T) {
	mockClient := NewMockRedisClient()
	logger := NewMockLogger()
	storage := NewRedisCoinDataStorage(mockClient, nil, logger)

	ctx := context.Background()

	// Test nil data
	err := storage.SaveCoinData(ctx, nil)
	if err == nil {
		t.Error("SaveCoinData(nil) should return error")
	}

	// Test empty symbol
	err = storage.SaveCoinData(ctx, &CoinData{Symbol: ""})
	if err == nil {
		t.Error("SaveCoinData with empty symbol should return error")
	}

	// Test GetCoinData with empty symbol
	_, err = storage.GetCoinData(ctx, "")
	if err == nil {
		t.Error("GetCoinData with empty symbol should return error")
	}

	// Test DeleteCoinData with empty symbol
	err = storage.DeleteCoinData(ctx, "")
	if err == nil {
		t.Error("DeleteCoinData with empty symbol should return error")
	}

	// Test SaveTimeframeData with empty symbol
	err = storage.SaveTimeframeData(ctx, "", &TimeframeData{Timeframe: "5m"})
	if err == nil {
		t.Error("SaveTimeframeData with empty symbol should return error")
	}

	// Test SaveTimeframeData with empty timeframe
	err = storage.SaveTimeframeData(ctx, "BTCUSDT", &TimeframeData{Timeframe: ""})
	if err == nil {
		t.Error("SaveTimeframeData with empty timeframe should return error")
	}

	// Test GetTimeframeData with empty inputs
	_, err = storage.GetTimeframeData(ctx, "", "5m")
	if err == nil {
		t.Error("GetTimeframeData with empty symbol should return error")
	}

	_, err = storage.GetTimeframeData(ctx, "BTCUSDT", "")
	if err == nil {
		t.Error("GetTimeframeData with empty timeframe should return error")
	}
}
