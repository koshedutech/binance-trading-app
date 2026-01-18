// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.17: Calibration Layer
package decision

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis key prefix for calibration data
const (
	// PrefixCalibration is the Redis key pattern for calibration data
	// Format: decision:calibration:{userID}:{strategy}:{indicatorHash}
	PrefixCalibration = "decision:calibration:%d:%s:%s"

	// DefaultCalibrationTTL is the default TTL for calibration data (7 days)
	DefaultCalibrationTTL = 7 * 24 * time.Hour

	// DefaultMinTradesForReliable is the minimum number of trades for calibration to be reliable
	DefaultMinTradesForReliable = 50
)

// Score bucket boundaries
const (
	BucketLow        = "0-24"
	BucketMediumLow  = "25-49"
	BucketMediumHigh = "50-74"
	BucketHigh       = "75-100"
)

// ScoreBucket represents a calibration score range with tracked outcomes.
type ScoreBucket struct {
	// MinScore is the minimum score (inclusive) for this bucket
	MinScore int `json:"min_score"`
	// MaxScore is the maximum score (inclusive) for this bucket
	MaxScore int `json:"max_score"`
	// TotalTrades is the total number of trades in this bucket
	TotalTrades int64 `json:"total_trades"`
	// WinningTrades is the number of winning trades in this bucket
	WinningTrades int64 `json:"winning_trades"`
	// WinRate is the calculated win rate (WinningTrades / TotalTrades)
	WinRate float64 `json:"win_rate"`
	// AvgPnLPercent is the average P&L percentage for trades in this bucket
	AvgPnLPercent float64 `json:"avg_pnl_percent"`
	// TotalPnLPercent is the sum of all P&L percentages (for calculating average)
	TotalPnLPercent float64 `json:"total_pnl_percent"`
	// LastUpdated is when this bucket was last updated
	LastUpdated time.Time `json:"last_updated"`
}

// RecordOutcome records a trade outcome and recalculates metrics.
func (sb *ScoreBucket) RecordOutcome(won bool, pnlPercent float64) {
	sb.TotalTrades++
	if won {
		sb.WinningTrades++
	}
	sb.TotalPnLPercent += pnlPercent

	// Recalculate metrics
	if sb.TotalTrades > 0 {
		sb.WinRate = float64(sb.WinningTrades) / float64(sb.TotalTrades)
		sb.AvgPnLPercent = sb.TotalPnLPercent / float64(sb.TotalTrades)
	}
	sb.LastUpdated = time.Now()
}

// CalibrationData holds all buckets for a user's strategy and indicator configuration.
type CalibrationData struct {
	// UserID is the user who owns this calibration data
	UserID int `json:"user_id"`
	// Strategy is the trading strategy name
	Strategy string `json:"strategy"`
	// IndicatorHash is the SHA256 hash (first 8 chars) of selected indicators
	IndicatorHash string `json:"indicator_hash"`
	// Buckets maps bucket names to ScoreBucket data
	Buckets map[string]*ScoreBucket `json:"buckets"`
	// TotalTrades is the total number of trades across all buckets
	TotalTrades int64 `json:"total_trades"`
	// FirstTradeDate is when the first trade was recorded
	FirstTradeDate time.Time `json:"first_trade_date"`
	// LastUpdated is when any bucket was last updated
	LastUpdated time.Time `json:"last_updated"`
}

// NewCalibrationData creates a new CalibrationData with default empty buckets.
func NewCalibrationData(userID int, strategy, indicatorHash string) *CalibrationData {
	return &CalibrationData{
		UserID:        userID,
		Strategy:      strategy,
		IndicatorHash: indicatorHash,
		Buckets:       GetDefaultBuckets(),
		TotalTrades:   0,
		LastUpdated:   time.Now(),
	}
}

// GetDefaultBuckets returns a map of initialized empty buckets.
func GetDefaultBuckets() map[string]*ScoreBucket {
	return map[string]*ScoreBucket{
		BucketLow: {
			MinScore: 0,
			MaxScore: 24,
		},
		BucketMediumLow: {
			MinScore: 25,
			MaxScore: 49,
		},
		BucketMediumHigh: {
			MinScore: 50,
			MaxScore: 74,
		},
		BucketHigh: {
			MinScore: 75,
			MaxScore: 100,
		},
	}
}

// RecordOutcome records a trade outcome to the appropriate bucket.
func (cd *CalibrationData) RecordOutcome(score int, won bool, pnlPercent float64) {
	bucketName := GetBucketForScore(score)
	bucket, exists := cd.Buckets[bucketName]
	if !exists {
		// Initialize bucket if missing (shouldn't happen with proper initialization)
		bucket = &ScoreBucket{
			MinScore: getBucketBounds(bucketName)[0],
			MaxScore: getBucketBounds(bucketName)[1],
		}
		cd.Buckets[bucketName] = bucket
	}

	bucket.RecordOutcome(won, pnlPercent)
	cd.TotalTrades++
	cd.LastUpdated = time.Now()

	// Set first trade date if this is the first trade
	if cd.TotalTrades == 1 {
		cd.FirstTradeDate = time.Now()
	}
}

