// Package settlement provides settlement monitoring and alerting for Epic 8 Story 8.9.
// Monitors for stalled/failed settlements and sends admin alerts.
package settlement

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/email"
)

// MonitoringConfig holds configuration for settlement monitoring
type MonitoringConfig struct {
	CheckInterval    time.Duration // How often to check for stalled settlements
	AlertThreshold   time.Duration // How long before alerting (e.g., 1 hour)
	AdminEmail       string        // Email address to send alerts to
	Enabled          bool
}

// DefaultMonitoringConfig returns default monitoring configuration
func DefaultMonitoringConfig() *MonitoringConfig {
	return &MonitoringConfig{
		CheckInterval:  15 * time.Minute,
		AlertThreshold: 1 * time.Hour,
		Enabled:        true,
	}
}

// SettlementMonitor monitors for settlement failures and sends alerts
type SettlementMonitor struct {
	repo         *database.Repository
	emailService *email.Service
	config       *MonitoringConfig

	mu       sync.Mutex
	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewSettlementMonitor creates a new settlement monitor
func NewSettlementMonitor(repo *database.Repository, emailService *email.Service, config *MonitoringConfig) *SettlementMonitor {
	if config == nil {
		config = DefaultMonitoringConfig()
	}

	return &SettlementMonitor{
		repo:         repo,
		emailService: emailService,
		config:       config,
		stopChan:     make(chan struct{}),
	}
}

// Start starts the monitoring loop
func (m *SettlementMonitor) Start() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("settlement monitor already running")
	}
	m.running = true
	m.stopChan = make(chan struct{})
	m.mu.Unlock()

	log.Println("[SETTLEMENT-MONITOR] Starting settlement monitor")

	m.wg.Add(1)
	go m.runMonitoringLoop()

	return nil
}

// Stop stops the monitoring loop
func (m *SettlementMonitor) Stop() error {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return fmt.Errorf("settlement monitor not running")
	}
	m.running = false
	m.mu.Unlock()

	close(m.stopChan)
	m.wg.Wait()

	log.Println("[SETTLEMENT-MONITOR] Settlement monitor stopped")
	return nil
}

// IsRunning returns whether the monitor is running
func (m *SettlementMonitor) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// runMonitoringLoop is the main monitoring loop
func (m *SettlementMonitor) runMonitoringLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	// Run immediately on start
	m.checkForStalledSettlements()

	for {
		select {
		case <-ticker.C:
			m.checkForStalledSettlements()
		case <-m.stopChan:
			log.Println("[SETTLEMENT-MONITOR] Received stop signal")
			return
		}
	}
}

// checkForStalledSettlements checks for failed settlements that haven't been alerted
func (m *SettlementMonitor) checkForStalledSettlements() {
	if !m.config.Enabled {
		return
	}

	ctx := context.Background()

	// Get failed settlements older than threshold that haven't been alerted
	failures, err := m.repo.GetFailedSettlements(ctx, m.config.AlertThreshold)
	if err != nil {
		log.Printf("[SETTLEMENT-MONITOR] Error getting failed settlements: %v", err)
		return
	}

	if len(failures) == 0 {
		return
	}

	log.Printf("[SETTLEMENT-MONITOR] Found %d stalled settlements needing alert", len(failures))

	for _, failure := range failures {
		// Send alert
		err := m.sendFailureAlert(ctx, &failure)
		if err != nil {
			log.Printf("[SETTLEMENT-MONITOR] Failed to send alert for user %s: %v", failure.UserID, err)
			continue
		}

		// Mark as alerted
		err = m.repo.MarkSettlementAlerted(ctx, failure.UserID, failure.SummaryDate)
		if err != nil {
			log.Printf("[SETTLEMENT-MONITOR] Failed to mark settlement as alerted: %v", err)
		}
	}
}

