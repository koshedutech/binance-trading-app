// Package settlement provides the settlement scheduler for Epic 8.
// This scheduler runs position snapshots at each user's timezone midnight.
package settlement

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"
)

// SchedulerConfig holds configuration for the settlement scheduler
type SchedulerConfig struct {
	// CheckInterval is how often to check for users needing settlement
	CheckInterval time.Duration

	// MaxConcurrent is the maximum number of concurrent user settlements
	MaxConcurrent int

	// SettlementTimeout is the maximum time allowed for a single user's settlement
	SettlementTimeout time.Duration
}

// DefaultSchedulerConfig returns default scheduler configuration
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		CheckInterval:     1 * time.Minute,  // Check every minute
		MaxConcurrent:     5,                // Process 5 users concurrently
		SettlementTimeout: 5 * time.Minute,  // 5 minute timeout per user (NFR-1)
	}
}

// Scheduler handles scheduled settlement operations
type Scheduler struct {
	snapshotService *PositionSnapshotService
	repo            *database.Repository
	clientFactory   *binance.ClientFactory
	config          *SchedulerConfig

	mu       sync.Mutex
	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewScheduler creates a new settlement scheduler
func NewScheduler(
	repo *database.Repository,
	clientFactory *binance.ClientFactory,
	config *SchedulerConfig,
) *Scheduler {
	if config == nil {
		config = DefaultSchedulerConfig()
	}

	snapshotService := NewPositionSnapshotService(repo, clientFactory)

	return &Scheduler{
		snapshotService: snapshotService,
		repo:            repo,
		clientFactory:   clientFactory,
		config:          config,
		stopChan:        make(chan struct{}),
	}
}

// Start starts the settlement scheduler
func (s *Scheduler) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("settlement scheduler already running")
	}
	s.running = true
	s.stopChan = make(chan struct{}) // Reinitialize for restart capability
	s.mu.Unlock()

	log.Println("[SETTLEMENT-SCHEDULER] Starting settlement scheduler")

	s.wg.Add(1)
	go s.runSettlementLoop()

	return nil
}

// Stop stops the settlement scheduler
func (s *Scheduler) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return fmt.Errorf("settlement scheduler not running")
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopChan)
	s.wg.Wait()

	log.Println("[SETTLEMENT-SCHEDULER] Settlement scheduler stopped")
	return nil
}

// IsRunning returns whether the scheduler is running
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// runSettlementLoop is the main scheduler loop
func (s *Scheduler) runSettlementLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	// Run immediately on start
	s.checkAndRunSettlements()

	for {
		select {
		case <-ticker.C:
			s.checkAndRunSettlements()
		case <-s.stopChan:
			log.Println("[SETTLEMENT-SCHEDULER] Received stop signal")
			return
		}
	}
}

// checkAndRunSettlements checks for users needing settlement and runs them
func (s *Scheduler) checkAndRunSettlements() {
	ctx := context.Background()

	// Get all users with their timezone and last settlement date
	users, err := s.repo.GetUsersForSettlementCheck(ctx)
	if err != nil {
		log.Printf("[SETTLEMENT-SCHEDULER] Error getting users: %v", err)
		return
	}

	// Filter users who need settlement
	var usersNeedingSettlement []database.User
	for _, user := range users {
		if s.userNeedsSettlement(user) {
			usersNeedingSettlement = append(usersNeedingSettlement, user)
		}
	}

	if len(usersNeedingSettlement) == 0 {
		return
	}

	log.Printf("[SETTLEMENT-SCHEDULER] Found %d users needing settlement", len(usersNeedingSettlement))

	// Process users with limited concurrency
	semaphore := make(chan struct{}, s.config.MaxConcurrent)
	var wg sync.WaitGroup

	for _, user := range usersNeedingSettlement {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore

		go func(u database.User) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[SETTLEMENT-SCHEDULER] Panic recovered for user %s: %v", u.ID, r)
				}
			}()

			s.processUserSettlement(ctx, u)
		}(user)
	}

	wg.Wait()
}