// getBucketBounds returns [minScore, maxScore] for a bucket name.
func getBucketBounds(bucketName string) [2]int {
	switch bucketName {
	case BucketLow:
		return [2]int{0, 24}
	case BucketMediumLow:
		return [2]int{25, 49}
	case BucketMediumHigh:
		return [2]int{50, 74}
	case BucketHigh:
		return [2]int{75, 100}
	default:
		return [2]int{0, 100}
	}
}

// GetBucketForScore returns the bucket name for a given score.
func GetBucketForScore(score int) string {
	// Clamp score to valid range
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	switch {
	case score <= 24:
		return BucketLow
	case score <= 49:
		return BucketMediumLow
	case score <= 74:
		return BucketMediumHigh
	default:
		return BucketHigh
	}
}

// GetCalibratedConfidence returns the historical win rate for the score's bucket.
// Returns 0 if the bucket has no trades.
func (cd *CalibrationData) GetCalibratedConfidence(score int) float64 {
	bucketName := GetBucketForScore(score)
	bucket, exists := cd.Buckets[bucketName]
	if !exists || bucket.TotalTrades == 0 {
		return 0
	}
	return bucket.WinRate
}

// IsReliable returns true if the calibration has enough trades for reliable predictions.
func (cd *CalibrationData) IsReliable(minTrades int) bool {
	if minTrades <= 0 {
		minTrades = DefaultMinTradesForReliable
	}
	return cd.TotalTrades >= int64(minTrades)
}

// Copy creates a deep copy of CalibrationData to prevent external mutation of cached data.
func (cd *CalibrationData) Copy() *CalibrationData {
	if cd == nil {
		return nil
	}

	copiedData := &CalibrationData{
		UserID:         cd.UserID,
		Strategy:       cd.Strategy,
		IndicatorHash:  cd.IndicatorHash,
		TotalTrades:    cd.TotalTrades,
		FirstTradeDate: cd.FirstTradeDate,
		LastUpdated:    cd.LastUpdated,
		Buckets:        make(map[string]*ScoreBucket),
	}

	for k, v := range cd.Buckets {
		copiedData.Buckets[k] = &ScoreBucket{
			MinScore:        v.MinScore,
			MaxScore:        v.MaxScore,
			TotalTrades:     v.TotalTrades,
			WinningTrades:   v.WinningTrades,
			WinRate:         v.WinRate,
			AvgPnLPercent:   v.AvgPnLPercent,
			TotalPnLPercent: v.TotalPnLPercent,
			LastUpdated:     v.LastUpdated,
		}
	}

	return copiedData
}

// CalculateIndicatorHash generates a SHA256 hash (first 8 chars) from a list of indicators.
// Indicators are sorted alphabetically before hashing to ensure consistent results.
func CalculateIndicatorHash(indicators []string) string {
	if len(indicators) == 0 {
		return "00000000"
	}

	// Sort indicators for consistent hashing
	sorted := make([]string, len(indicators))
	copy(sorted, indicators)
	sort.Strings(sorted)

	// Join with strings.Join for efficiency (avoids multiple allocations)
	combined := strings.Join(sorted, ",")

	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:4]) // First 8 hex chars (4 bytes)
}

// CalibrationRepository defines the interface for database operations.
// This allows dependency injection and testing.
type CalibrationRepository interface {
	UpsertCalibrationData(ctx context.Context, data *CalibrationDataRecord) error
	GetCalibrationData(ctx context.Context, userID, strategy, indicatorHash string) (*CalibrationDataRecord, error)
	GetAllCalibrationDataForUser(ctx context.Context, userID string) ([]*CalibrationDataRecord, error)
	GetCalibrationDataByStrategy(ctx context.Context, userID, strategy string) ([]*CalibrationDataRecord, error)
	GetCalibrationHistory(ctx context.Context, userID, strategy string) ([]*CalibrationHistoryRecord, error)
	ArchiveCalibrationData(ctx context.Context, userID, strategy, indicatorHash, reason string) error
	DeleteCalibrationData(ctx context.Context, userID, strategy, indicatorHash string) error
}

