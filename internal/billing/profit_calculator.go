package billing

import (
	"context"
	"fmt"
	"log"
	"time"

	"binance-trading-bot/internal/database"
)

// ProfitCalculator calculates profit and profit share for users
type ProfitCalculator struct {
	repo   *database.Repository
	config *BillingConfig
}

// NewProfitCalculator creates a new profit calculator
func NewProfitCalculator(repo *database.Repository, config *BillingConfig) *ProfitCalculator {
	if config == nil {
		config = DefaultBillingConfig()
	}
	return &ProfitCalculator{
		repo:   repo,
		config: config,
	}
}

// CalculatePeriodProfit calculates profit for a specific period
// This is the core profit calculation with high-water mark and loss carryforward
func (p *ProfitCalculator) CalculatePeriodProfit(ctx context.Context, userID string, periodStart, periodEnd time.Time) (*ProfitReport, error) {
	// 1. Get user info for profit share rate
	user, err := p.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	profitShareRate := GetProfitShareRate(SubscriptionTier(user.SubscriptionTier))

	// 2. Get starting balance (from period start snapshot or previous period end)
	startingBalance, err := p.getBalanceAtTime(ctx, userID, periodStart)
	if err != nil {
		return nil, fmt.Errorf("failed to get starting balance: %w", err)
	}

	// 3. Get ending balance
	endingBalance, err := p.getBalanceAtTime(ctx, userID, periodEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get ending balance: %w", err)
	}

	// 4. Get deposits and withdrawals during the period
	deposits, withdrawals, err := p.getTransactionsForPeriod(ctx, userID, periodStart, periodEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	// 5. Calculate gross profit (excluding deposits/withdrawals)
	// Formula: Gross Profit = (Ending Balance - Starting Balance) - (Deposits - Withdrawals)
	netTransactions := deposits - withdrawals
	grossProfit := (endingBalance - startingBalance) - netTransactions

	// 6. Get previous loss carryforward and high-water mark
	previousPeriod, err := p.getPreviousProfitPeriod(ctx, userID, periodStart)
	if err != nil {
		log.Printf("Warning: failed to get previous period: %v", err)
	}

	var previousLossCarry float64
	var previousHighWaterMark float64
	if previousPeriod != nil {
		previousLossCarry = previousPeriod.LossCarryforward
		previousHighWaterMark = previousPeriod.HighWaterMark
	} else {
		// First period - use starting balance as high-water mark
		previousHighWaterMark = startingBalance
	}

	// 7. Apply loss carryforward
	// Net profit = Gross profit - Previous losses carried forward
	netProfit := grossProfit
	if previousLossCarry > 0 {
		netProfit = grossProfit - previousLossCarry
	}

	// 8. Calculate new loss carryforward (if still in loss)
	var newLossCarryforward float64
	if netProfit < 0 {
		newLossCarryforward = -netProfit
		netProfit = 0
	}

	// 9. Apply high-water mark
	// Only pay profit share on new profits above the previous high-water mark
	currentBalance := endingBalance
	var profitAboveHWM float64
	var newHighWaterMark float64

	if currentBalance > previousHighWaterMark {
		profitAboveHWM = currentBalance - previousHighWaterMark
		newHighWaterMark = currentBalance
	} else {
		profitAboveHWM = 0
		newHighWaterMark = previousHighWaterMark // HWM doesn't decrease
	}

	// 10. Calculate profit share due
	// Use the lesser of netProfit and profitAboveHWM to ensure we don't double-charge
	taxableProfit := netProfit
	if profitAboveHWM < taxableProfit {
		taxableProfit = profitAboveHWM
	}

	var profitShareDue float64
	if taxableProfit > 0 {
		profitShareDue = taxableProfit * profitShareRate
	}

	// 11. Get trade statistics for the period
	totalTrades, winningTrades, losingTrades, err := p.getTradeStats(ctx, userID, periodStart, periodEnd)
	if err != nil {
		log.Printf("Warning: failed to get trade stats: %v", err)
	}

	var winRate float64
	if totalTrades > 0 {
		winRate = float64(winningTrades) / float64(totalTrades) * 100
	}

	return &ProfitReport{
		UserID:              userID,
		PeriodStart:         periodStart,
		PeriodEnd:           periodEnd,
		StartingBalance:     startingBalance,
		EndingBalance:       endingBalance,
		TotalDeposits:       deposits,
		TotalWithdrawals:    withdrawals,
		GrossProfit:         grossProfit,
		PreviousLossCarry:   previousLossCarry,
		NetProfit:           netProfit,
		NewHighWaterMark:    newHighWaterMark,
		ProfitAboveHWM:      profitAboveHWM,
		NewLossCarryforward: newLossCarryforward,
		ProfitShareRate:     profitShareRate,
		ProfitShareDue:      profitShareDue,
		TotalTrades:         totalTrades,
		WinningTrades:       winningTrades,
		LosingTrades:        losingTrades,
		WinRate:             winRate,
	}, nil
}

// CalculateWeeklyProfit calculates profit for the current week
func (p *ProfitCalculator) CalculateWeeklyProfit(ctx context.Context, userID string) (*ProfitReport, error) {
	now := time.Now().UTC()

	// Find the start of the current week (Sunday)
	daysUntilSunday := int(now.Weekday())
	periodStart := time.Date(now.Year(), now.Month(), now.Day()-daysUntilSunday, 0, 0, 0, 0, time.UTC)
	periodEnd := now

	return p.CalculatePeriodProfit(ctx, userID, periodStart, periodEnd)
}

// CalculateLastWeekProfit calculates profit for the previous complete week
func (p *ProfitCalculator) CalculateLastWeekProfit(ctx context.Context, userID string) (*ProfitReport, error) {
	now := time.Now().UTC()

	// Find the start of the current week (Sunday)
	daysUntilSunday := int(now.Weekday())
	currentWeekStart := time.Date(now.Year(), now.Month(), now.Day()-daysUntilSunday, 0, 0, 0, 0, time.UTC)

	// Last week
	periodStart := currentWeekStart.AddDate(0, 0, -7)
	periodEnd := currentWeekStart

	return p.CalculatePeriodProfit(ctx, userID, periodStart, periodEnd)
}

// GetProfitHistory returns the profit history for a user
func (p *ProfitCalculator) GetProfitHistory(ctx context.Context, userID string, limit int) ([]database.ProfitPeriod, error) {
	return p.repo.GetUserProfitPeriods(ctx, userID, limit)
}

// SaveProfitPeriod saves a calculated profit period to the database
func (p *ProfitCalculator) SaveProfitPeriod(ctx context.Context, report *ProfitReport) (*database.ProfitPeriod, error) {
	period := &database.ProfitPeriod{
		UserID:           report.UserID,
		PeriodStart:      report.PeriodStart,
		PeriodEnd:        report.PeriodEnd,
		StartingBalance:  report.StartingBalance,
		EndingBalance:    report.EndingBalance,
		Deposits:         report.TotalDeposits,
		Withdrawals:      report.TotalWithdrawals,
		GrossProfit:      report.GrossProfit,
		LossCarryforward: report.NewLossCarryforward,
		NetProfit:        report.NetProfit,
		HighWaterMark:    report.NewHighWaterMark,
		ProfitShareRate:  report.ProfitShareRate,
		ProfitShareDue:   report.ProfitShareDue,
		SettlementStatus: string(StatusPending),
	}

	if err := p.repo.CreateProfitPeriod(ctx, period); err != nil {
		return nil, fmt.Errorf("failed to save profit period: %w", err)
	}

	return period, nil
}

// ShouldInvoice checks if a profit report meets the minimum payout threshold
func (p *ProfitCalculator) ShouldInvoice(report *ProfitReport) bool {
	return report.ProfitShareDue >= p.config.MinimumPayout
}

// Helper functions

// getBalanceAtTime gets the user's balance at a specific point in time
func (p *ProfitCalculator) getBalanceAtTime(ctx context.Context, userID string, timestamp time.Time) (float64, error) {
	snapshot, err := p.repo.GetBalanceSnapshotNear(ctx, userID, timestamp)
	if err != nil {
		return 0, err
	}
	if snapshot != nil {
		return snapshot.TotalBalance, nil
	}

	// If no snapshot, try to calculate from trades
	return p.calculateBalanceFromTrades(ctx, userID, timestamp)
}

// calculateBalanceFromTrades calculates balance by summing up all trades
func (p *ProfitCalculator) calculateBalanceFromTrades(ctx context.Context, userID string, upTo time.Time) (float64, error) {
	// This is a fallback - ideally we always have snapshots
	// For now, return 0 and let the caller handle it
	log.Printf("Warning: No balance snapshot found for user %s at %v", userID, upTo)
	return 0, nil
}

// getTransactionsForPeriod gets total deposits and withdrawals for a period
func (p *ProfitCalculator) getTransactionsForPeriod(ctx context.Context, userID string, start, end time.Time) (deposits, withdrawals float64, err error) {
	transactions, err := p.repo.GetUserTransactions(ctx, userID, start, end)
	if err != nil {
		return 0, 0, err
	}

	for _, tx := range transactions {
		if tx.Status != "confirmed" {
			continue
		}
		switch tx.Type {
		case "deposit":
			deposits += tx.Amount
		case "withdrawal":
			withdrawals += tx.Amount
		}
	}

	return deposits, withdrawals, nil
}

// getPreviousProfitPeriod gets the most recent completed profit period
func (p *ProfitCalculator) getPreviousProfitPeriod(ctx context.Context, userID string, beforeDate time.Time) (*database.ProfitPeriod, error) {
	return p.repo.GetLatestProfitPeriod(ctx, userID, beforeDate)
}

// getTradeStats gets trade statistics for a period
func (p *ProfitCalculator) getTradeStats(ctx context.Context, userID string, start, end time.Time) (total, winning, losing int, err error) {
	// Get spot trades
	spotTrades, err := p.repo.GetTradesForPeriod(ctx, userID, start, end)
	if err != nil {
		return 0, 0, 0, err
	}

	for _, trade := range spotTrades {
		if trade.Status != "CLOSED" {
			continue
		}
		total++
		if trade.PnL != nil && *trade.PnL > 0 {
			winning++
		} else {
			losing++
		}
	}

	// Get futures trades
	futuresTrades, err := p.repo.GetFuturesTradesForPeriod(ctx, userID, start, end)
	if err != nil {
		return total, winning, losing, nil // Don't fail if futures not enabled
	}

	for _, trade := range futuresTrades {
		if trade.Status != "CLOSED" {
			continue
		}
		total++
		if trade.RealizedPnL != nil && *trade.RealizedPnL > 0 {
			winning++
		} else {
			losing++
		}
	}

	return total, winning, losing, nil
}

// CreateBalanceSnapshot creates a balance snapshot for a user
func (p *ProfitCalculator) CreateBalanceSnapshot(ctx context.Context, userID string, snapshotType string, balance, unrealizedPnL float64) error {
	snapshot := &database.BalanceSnapshot{
		UserID:        userID,
		SnapshotType:  snapshotType,
		TotalBalance:  balance,
		UnrealizedPnL: unrealizedPnL,
		CreatedAt:     time.Now().UTC(),
	}
	return p.repo.CreateBalanceSnapshot(ctx, snapshot)
}

// GetCurrentBalance gets the current total balance for a user (from Binance)
// This is a placeholder - actual implementation would call Binance API
func (p *ProfitCalculator) GetCurrentBalance(ctx context.Context, userID string) (float64, float64, error) {
	// TODO: Implement actual balance fetch from Binance
	// For now, return the latest snapshot
	snapshot, err := p.repo.GetLatestBalanceSnapshot(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	if snapshot != nil {
		return snapshot.TotalBalance, snapshot.UnrealizedPnL, nil
	}
	return 0, 0, nil
}