// userNeedsSettlement determines if a user needs settlement based on their timezone
func (s *Scheduler) userNeedsSettlement(user database.User) bool {
	// Load user's timezone
	loc, err := time.LoadLocation(user.Timezone)
	if err != nil {
		log.Printf("[SETTLEMENT-SCHEDULER] Invalid timezone '%s' for user %s, using UTC", user.Timezone, user.ID)
		loc = time.UTC
	}

	// Get current time in user's timezone
	nowInUserTZ := time.Now().In(loc)
	todayInUserTZ := time.Date(nowInUserTZ.Year(), nowInUserTZ.Month(), nowInUserTZ.Day(), 0, 0, 0, 0, loc)

	// If no last settlement date, user needs settlement
	if user.LastSettlementDate == nil {
		// Only run if we're past midnight
		return nowInUserTZ.After(todayInUserTZ)
	}

	// User needs settlement if last settlement date is before today
	lastSettlementDate := time.Date(
		user.LastSettlementDate.Year(),
		user.LastSettlementDate.Month(),
		user.LastSettlementDate.Day(),
		0, 0, 0, 0, loc,
	)

	// Need settlement if last settlement date is before today
	return lastSettlementDate.Before(todayInUserTZ)
}

// processUserSettlement runs settlement for a single user
func (s *Scheduler) processUserSettlement(ctx context.Context, user database.User) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, s.config.SettlementTimeout)
	defer cancel()

	log.Printf("[SETTLEMENT-SCHEDULER] Starting settlement for user %s (timezone: %s)", user.ID, user.Timezone)

	// Load user's timezone
	loc, err := time.LoadLocation(user.Timezone)
	if err != nil {
		log.Printf("[SETTLEMENT-SCHEDULER] Invalid timezone for user %s: %v", user.ID, err)
		loc = time.UTC
	}

	// Calculate yesterday's date in user's timezone (we snapshot yesterday's positions)
	nowInUserTZ := time.Now().In(loc)
	yesterdayInUserTZ := time.Date(nowInUserTZ.Year(), nowInUserTZ.Month(), nowInUserTZ.Day()-1, 0, 0, 0, 0, loc)

	// Take position snapshot
	result, err := s.snapshotService.SnapshotOpenPositions(ctx, user.ID, yesterdayInUserTZ)
	if err != nil {
		log.Printf("[SETTLEMENT-SCHEDULER] Snapshot failed for user %s: %v", user.ID, err)
		// Don't update last settlement date if snapshot failed
		return
	}

	if !result.Success {
		log.Printf("[SETTLEMENT-SCHEDULER] Snapshot unsuccessful for user %s: %s", user.ID, result.Error)
		return
	}

	// Update user's last settlement date
	todayInUserTZ := time.Date(nowInUserTZ.Year(), nowInUserTZ.Month(), nowInUserTZ.Day(), 0, 0, 0, 0, loc)
	if err := s.repo.UpdateLastSettlementDate(ctx, user.ID, todayInUserTZ); err != nil {
		log.Printf("[SETTLEMENT-SCHEDULER] Failed to update last settlement date for user %s: %v", user.ID, err)
		return
	}

	log.Printf("[SETTLEMENT-SCHEDULER] Settlement completed for user %s: %d positions, duration: %v",
		user.ID, result.PositionCount, result.Duration)
}

// RunManualSettlement runs settlement for a specific user immediately
// This is useful for testing or manual intervention
func (s *Scheduler) RunManualSettlement(ctx context.Context, userID string, snapshotDate time.Time) (*SnapshotResult, error) {
	log.Printf("[SETTLEMENT-SCHEDULER] Running manual settlement for user %s on %s", userID, snapshotDate.Format("2006-01-02"))

	result, err := s.snapshotService.SnapshotOpenPositions(ctx, userID, snapshotDate)
	if err != nil {
		return nil, fmt.Errorf("snapshot failed: %w", err)
	}

	return result, nil
}

// GetSettlementStatus returns the settlement status for all users
func (s *Scheduler) GetSettlementStatus(ctx context.Context) ([]SettlementStatus, error) {
	users, err := s.repo.GetUsersForSettlementCheck(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	var statuses []SettlementStatus
	for _, user := range users {
		loc, err := time.LoadLocation(user.Timezone)
		if err != nil {
			loc = time.UTC
		}

		nowInUserTZ := time.Now().In(loc)
		todayInUserTZ := time.Date(nowInUserTZ.Year(), nowInUserTZ.Month(), nowInUserTZ.Day(), 0, 0, 0, 0, loc)
		tomorrowMidnight := todayInUserTZ.AddDate(0, 0, 1)

		status := SettlementStatus{
			UserID:             user.ID,
			Timezone:           user.Timezone,
			LastSettlementDate: user.LastSettlementDate,
			NextSettlementTime: tomorrowMidnight,
			NeedsSettlement:    s.userNeedsSettlement(user),
		}

		statuses = append(statuses, status)
	}

	return statuses, nil
}

// GetSnapshotService returns the underlying snapshot service
func (s *Scheduler) GetSnapshotService() *PositionSnapshotService {
	return s.snapshotService
}
