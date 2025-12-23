package billing

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"binance-trading-bot/internal/database"
)

// Scheduler handles scheduled billing operations
type Scheduler struct {
	profitCalc    *ProfitCalculator
	stripeService *StripeService
	repo          *database.Repository
	config        *SchedulerConfig

	mu        sync.Mutex
	running   bool
	stopChan  chan struct{}
	wg        sync.WaitGroup
	lastRun   time.Time
	nextRun   time.Time
}

// SchedulerConfig holds scheduler configuration
type SchedulerConfig struct {
	// Settlement timing
	SettlementDayOfWeek time.Weekday // 0 = Sunday
	SettlementHourUTC   int          // Hour in UTC

	// Balance snapshot settings
	SnapshotIntervalHours int

	// Retry settings
	MaxRetries     int
	RetryDelayMins int

	// Minimum payout threshold
	MinimumPayout float64
}

// DefaultSchedulerConfig returns default scheduler configuration
func DefaultSchedulerConfig() *SchedulerConfig {
	return &SchedulerConfig{
		SettlementDayOfWeek:   time.Sunday,
		SettlementHourUTC:     0,
		SnapshotIntervalHours: 4,
		MaxRetries:            3,
		RetryDelayMins:        30,
		MinimumPayout:         10.0,
	}
}

// NewScheduler creates a new billing scheduler
func NewScheduler(
	profitCalc *ProfitCalculator,
	stripeService *StripeService,
	repo *database.Repository,
	config *SchedulerConfig,
) *Scheduler {
	if config == nil {
		config = DefaultSchedulerConfig()
	}

	return &Scheduler{
		profitCalc:    profitCalc,
		stripeService: stripeService,
		repo:          repo,
		config:        config,
		stopChan:      make(chan struct{}),
	}
}

// Start starts the scheduler
func (s *Scheduler) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler already running")
	}
	s.running = true
	s.mu.Unlock()

	log.Println("Starting billing scheduler")

	// Start settlement check goroutine
	s.wg.Add(1)
	go s.runSettlementLoop()

	// Start balance snapshot goroutine
	s.wg.Add(1)
	go s.runSnapshotLoop()

	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	s.mu.Unlock()

	log.Println("Stopping billing scheduler")
	close(s.stopChan)
	s.wg.Wait()

	return nil
}

// IsRunning returns whether the scheduler is running
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// GetStatus returns the scheduler status
func (s *Scheduler) GetStatus() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	return map[string]interface{}{
		"running":          s.running,
		"last_run":         s.lastRun,
		"next_run":         s.nextRun,
		"settlement_day":   s.config.SettlementDayOfWeek.String(),
		"settlement_hour":  s.config.SettlementHourUTC,
		"minimum_payout":   s.config.MinimumPayout,
	}
}

// runSettlementLoop runs the settlement check loop
func (s *Scheduler) runSettlementLoop() {
	defer s.wg.Done()

	// Calculate next settlement time
	s.updateNextSettlementTime()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			now := time.Now().UTC()
			if now.After(s.nextRun) {
				log.Println("Running weekly settlement")
				if err := s.RunWeeklySettlement(context.Background()); err != nil {
					log.Printf("Error running settlement: %v", err)
				}
				s.updateNextSettlementTime()
			}
		}
	}
}

// updateNextSettlementTime calculates the next settlement time
func (s *Scheduler) updateNextSettlementTime() {
	now := time.Now().UTC()

	// Find next occurrence of settlement day/hour
	next := time.Date(now.Year(), now.Month(), now.Day(), s.config.SettlementHourUTC, 0, 0, 0, time.UTC)

	// Adjust to next settlement day
	daysUntilSettlement := (int(s.config.SettlementDayOfWeek) - int(now.Weekday()) + 7) % 7
	if daysUntilSettlement == 0 && now.After(next) {
		daysUntilSettlement = 7
	}
	next = next.AddDate(0, 0, daysUntilSettlement)

	s.mu.Lock()
	s.nextRun = next
	s.mu.Unlock()

	log.Printf("Next settlement scheduled for: %v", next)
}

// runSnapshotLoop runs the balance snapshot loop
func (s *Scheduler) runSnapshotLoop() {
	defer s.wg.Done()

	interval := time.Duration(s.config.SnapshotIntervalHours) * time.Hour
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Take initial snapshot
	s.takeAllBalanceSnapshots(context.Background())

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.takeAllBalanceSnapshots(context.Background())
		}
	}
}

// RunWeeklySettlement runs the weekly profit settlement for all users
func (s *Scheduler) RunWeeklySettlement(ctx context.Context) error {
	log.Println("Starting weekly settlement process")

	s.mu.Lock()
	s.lastRun = time.Now().UTC()
	s.mu.Unlock()

	// Get all active users
	users, err := s.repo.GetActiveUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active users: %w", err)
	}

	log.Printf("Processing settlement for %d active users", len(users))

	var successCount, failCount, skippedCount int
	var totalProfitShare float64

	for _, user := range users {
		report, err := s.processUserSettlement(ctx, user)
		if err != nil {
			log.Printf("Error processing settlement for user %s: %v", user.ID, err)
			failCount++
			continue
		}

		if report == nil {
			skippedCount++
			continue
		}

		successCount++
		totalProfitShare += report.ProfitShareDue
	}

	log.Printf("Settlement complete: %d success, %d failed, %d skipped, total profit share: $%.2f",
		successCount, failCount, skippedCount, totalProfitShare)

	return nil
}