// sendFailureAlert sends an email alert for a failed settlement
// FIX: Added mutex for config access and safe user ID handling (Issues #10, #11)
func (m *SettlementMonitor) sendFailureAlert(ctx context.Context, failure *database.DailyModeSummary) error {
	if m.emailService == nil || !m.emailService.IsSMTPConfigured(ctx) {
		log.Printf("[SETTLEMENT-MONITOR] Email not configured, logging alert instead")
		log.Printf("[SETTLEMENT-MONITOR] ALERT: Settlement failed for user %s on %s: %s",
			failure.UserID, failure.SummaryDate.Format("2006-01-02"), safeString(failure.SettlementError))
		return nil
	}

	// FIX: Read config under mutex to prevent race condition (Issue #10)
	m.mu.Lock()
	adminEmail := m.config.AdminEmail
	m.mu.Unlock()

	if adminEmail == "" {
		log.Printf("[SETTLEMENT-MONITOR] No admin email configured, skipping email alert")
		return nil
	}

	// Calculate how long ago it failed
	failedSince := time.Since(failure.SettlementTime)
	failedHours := failedSince.Hours()

	// FIX: Safe user ID truncation to prevent panic (Issue #11)
	// Use last 8 chars of UUID to avoid PII correlation in email subjects
	userIDShort := failure.UserID
	if len(userIDShort) > 8 {
		userIDShort = userIDShort[len(userIDShort)-8:] // Last 8 chars
	}

	// Build email
	subject := fmt.Sprintf("Settlement Failed: User ...%s - %s",
		userIDShort, failure.SummaryDate.Format("2006-01-02"))

	body := fmt.Sprintf(`Settlement failed and needs manual intervention:

User ID: %s
Date: %s
Error: %s
Failed Since: %s (%.1f hours ago)

Settlement Status: %s
User Timezone: %s

To retry: POST /api/admin/settlements/retry/%s/%s

This is an automated alert from the Settlement Monitoring System.
`,
		failure.UserID,
		failure.SummaryDate.Format("2006-01-02"),
		safeString(failure.SettlementError),
		failure.SettlementTime.Format(time.RFC3339),
		failedHours,
		failure.SettlementStatus,
		failure.UserTimezone,
		failure.UserID,
		failure.SummaryDate.Format("2006-01-02"),
	)

	return m.emailService.SendEmail(ctx, adminEmail, subject, body)
}

// SetAdminEmail sets the admin email for alerts
// FIX: Added mutex to prevent race condition with sendFailureAlert (Issue #10)
func (m *SettlementMonitor) SetAdminEmail(email string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.AdminEmail = email
}

// GetMetrics returns current monitoring metrics
func (m *SettlementMonitor) GetMetrics(ctx context.Context) (*MonitoringMetrics, error) {
	// Get settlement status summary from last 24 hours
	now := time.Now()
	startDate := now.AddDate(0, 0, -1)

	filter := database.AdminSummaryFilter{
		StartDate: startDate,
		EndDate:   now,
		Limit:     10000,
	}

	result, err := m.repo.GetAdminDailySummaries(ctx, filter)
	if err != nil {
		return nil, err
	}

	metrics := &MonitoringMetrics{
		Timestamp: now,
	}

	for _, summary := range result.Summaries {
		switch summary.SettlementStatus {
		case "completed":
			metrics.CompletedCount++
		case "failed":
			metrics.FailedCount++
		case "retrying":
			metrics.RetryingCount++
		}
	}

	metrics.TotalCount = metrics.CompletedCount + metrics.FailedCount + metrics.RetryingCount
	if metrics.TotalCount > 0 {
		metrics.SuccessRate = float64(metrics.CompletedCount) / float64(metrics.TotalCount) * 100
	}

	return metrics, nil
}

// MonitoringMetrics represents monitoring statistics
type MonitoringMetrics struct {
	Timestamp      time.Time `json:"timestamp"`
	TotalCount     int       `json:"total_count"`
	CompletedCount int       `json:"completed_count"`
	FailedCount    int       `json:"failed_count"`
	RetryingCount  int       `json:"retrying_count"`
	SuccessRate    float64   `json:"success_rate"`
}

// safeString returns the string value or "N/A" if nil
func safeString(s *string) string {
	if s == nil {
		return "N/A"
	}
	return *s
}
