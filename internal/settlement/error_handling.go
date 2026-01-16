// Package settlement provides error handling and retry logic for Epic 8 Story 8.8.
// Implements exponential backoff retry for Binance API and database failures.
package settlement

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

// RetryConfig defines retry behavior
type RetryConfig struct {
	MaxRetries      int
	BackoffDelays   []time.Duration
	DBRetryDelay    time.Duration
	DBMaxRetries    int
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    3,
		BackoffDelays: []time.Duration{5 * time.Second, 15 * time.Second, 45 * time.Second},
		DBRetryDelay:  10 * time.Second,
		DBMaxRetries:  1,
	}
}

// SettlementError represents a detailed settlement error
type SettlementError struct {
	UserID    string
	Date      time.Time
	Phase     string // snapshot, aggregate, store, validate
	Attempt   int
	Error     error
	Timestamp time.Time
	Retryable bool
}

func (e *SettlementError) String() string {
	return fmt.Sprintf("[%s] User %s, Date %s, Phase %s, Attempt %d: %v",
		e.Timestamp.Format(time.RFC3339), e.UserID, e.Date.Format("2006-01-02"),
		e.Phase, e.Attempt, e.Error)
}

// ErrorClassifier classifies errors as retryable or not
type ErrorClassifier struct{}

// IsRetryable determines if an error should trigger a retry
func (c *ErrorClassifier) IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Binance rate limit, timeout, connection errors - retryable
	retryablePatterns := []string{
		"rate limit",
		"timeout",
		"connection refused",
		"connection reset",
		"temporary failure",
		"service unavailable",
		"gateway timeout",
		"too many requests",
		"429",
		"503",
		"504",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Database deadlock, connection errors - retryable
	dbRetryablePatterns := []string{
		"deadlock",
		"connection",
		"lock timeout",
		"serialization failure",
	}

	for _, pattern := range dbRetryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// IdentifyPhase identifies the phase where an error occurred based on error message
func (c *ErrorClassifier) IdentifyPhase(err error) string {
	if err == nil {
		return "unknown"
	}

	errStr := strings.ToLower(err.Error())

	if strings.Contains(errStr, "snapshot") {
		return "snapshot"
	}
	if strings.Contains(errStr, "aggregate") || strings.Contains(errStr, "trade") {
		return "aggregate"
	}
	if strings.Contains(errStr, "save") || strings.Contains(errStr, "store") || strings.Contains(errStr, "database") {
		return "store"
	}
	if strings.Contains(errStr, "valid") {
		return "validate"
	}

	return "unknown"
}

// RetryableSettlementService wraps SettlementService with retry logic
type RetryableSettlementService struct {
	service    *SettlementService
	config     *RetryConfig
	classifier *ErrorClassifier
	onError    func(SettlementError) // Optional error callback
}

// NewRetryableSettlementService creates a settlement service with retry support
func NewRetryableSettlementService(service *SettlementService, config *RetryConfig) *RetryableSettlementService {
	if config == nil {
		config = DefaultRetryConfig()
	}

	return &RetryableSettlementService{
		service:    service,
		config:     config,
		classifier: &ErrorClassifier{},
	}
}

// SetErrorCallback sets an optional callback for error logging/alerting
func (r *RetryableSettlementService) SetErrorCallback(callback func(SettlementError)) {
	r.onError = callback
}

// RunDailySettlementWithRetry runs settlement with exponential backoff retry
func (r *RetryableSettlementService) RunDailySettlementWithRetry(ctx context.Context, userID string, settlementDate time.Time, timezone string) (*SettlementResult, error) {
	var lastErr error

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// Run settlement
		result, err := r.service.RunDailySettlement(ctx, userID, settlementDate, timezone)

		if err == nil && result.Success {
			return result, nil
		}

		lastErr = err
		if err == nil && !result.Success {
			lastErr = errors.New(result.Error)
		}

		// Log the error
		settlementErr := SettlementError{
			UserID:    userID,
			Date:      settlementDate,
			Phase:     r.classifier.IdentifyPhase(lastErr),
			Attempt:   attempt + 1,
			Error:     lastErr,
			Timestamp: time.Now(),
			Retryable: r.classifier.IsRetryable(lastErr),
		}

		log.Printf("[SETTLEMENT-RETRY] %s", settlementErr.String())

		if r.onError != nil {
			r.onError(settlementErr)
		}

		// Check if retryable
		if !settlementErr.Retryable {
			log.Printf("[SETTLEMENT-RETRY] Non-retryable error for user %s, marking as failed", userID)
			r.markSettlementFailed(ctx, userID, settlementDate, lastErr)
			return result, lastErr
		}

		// Don't sleep after last attempt
		if attempt < r.config.MaxRetries {
			// FIX: Bounds check to prevent panic if BackoffDelays is shorter than MaxRetries (Issue #9)
			delayIdx := attempt
			if delayIdx >= len(r.config.BackoffDelays) {
				delayIdx = len(r.config.BackoffDelays) - 1
			}
			delay := r.config.BackoffDelays[delayIdx]
			log.Printf("[SETTLEMENT-RETRY] Waiting %v before retry %d for user %s", delay, attempt+2, userID)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				// Continue to retry
			}
		}
	}

	// All retries exhausted
	log.Printf("[SETTLEMENT-RETRY] Max retries exceeded for user %s", userID)
	r.markSettlementFailed(ctx, userID, settlementDate, lastErr)

	return nil, fmt.Errorf("settlement failed after %d retries: %w", r.config.MaxRetries+1, lastErr)
}

// markSettlementFailed marks the settlement as failed in the database
func (r *RetryableSettlementService) markSettlementFailed(ctx context.Context, userID string, settlementDate time.Time, err error) {
	errMsg := err.Error()
	updateErr := r.service.repo.UpdateSettlementStatus(ctx, userID, settlementDate, ModeAll, "failed", &errMsg)
	if updateErr != nil {
		log.Printf("[SETTLEMENT-RETRY] Failed to mark settlement as failed: %v", updateErr)
	}
}

// GetService returns the underlying settlement service
func (r *RetryableSettlementService) GetService() *SettlementService {
	return r.service
}