// processUserSettlement processes settlement for a single user
func (s *Scheduler) processUserSettlement(ctx context.Context, user *database.User) (*ProfitReport, error) {
	// Calculate last week's profit
	report, err := s.profitCalc.CalculateLastWeekProfit(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate profit: %w", err)
	}

	// Check if there's any profit share due
	if report.ProfitShareDue < s.config.MinimumPayout {
		log.Printf("User %s: profit share $%.2f below minimum $%.2f, skipping invoice",
			user.ID, report.ProfitShareDue, s.config.MinimumPayout)

		// Still save the period for record keeping
		if _, err := s.profitCalc.SaveProfitPeriod(ctx, report); err != nil {
			log.Printf("Warning: failed to save profit period for user %s: %v", user.ID, err)
		}
		return nil, nil
	}

	// Save the profit period
	period, err := s.profitCalc.SaveProfitPeriod(ctx, report)
	if err != nil {
		return nil, fmt.Errorf("failed to save profit period: %w", err)
	}

	// Create Stripe invoice if configured
	if s.stripeService != nil && s.stripeService.IsConfigured() {
		if err := s.createProfitShareInvoice(ctx, user, period, report); err != nil {
			log.Printf("Warning: failed to create invoice for user %s: %v", user.ID, err)
			// Don't fail the whole process if invoice creation fails
		}
	}

	log.Printf("User %s settlement: gross profit $%.2f, net profit $%.2f, profit share due $%.2f",
		user.ID, report.GrossProfit, report.NetProfit, report.ProfitShareDue)

	return report, nil
}

// createProfitShareInvoice creates a Stripe invoice for profit share
func (s *Scheduler) createProfitShareInvoice(ctx context.Context, user *database.User, period *database.ProfitPeriod, report *ProfitReport) error {
	// Get or create Stripe customer
	customerID, err := s.stripeService.GetOrCreateCustomer(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to get/create customer: %w", err)
	}

	// Create description
	description := fmt.Sprintf("Trading Profit Share - %s to %s (%.1f%% of $%.2f profit)",
		report.PeriodStart.Format("Jan 2"),
		report.PeriodEnd.Format("Jan 2, 2006"),
		report.ProfitShareRate*100,
		report.NetProfit,
	)

	// Create invoice
	invoice, err := s.stripeService.CreateProfitShareInvoice(
		ctx,
		customerID,
		report.ProfitShareDue,
		period.ID,
		description,
	)
	if err != nil {
		return fmt.Errorf("failed to create invoice: %w", err)
	}

	// Update period with invoice ID
	if err := s.repo.UpdateProfitPeriodStatus(ctx, period.ID, string(StatusInvoiced), &invoice.ID); err != nil {
		log.Printf("Warning: failed to update period status: %v", err)
	}

	return nil
}

// takeAllBalanceSnapshots takes balance snapshots for all active users
func (s *Scheduler) takeAllBalanceSnapshots(ctx context.Context) {
	users, err := s.repo.GetActiveUsers(ctx)
	if err != nil {
		log.Printf("Error getting users for snapshots: %v", err)
		return
	}

	log.Printf("Taking balance snapshots for %d users", len(users))

	for _, user := range users {
		if err := s.takeUserBalanceSnapshot(ctx, user.ID); err != nil {
			log.Printf("Error taking snapshot for user %s: %v", user.ID, err)
		}
	}
}

// takeUserBalanceSnapshot takes a balance snapshot for a single user
func (s *Scheduler) takeUserBalanceSnapshot(ctx context.Context, userID string) error {
	// Get current balance from Binance
	balance, unrealizedPnL, err := s.profitCalc.GetCurrentBalance(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}

	// Save snapshot
	return s.profitCalc.CreateBalanceSnapshot(ctx, userID, "periodic", balance, unrealizedPnL)
}

// ManualSettlement runs settlement for a specific user manually
func (s *Scheduler) ManualSettlement(ctx context.Context, userID string) (*ProfitReport, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	return s.processUserSettlement(ctx, user)
}

// GetPendingSettlements returns all pending settlements
func (s *Scheduler) GetPendingSettlements(ctx context.Context) ([]database.ProfitPeriod, error) {
	return s.repo.GetPendingProfitPeriods(ctx)
}

// RetryFailedInvoices retries failed invoice creation
func (s *Scheduler) RetryFailedInvoices(ctx context.Context) error {
	periods, err := s.repo.GetPendingProfitPeriods(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending periods: %w", err)
	}

	var retried, failed int
	for _, period := range periods {
		if period.ProfitShareDue >= s.config.MinimumPayout {
			user, err := s.repo.GetUserByID(ctx, period.UserID)
			if err != nil || user == nil {
				failed++
				continue
			}

			if err := s.createProfitShareInvoice(ctx, user, &period, nil); err != nil {
				log.Printf("Failed to retry invoice for period %s: %v", period.ID, err)
				failed++
			} else {
				retried++
			}
		}
	}

	log.Printf("Invoice retry: %d success, %d failed", retried, failed)
	return nil
}

// GetUserProfitHistory returns profit history for a user
func (s *Scheduler) GetUserProfitHistory(ctx context.Context, userID string, limit int) ([]database.ProfitPeriod, error) {
	return s.repo.GetUserProfitPeriods(ctx, userID, limit)
}

// GetPlatformStats returns platform-wide billing statistics
func (s *Scheduler) GetPlatformStats(ctx context.Context) (map[string]interface{}, error) {
	return s.repo.GetPlatformProfitStats(ctx)
}