// CalibrationDataRecord represents a calibration data record in the database.
type CalibrationDataRecord struct {
	ID            int64                  `json:"id"`
	UserID        string                 `json:"user_id"`
	Strategy      string                 `json:"strategy"`
	IndicatorHash string                 `json:"indicator_hash"`
	IndicatorList []string               `json:"indicator_list"`
	Bucket0_24    *CalibrationBucketData `json:"bucket_0_24"`
	Bucket25_49   *CalibrationBucketData `json:"bucket_25_49"`
	Bucket50_74   *CalibrationBucketData `json:"bucket_50_74"`
	Bucket75_100  *CalibrationBucketData `json:"bucket_75_100"`
	Metadata      *CalibrationMetadata   `json:"metadata"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// CalibrationBucketData represents the data for a single score bucket in database format.
type CalibrationBucketData struct {
	TotalTrades     int64   `json:"total_trades"`
	WinningTrades   int64   `json:"winning_trades"`
	WinRate         float64 `json:"win_rate"`
	AvgPnLPercent   float64 `json:"avg_pnl_percent"`
	TotalPnLPercent float64 `json:"total_pnl_percent"`
	LastUpdated     string  `json:"last_updated,omitempty"`
}

// CalibrationMetadata contains metadata about the calibration.
type CalibrationMetadata struct {
	LastUpdated            string `json:"last_updated"`
	FirstTradeDate         string `json:"first_trade_date,omitempty"`
	TotalCalibrationTrades int64  `json:"total_calibration_trades"`
}

// CalibrationHistoryRecord represents an archived calibration record.
type CalibrationHistoryRecord struct {
	ID            int64                  `json:"id"`
	OriginalID    int64                  `json:"original_id"`
	UserID        string                 `json:"user_id"`
	Strategy      string                 `json:"strategy"`
	IndicatorHash string                 `json:"indicator_hash"`
	IndicatorList []string               `json:"indicator_list"`
	Bucket0_24    *CalibrationBucketData `json:"bucket_0_24"`
	Bucket25_49   *CalibrationBucketData `json:"bucket_25_49"`
	Bucket50_74   *CalibrationBucketData `json:"bucket_50_74"`
	Bucket75_100  *CalibrationBucketData `json:"bucket_75_100"`
	Metadata      *CalibrationMetadata   `json:"metadata"`
	ArchivedAt    time.Time              `json:"archived_at"`
	ArchiveReason string                 `json:"archive_reason"`
}

// CalibrationService manages calibration data operations with Redis and PostgreSQL persistence.
// Story 11.25: Calibration Data Storage - Write-through pattern to DB + Redis.
type CalibrationService struct {
	client       *redis.Client
	dbRepo       CalibrationRepository
	mu           sync.RWMutex
	calibrations map[string]*CalibrationData // key: "userID:strategy:hash"
	ttl          time.Duration
}

// NewCalibrationService creates a new CalibrationService with the given Redis client.
// Returns nil if client is nil - caller must check.
// For full persistence (DB + Redis), use NewCalibrationServiceWithDB instead.
func NewCalibrationService(client *redis.Client) *CalibrationService {
	if client == nil {
		log.Printf("[CALIBRATION] Warning: NewCalibrationService called with nil client")
		return nil
	}
	return &CalibrationService{
		client:       client,
		dbRepo:       nil, // No DB persistence
		calibrations: make(map[string]*CalibrationData),
		ttl:          DefaultCalibrationTTL,
	}
}

// NewCalibrationServiceWithDB creates a CalibrationService with both Redis and PostgreSQL persistence.
// Story 11.25: Calibration Data Storage - Adds database persistence with write-through pattern.
func NewCalibrationServiceWithDB(client *redis.Client, dbRepo CalibrationRepository) *CalibrationService {
	if client == nil {
		log.Printf("[CALIBRATION] Warning: NewCalibrationServiceWithDB called with nil Redis client")
		return nil
	}
	return &CalibrationService{
		client:       client,
		dbRepo:       dbRepo,
		calibrations: make(map[string]*CalibrationData),
		ttl:          DefaultCalibrationTTL,
	}
}

// NewCalibrationServiceWithTTL creates a CalibrationService with custom TTL.
func NewCalibrationServiceWithTTL(client *redis.Client, ttl time.Duration) *CalibrationService {
	svc := NewCalibrationService(client)
	if svc != nil && ttl > 0 {
		svc.ttl = ttl
	}
	return svc
}

// SetDBRepository sets the database repository for persistence.
// This allows adding DB persistence to an existing service.
func (s *CalibrationService) SetDBRepository(dbRepo CalibrationRepository) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dbRepo = dbRepo
	log.Printf("[CALIBRATION] Database repository configured for persistence")
}

// calibrationKey generates the memory cache key.
func calibrationKey(userID int, strategy, indicatorHash string) string {
	return fmt.Sprintf("%d:%s:%s", userID, strategy, indicatorHash)
}

// redisKey generates the Redis key for calibration data.
func redisKey(userID int, strategy, indicatorHash string) string {
	return fmt.Sprintf(PrefixCalibration, userID, strategy, indicatorHash)
}

// RecordTradeOutcome records a trade outcome and updates memory cache, Redis, and PostgreSQL.
// Story 11.25: Write-through pattern - saves to both Redis and DB on every update.
func (s *CalibrationService) RecordTradeOutcome(ctx context.Context, userID int, strategy string, indicatorHash string, entryScore int, won bool, pnlPercent float64) error {
	if s == nil {
		return fmt.Errorf("calibration service is nil")
	}

	// Validate inputs
	if userID <= 0 {
		return fmt.Errorf("invalid userID: %d", userID)
	}
	if strategy == "" {
		return fmt.Errorf("strategy is required")
	}
	if indicatorHash == "" {
		indicatorHash = "00000000" // Default for no specific indicators
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get or create calibration data
	key := calibrationKey(userID, strategy, indicatorHash)
	data, exists := s.calibrations[key]
	if !exists {
		// Try to load from Redis first
		loadedData, err := s.loadFromRedisInternal(ctx, userID, strategy, indicatorHash)
		if err != nil {
			log.Printf("[CALIBRATION] Failed to load from Redis for %s: %v", key, err)
		}
		if loadedData != nil {
			data = loadedData
		} else if s.dbRepo != nil {
			// Story 11.25: Fall back to database if Redis miss
			dbRecord, err := s.dbRepo.GetCalibrationData(ctx, fmt.Sprintf("%d", userID), strategy, indicatorHash)
			if err != nil {
				log.Printf("[CALIBRATION] Failed to load from DB for %s: %v", key, err)
			}
			if dbRecord != nil {
				data = s.dbRecordToCalibrationData(dbRecord)
				log.Printf("[CALIBRATION] Loaded from DB for %s", key)
			}
		}
		if data == nil {
			data = NewCalibrationData(userID, strategy, indicatorHash)
		}
		s.calibrations[key] = data
	}

	// Record the outcome
	data.RecordOutcome(entryScore, won, pnlPercent)

	// Save to Redis
	if err := s.saveToRedisInternal(ctx, userID, strategy, indicatorHash, data); err != nil {
		log.Printf("[CALIBRATION] Failed to save to Redis for %s: %v", key, err)
		// Continue - try to save to DB even if Redis fails
	}

	// Story 11.25: Write-through to PostgreSQL
	if s.dbRepo != nil {
		dbRecord := s.calibrationDataToDBRecord(data)
		if err := s.dbRepo.UpsertCalibrationData(ctx, dbRecord); err != nil {
			log.Printf("[CALIBRATION] Failed to save to DB for %s: %v", key, err)
			// Don't return error - in-memory and Redis updates succeeded
		} else {
			log.Printf("[CALIBRATION] Saved to DB for user %d, strategy %s (total trades: %d)", userID, strategy, data.TotalTrades)
		}
	}

	return nil
}

// GetCalibratedConfidence returns the calibrated confidence (historical win rate) for a score.
// Falls back to rawScore/100 if no calibration data or insufficient trades.
func (s *CalibrationService) GetCalibratedConfidence(ctx context.Context, userID int, strategy string, indicatorHash string, score int) (float64, error) {
	if s == nil {
		return float64(score) / 100.0, fmt.Errorf("calibration service is nil")
	}

	// Validate inputs
	if userID <= 0 {
		return float64(score) / 100.0, fmt.Errorf("invalid userID: %d", userID)
	}
	if strategy == "" {
		return float64(score) / 100.0, fmt.Errorf("strategy is required")
	}
	if indicatorHash == "" {
		indicatorHash = "00000000"
	}

	// Clamp score
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	data, err := s.GetCalibrationData(ctx, userID, strategy, indicatorHash)
	if err != nil || data == nil {
		// Fallback to raw score as confidence
		return float64(score) / 100.0, nil
	}

	// Check if calibration is reliable
	if !data.IsReliable(DefaultMinTradesForReliable) {
		// Fallback to raw score as confidence
		return float64(score) / 100.0, nil
	}

	// Get calibrated confidence from bucket
	calibrated := data.GetCalibratedConfidence(score)
	if calibrated == 0 {
		// Bucket has no trades, fallback
		return float64(score) / 100.0, nil
	}

	return calibrated, nil
}

// GetCalibrationData retrieves calibration data from memory cache, Redis, or PostgreSQL.
// Story 11.25: Read pattern - Check memory, then Redis, then DB, populate caches on miss.
func (s *CalibrationService) GetCalibrationData(ctx context.Context, userID int, strategy string, indicatorHash string) (*CalibrationData, error) {
	if s == nil {
		return nil, fmt.Errorf("calibration service is nil")
	}

	if userID <= 0 {
		return nil, fmt.Errorf("invalid userID: %d", userID)
	}
	if strategy == "" {
		return nil, fmt.Errorf("strategy is required")
	}
	if indicatorHash == "" {
		indicatorHash = "00000000"
	}

	s.mu.RLock()
	key := calibrationKey(userID, strategy, indicatorHash)
	data, exists := s.calibrations[key]
	s.mu.RUnlock()

	if exists {
		// Return a copy to prevent external mutation of cached data
		return data.Copy(), nil
	}

	// Try to load from Redis
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if data, exists = s.calibrations[key]; exists {
		// Return a copy to prevent external mutation of cached data
		return data.Copy(), nil
	}

	loadedData, err := s.loadFromRedisInternal(ctx, userID, strategy, indicatorHash)
	if err != nil {
		log.Printf("[CALIBRATION] Redis load error for %s: %v", key, err)
	}

	if loadedData != nil {
		s.calibrations[key] = loadedData
		// Return a copy to prevent external mutation of cached data
		return loadedData.Copy(), nil
	}

	// Story 11.25: Fall back to database if Redis miss
	if s.dbRepo != nil {
		dbRecord, err := s.dbRepo.GetCalibrationData(ctx, fmt.Sprintf("%d", userID), strategy, indicatorHash)
		if err != nil {
			log.Printf("[CALIBRATION] DB load error for %s: %v", key, err)
			return nil, err
		}
		if dbRecord != nil {
			loadedData = s.dbRecordToCalibrationData(dbRecord)
			s.calibrations[key] = loadedData
			// Populate Redis cache
			if err := s.saveToRedisInternal(ctx, userID, strategy, indicatorHash, loadedData); err != nil {
				log.Printf("[CALIBRATION] Failed to populate Redis cache for %s: %v", key, err)
			}
			log.Printf("[CALIBRATION] Loaded from DB and populated cache for %s", key)
			return loadedData.Copy(), nil
		}
	}

	return nil, nil
}

// ResetCalibration clears calibration data for a user/strategy/indicator combination.
// Story 11.25: Archives to DB history before deleting.
func (s *CalibrationService) ResetCalibration(ctx context.Context, userID int, strategy string, indicatorHash string) error {
	if s == nil {
		return fmt.Errorf("calibration service is nil")
	}

	if userID <= 0 {
		return fmt.Errorf("invalid userID: %d", userID)
	}
	if strategy == "" {
		return fmt.Errorf("strategy is required")
	}
	if indicatorHash == "" {
		indicatorHash = "00000000"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove from memory cache
	key := calibrationKey(userID, strategy, indicatorHash)
	delete(s.calibrations, key)

	// Remove from Redis
	rKey := redisKey(userID, strategy, indicatorHash)
	if err := s.client.Del(ctx, rKey).Err(); err != nil {
		return fmt.Errorf("failed to delete calibration from Redis: %w", err)
	}

	// Story 11.25: Archive to database history before deleting
	if s.dbRepo != nil {
		userIDStr := fmt.Sprintf("%d", userID)
		if err := s.dbRepo.ArchiveCalibrationData(ctx, userIDStr, strategy, indicatorHash, "USER_RESET"); err != nil {
			log.Printf("[CALIBRATION] Failed to archive to DB for %s: %v (continuing)", key, err)
			// Continue even if archive fails - Redis deletion succeeded
		}
	}

	log.Printf("[CALIBRATION] Reset calibration for user %d, strategy %s, hash %s", userID, strategy, indicatorHash)
	return nil
}

// ArchiveAndReplace archives current calibration and creates a new one (for indicator changes).
// Story 11.25: Migration path when indicators change.
func (s *CalibrationService) ArchiveAndReplace(ctx context.Context, userID int, strategy, oldHash, newHash string, newIndicators []string) error {
	if s == nil {
		return fmt.Errorf("calibration service is nil")
	}

	if userID <= 0 {
		return fmt.Errorf("invalid userID: %d", userID)
	}
	if strategy == "" {
		return fmt.Errorf("strategy is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	userIDStr := fmt.Sprintf("%d", userID)

	// Archive old calibration in database
	if s.dbRepo != nil && oldHash != "" && oldHash != newHash {
		if err := s.dbRepo.ArchiveCalibrationData(ctx, userIDStr, strategy, oldHash, "INDICATOR_CHANGE"); err != nil {
			log.Printf("[CALIBRATION] Failed to archive old calibration for %s:%s: %v", strategy, oldHash, err)
		}
	}

	// Remove old from memory cache
	oldKey := calibrationKey(userID, strategy, oldHash)
	delete(s.calibrations, oldKey)

	// Remove old from Redis
	oldRedisKey := redisKey(userID, strategy, oldHash)
	if err := s.client.Del(ctx, oldRedisKey).Err(); err != nil {
		log.Printf("[CALIBRATION] Failed to delete old Redis key %s: %v", oldRedisKey, err)
	}

	// Create new calibration data
	newData := NewCalibrationData(userID, strategy, newHash)
	newKey := calibrationKey(userID, strategy, newHash)
	s.calibrations[newKey] = newData

	// Save to Redis
	if err := s.saveToRedisInternal(ctx, userID, strategy, newHash, newData); err != nil {
		log.Printf("[CALIBRATION] Failed to save new calibration to Redis: %v", err)
	}

	// Save to database
	if s.dbRepo != nil {
		dbRecord := s.calibrationDataToDBRecord(newData)
		dbRecord.IndicatorList = newIndicators
		if err := s.dbRepo.UpsertCalibrationData(ctx, dbRecord); err != nil {
			log.Printf("[CALIBRATION] Failed to save new calibration to DB: %v", err)
		}
	}

	log.Printf("[CALIBRATION] Archived old calibration and created new for user %d, strategy %s (old: %s, new: %s)",
		userID, strategy, oldHash, newHash)
	return nil
}

// IsCalibrationReliable checks if calibration data has enough trades.
func (s *CalibrationService) IsCalibrationReliable(data *CalibrationData, minTrades int) bool {
	if data == nil {
		return false
	}
	if minTrades <= 0 {
		minTrades = DefaultMinTradesForReliable
	}
	return data.IsReliable(minTrades)
}

// LoadFromRedis loads calibration data from Redis into memory cache.
func (s *CalibrationService) LoadFromRedis(ctx context.Context, userID int, strategy string, indicatorHash string) error {
	if s == nil {
		return fmt.Errorf("calibration service is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.loadFromRedisInternal(ctx, userID, strategy, indicatorHash)
	if err != nil {
		return err
	}

	if data != nil {
		key := calibrationKey(userID, strategy, indicatorHash)
		s.calibrations[key] = data
	}

	return nil
}

// loadFromRedisInternal loads data from Redis (caller must hold lock).
func (s *CalibrationService) loadFromRedisInternal(ctx context.Context, userID int, strategy string, indicatorHash string) (*CalibrationData, error) {
	rKey := redisKey(userID, strategy, indicatorHash)

	jsonData, err := s.client.Get(ctx, rKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Key doesn't exist
		}
		return nil, fmt.Errorf("failed to get calibration from Redis: %w", err)
	}

	var data CalibrationData
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal calibration data: %w", err)
	}

	return &data, nil
}

// SaveToRedis persists calibration data to Redis.
func (s *CalibrationService) SaveToRedis(ctx context.Context, userID int, strategy string, indicatorHash string) error {
	if s == nil {
		return fmt.Errorf("calibration service is nil")
	}

	// Use single Lock to avoid race condition between RUnlock and Lock
	s.mu.Lock()
	defer s.mu.Unlock()

	key := calibrationKey(userID, strategy, indicatorHash)
	data, exists := s.calibrations[key]

	if !exists || data == nil {
		return fmt.Errorf("no calibration data found for %s", key)
	}

	return s.saveToRedisInternal(ctx, userID, strategy, indicatorHash, data)
}

// saveToRedisInternal saves data to Redis (caller must hold lock).
func (s *CalibrationService) saveToRedisInternal(ctx context.Context, userID int, strategy string, indicatorHash string, data *CalibrationData) error {
	rKey := redisKey(userID, strategy, indicatorHash)

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal calibration data: %w", err)
	}

	if err := s.client.Set(ctx, rKey, jsonData, s.ttl).Err(); err != nil {
		return fmt.Errorf("failed to save calibration to Redis: %w", err)
	}

	return nil
}

// GetAllUserCalibrations returns all calibration data for a user from memory cache.
// Returns deep copies to prevent external mutation of cached data.
func (s *CalibrationService) GetAllUserCalibrations(userID int) []*CalibrationData {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*CalibrationData
	prefix := fmt.Sprintf("%d:", userID)

	for key, data := range s.calibrations {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			// Return copies to prevent external mutation of cached data
			results = append(results, data.Copy())
		}
	}

	return results
}

// ClearMemoryCache clears all in-memory calibration data.
// Does not affect Redis storage.
func (s *CalibrationService) ClearMemoryCache() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.calibrations = make(map[string]*CalibrationData)
	log.Printf("[CALIBRATION] Memory cache cleared")
}

// Stats returns statistics about the calibration service.
type CalibrationStats struct {
	CachedCalibrations int   `json:"cached_calibrations"`
	TotalTradesTracked int64 `json:"total_trades_tracked"`
}

// GetStats returns current service statistics.
func (s *CalibrationService) GetStats() CalibrationStats {
	if s == nil {
		return CalibrationStats{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := CalibrationStats{
		CachedCalibrations: len(s.calibrations),
	}

	for _, data := range s.calibrations {
		stats.TotalTradesTracked += data.TotalTrades
	}

	return stats
}

// CalibrationResponse is the API response format for calibration data.
type CalibrationResponse struct {
	UserID         int                        `json:"user_id"`
	Strategy       string                     `json:"strategy"`
	IndicatorHash  string                     `json:"indicator_hash"`
	TotalTrades    int64                      `json:"total_trades"`
	FirstTradeDate *time.Time                 `json:"first_trade_date,omitempty"`
	LastUpdated    time.Time                  `json:"last_updated"`
	IsReliable     bool                       `json:"is_reliable"`
	Buckets        map[string]BucketResponse  `json:"buckets"`
}

// BucketResponse is the API response format for a bucket.
type BucketResponse struct {
	MinScore      int     `json:"min_score"`
	MaxScore      int     `json:"max_score"`
	TotalTrades   int64   `json:"total_trades"`
	WinningTrades int64   `json:"winning_trades"`
	WinRate       float64 `json:"win_rate"`
	AvgPnLPercent float64 `json:"avg_pnl_percent"`
}

// ToResponse converts CalibrationData to API response format.
func (cd *CalibrationData) ToResponse(minTrades int) CalibrationResponse {
	resp := CalibrationResponse{
		UserID:        cd.UserID,
		Strategy:      cd.Strategy,
		IndicatorHash: cd.IndicatorHash,
		TotalTrades:   cd.TotalTrades,
		LastUpdated:   cd.LastUpdated,
		IsReliable:    cd.IsReliable(minTrades),
		Buckets:       make(map[string]BucketResponse),
	}

	if !cd.FirstTradeDate.IsZero() {
		resp.FirstTradeDate = &cd.FirstTradeDate
	}

	for name, bucket := range cd.Buckets {
		resp.Buckets[name] = BucketResponse{
			MinScore:      bucket.MinScore,
			MaxScore:      bucket.MaxScore,
			TotalTrades:   bucket.TotalTrades,
			WinningTrades: bucket.WinningTrades,
			WinRate:       bucket.WinRate,
			AvgPnLPercent: bucket.AvgPnLPercent,
		}
	}

	return resp
}

// ============================================================================
// Story 11.25: Database Conversion Functions
// ============================================================================

// calibrationDataToDBRecord converts CalibrationData to database record format.
func (s *CalibrationService) calibrationDataToDBRecord(data *CalibrationData) *CalibrationDataRecord {
	if data == nil {
		return nil
	}

	record := &CalibrationDataRecord{
		UserID:        fmt.Sprintf("%d", data.UserID),
		Strategy:      data.Strategy,
		IndicatorHash: data.IndicatorHash,
		IndicatorList: []string{}, // Will be populated if available
		Bucket0_24:    &CalibrationBucketData{},
		Bucket25_49:   &CalibrationBucketData{},
		Bucket50_74:   &CalibrationBucketData{},
		Bucket75_100:  &CalibrationBucketData{},
		Metadata: &CalibrationMetadata{
			LastUpdated:            data.LastUpdated.Format(time.RFC3339),
			TotalCalibrationTrades: data.TotalTrades,
		},
	}

	if !data.FirstTradeDate.IsZero() {
		record.Metadata.FirstTradeDate = data.FirstTradeDate.Format(time.RFC3339)
	}

	// Convert buckets
	if bucket, ok := data.Buckets[BucketLow]; ok {
		record.Bucket0_24 = s.scoreBucketToBucketData(bucket)
	}
	if bucket, ok := data.Buckets[BucketMediumLow]; ok {
		record.Bucket25_49 = s.scoreBucketToBucketData(bucket)
	}
	if bucket, ok := data.Buckets[BucketMediumHigh]; ok {
		record.Bucket50_74 = s.scoreBucketToBucketData(bucket)
	}
	if bucket, ok := data.Buckets[BucketHigh]; ok {
		record.Bucket75_100 = s.scoreBucketToBucketData(bucket)
	}

	return record
}

// scoreBucketToBucketData converts a ScoreBucket to CalibrationBucketData.
func (s *CalibrationService) scoreBucketToBucketData(bucket *ScoreBucket) *CalibrationBucketData {
	if bucket == nil {
		return &CalibrationBucketData{}
	}
	data := &CalibrationBucketData{
		TotalTrades:     bucket.TotalTrades,
		WinningTrades:   bucket.WinningTrades,
		WinRate:         bucket.WinRate,
		AvgPnLPercent:   bucket.AvgPnLPercent,
		TotalPnLPercent: bucket.TotalPnLPercent,
	}
	if !bucket.LastUpdated.IsZero() {
		data.LastUpdated = bucket.LastUpdated.Format(time.RFC3339)
	}
	return data
}

// dbRecordToCalibrationData converts a database record to CalibrationData.
func (s *CalibrationService) dbRecordToCalibrationData(record *CalibrationDataRecord) *CalibrationData {
	if record == nil {
		return nil
	}

	// Parse userID (stored as string in DB for UUID support)
	var userID int
	fmt.Sscanf(record.UserID, "%d", &userID)

	data := &CalibrationData{
		UserID:        userID,
		Strategy:      record.Strategy,
		IndicatorHash: record.IndicatorHash,
		Buckets:       make(map[string]*ScoreBucket),
		TotalTrades:   0,
		LastUpdated:   record.UpdatedAt,
	}

	// Parse metadata
	if record.Metadata != nil {
		data.TotalTrades = record.Metadata.TotalCalibrationTrades
		if record.Metadata.FirstTradeDate != "" {
			if t, err := time.Parse(time.RFC3339, record.Metadata.FirstTradeDate); err == nil {
				data.FirstTradeDate = t
			}
		}
		if record.Metadata.LastUpdated != "" {
			if t, err := time.Parse(time.RFC3339, record.Metadata.LastUpdated); err == nil {
				data.LastUpdated = t
			}
		}
	}

	// Convert buckets
	data.Buckets[BucketLow] = s.bucketDataToScoreBucket(record.Bucket0_24, 0, 24)
	data.Buckets[BucketMediumLow] = s.bucketDataToScoreBucket(record.Bucket25_49, 25, 49)
	data.Buckets[BucketMediumHigh] = s.bucketDataToScoreBucket(record.Bucket50_74, 50, 74)
	data.Buckets[BucketHigh] = s.bucketDataToScoreBucket(record.Bucket75_100, 75, 100)

	return data
}

// bucketDataToScoreBucket converts CalibrationBucketData to a ScoreBucket.
func (s *CalibrationService) bucketDataToScoreBucket(data *CalibrationBucketData, minScore, maxScore int) *ScoreBucket {
	bucket := &ScoreBucket{
		MinScore: minScore,
		MaxScore: maxScore,
	}
	if data != nil {
		bucket.TotalTrades = data.TotalTrades
		bucket.WinningTrades = data.WinningTrades
		bucket.WinRate = data.WinRate
		bucket.AvgPnLPercent = data.AvgPnLPercent
		bucket.TotalPnLPercent = data.TotalPnLPercent
		if data.LastUpdated != "" {
			if t, err := time.Parse(time.RFC3339, data.LastUpdated); err == nil {
				bucket.LastUpdated = t
			}
		}
	}
	return bucket
}

// ============================================================================
// Story 11.25: API Response Types for Calibration Accuracy UI
// ============================================================================

// CalibrationAccuracyResponse provides detailed accuracy information for UI display.
type CalibrationAccuracyResponse struct {
	UserID          int                       `json:"user_id"`
	Strategy        string                    `json:"strategy"`
	IndicatorHash   string                    `json:"indicator_hash"`
	Indicators      []string                  `json:"indicators"`
	IsReliable      bool                      `json:"is_reliable"`
	MinTradesNeeded int                       `json:"min_trades_needed"`
	TotalTrades     int64                     `json:"total_trades"`
	Buckets         []BucketAccuracyResponse  `json:"buckets"`
	LastUpdated     time.Time                 `json:"last_updated"`
	FirstTradeDate  *time.Time                `json:"first_trade_date,omitempty"`
}

// BucketAccuracyResponse provides accuracy metrics for a single score bucket.
type BucketAccuracyResponse struct {
	Range         string  `json:"range"`            // "0-24", "25-49", etc.
	TotalTrades   int64   `json:"total_trades"`
	WinningTrades int64   `json:"winning_trades"`
	WinRate       float64 `json:"win_rate"`
	AvgPnLPercent float64 `json:"avg_pnl_percent"`
	ExpectedRate  float64 `json:"expected_rate"`    // Expected win rate based on score range midpoint
	Accuracy      string  `json:"accuracy"`         // "Good", "Needs Calibration", "Insufficient Data"
}

// ToAccuracyResponse converts CalibrationData to accuracy response for UI.
func (cd *CalibrationData) ToAccuracyResponse(indicators []string, minTrades int) CalibrationAccuracyResponse {
	if minTrades <= 0 {
		minTrades = DefaultMinTradesForReliable
	}

	resp := CalibrationAccuracyResponse{
		UserID:          cd.UserID,
		Strategy:        cd.Strategy,
		IndicatorHash:   cd.IndicatorHash,
		Indicators:      indicators,
		IsReliable:      cd.IsReliable(minTrades),
		MinTradesNeeded: minTrades,
		TotalTrades:     cd.TotalTrades,
		LastUpdated:     cd.LastUpdated,
		Buckets:         make([]BucketAccuracyResponse, 0, 4),
	}

	if !cd.FirstTradeDate.IsZero() {
		resp.FirstTradeDate = &cd.FirstTradeDate
	}

	// Add buckets in order
	bucketOrder := []string{BucketLow, BucketMediumLow, BucketMediumHigh, BucketHigh}
	expectedRates := map[string]float64{
		BucketLow:        0.12, // 0-24 range midpoint ~12%
		BucketMediumLow:  0.37, // 25-49 range midpoint ~37%
		BucketMediumHigh: 0.62, // 50-74 range midpoint ~62%
		BucketHigh:       0.87, // 75-100 range midpoint ~87%
	}

	for _, bucketName := range bucketOrder {
		bucket, ok := cd.Buckets[bucketName]
		if !ok {
			continue
		}

		bucketResp := BucketAccuracyResponse{
			Range:         bucketName,
			TotalTrades:   bucket.TotalTrades,
			WinningTrades: bucket.WinningTrades,
			WinRate:       bucket.WinRate,
			AvgPnLPercent: bucket.AvgPnLPercent,
			ExpectedRate:  expectedRates[bucketName],
		}

		// Calculate accuracy status
		if bucket.TotalTrades < 10 {
			bucketResp.Accuracy = "Insufficient Data"
		} else {
			// Compare actual win rate to expected
			diff := bucket.WinRate - expectedRates[bucketName]
			if diff >= -0.1 && diff <= 0.1 {
				bucketResp.Accuracy = "Good"
			} else if diff < -0.1 {
				bucketResp.Accuracy = "Underperforming"
			} else {
				bucketResp.Accuracy = "Outperforming"
			}
		}

		resp.Buckets = append(resp.Buckets, bucketResp)
	}

	return resp
}

// SyncFromDatabase loads all calibration data from database for a user and populates caches.
// Story 11.25: Sync Redis and DB on startup.
func (s *CalibrationService) SyncFromDatabase(ctx context.Context, userID int) error {
	if s == nil || s.dbRepo == nil {
		return nil
	}

	userIDStr := fmt.Sprintf("%d", userID)
	records, err := s.dbRepo.GetAllCalibrationDataForUser(ctx, userIDStr)
	if err != nil {
		return fmt.Errorf("failed to load calibrations from DB: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, record := range records {
		data := s.dbRecordToCalibrationData(record)
		if data == nil {
			continue
		}

		key := calibrationKey(data.UserID, data.Strategy, data.IndicatorHash)
		s.calibrations[key] = data

		// Populate Redis cache
		if err := s.saveToRedisInternal(ctx, data.UserID, data.Strategy, data.IndicatorHash, data); err != nil {
			log.Printf("[CALIBRATION] Failed to sync to Redis for %s: %v", key, err)
		}
	}

	log.Printf("[CALIBRATION] Synced %d calibrations from DB for user %d", len(records), userID)
	return nil
}

// HasDBRepository returns true if database persistence is configured.
func (s *CalibrationService) HasDBRepository() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dbRepo != nil
}
