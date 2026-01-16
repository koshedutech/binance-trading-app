// Package settlement provides data quality validation for Epic 8 Story 8.10.
// Validates settlement data and flags anomalies for admin review.
package settlement

import (
	"fmt"
	"math"
	"strings"

	"binance-trading-bot/internal/database"
)

// ValidationConfig holds configurable validation thresholds
// These are admin system settings, not per-user settings
type ValidationConfig struct {
	PnLMin          float64 // Minimum expected P&L (e.g., -10000)
	PnLMax          float64 // Maximum expected P&L (e.g., 10000)
	MaxTradeCount   int     // Maximum trades per day before flagging (e.g., 500)
	UnrealizedDiff  float64 // Max acceptable difference from Binance (e.g., 100)
}

// DefaultValidationConfig returns default validation thresholds
func DefaultValidationConfig() *ValidationConfig {
	return &ValidationConfig{
		PnLMin:          -10000,
		PnLMax:          10000,
		MaxTradeCount:   500,
		UnrealizedDiff:  100,
	}
}

// ValidationResult holds the result of data validation
type ValidationResult struct {
	IsValid  bool     `json:"is_valid"`
	Errors   []string `json:"errors"`   // Hard failures (reject settlement)
	Warnings []string `json:"warnings"` // Anomalies (flag for review)
}

// DataValidator validates settlement data quality
type DataValidator struct {
	config *ValidationConfig
}

// NewDataValidator creates a new validator with the given config
func NewDataValidator(config *ValidationConfig) *DataValidator {
	if config == nil {
		config = DefaultValidationConfig()
	}
	return &DataValidator{config: config}
}

// ValidateSummary validates a daily mode summary for data quality
func (v *DataValidator) ValidateSummary(summary *database.DailyModeSummary) *ValidationResult {
	result := &ValidationResult{
		IsValid:  true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Skip validation for "ALL" mode aggregated rows
	if summary.Mode == ModeAll {
		return result
	}

	// === HARD ERRORS (reject settlement) ===

	// Win rate must be between 0-100
	if summary.WinRate < 0 || summary.WinRate > 100 {
		result.Errors = append(result.Errors,
			fmt.Sprintf("Invalid win rate: %.2f%% (must be 0-100)", summary.WinRate))
		result.IsValid = false
	}

	// Win/loss count consistency check
	if summary.WinCount+summary.LossCount != summary.TradeCount {
		result.Errors = append(result.Errors,
			fmt.Sprintf("Win+Loss count mismatch: %d wins + %d losses != %d total trades",
				summary.WinCount, summary.LossCount, summary.TradeCount))
		result.IsValid = false
	}

	// Trade count cannot be negative
	if summary.TradeCount < 0 {
		result.Errors = append(result.Errors,
			fmt.Sprintf("Negative trade count: %d", summary.TradeCount))
		result.IsValid = false
	}

	// === WARNINGS (flag for admin review) ===

	// P&L bounds check
	if summary.TotalPnL < v.config.PnLMin {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Large loss: $%.2f (below threshold $%.2f)",
				summary.TotalPnL, v.config.PnLMin))
	}

	if summary.TotalPnL > v.config.PnLMax {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Large profit: $%.2f (above threshold $%.2f)",
				summary.TotalPnL, v.config.PnLMax))
	}

	// High trade count check
	if summary.TradeCount > v.config.MaxTradeCount {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("High trade count: %d (threshold: %d)",
				summary.TradeCount, v.config.MaxTradeCount))
	}

	// Largest win/loss sanity check (largest win should be positive)
	if summary.LargestWin < 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Suspicious largest win: $%.2f (negative value)", summary.LargestWin))
	}

	// Largest loss should be negative
	if summary.LargestLoss > 0 && summary.LossCount > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Suspicious largest loss: $%.2f (positive value)", summary.LargestLoss))
	}

	// Volume sanity check
	if summary.TotalVolume < 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Negative volume: $%.2f", summary.TotalVolume))
	}

	return result
}

// ValidateUnrealizedPnL validates unrealized P&L against Binance snapshot
func (v *DataValidator) ValidateUnrealizedPnL(stored, binanceActual float64) *ValidationResult {
	result := &ValidationResult{
		IsValid:  true,
		Errors:   []string{},
		Warnings: []string{},
	}

	diff := math.Abs(stored - binanceActual)
	if diff > v.config.UnrealizedDiff {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Unrealized P&L mismatch: Stored=$%.2f, Binance=$%.2f (diff: $%.2f)",
				stored, binanceActual, diff))
	}

	return result
}

// ApplyValidation applies validation result to a summary
func (v *DataValidator) ApplyValidation(summary *database.DailyModeSummary, validationResult *ValidationResult) {
	if len(validationResult.Warnings) > 0 {
		summary.DataQualityFlag = true
		notes := strings.Join(validationResult.Warnings, "; ")
		summary.DataQualityNotes = &notes
	}
}

// ValidateAndApply validates a summary and applies the results
// Returns the validation result and an error if validation failed with hard errors
func (v *DataValidator) ValidateAndApply(summary *database.DailyModeSummary) (*ValidationResult, error) {
	result := v.ValidateSummary(summary)

	// Apply warnings to summary
	v.ApplyValidation(summary, result)

	// Return error if hard failures exist
	if !result.IsValid {
		return result, fmt.Errorf("validation failed: %v", result.Errors)
	}

	return result, nil
}

// BatchValidate validates multiple summaries
func (v *DataValidator) BatchValidate(summaries []database.DailyModeSummary) ([]ValidationResult, bool) {
	results := make([]ValidationResult, len(summaries))
	allValid := true

	for i := range summaries {
		result := v.ValidateSummary(&summaries[i])
		results[i] = *result
		v.ApplyValidation(&summaries[i], result)

		if !result.IsValid {
			allValid = false
		}
	}

	return results, allValid
}
