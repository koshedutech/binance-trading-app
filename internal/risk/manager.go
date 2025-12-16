package risk

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// RiskManager handles position sizing and risk management
type RiskManager struct {
	config          *Config
	dailyPnL        float64
	dailyPnLReset   time.Time
	openPositions   int
	accountBalance  float64
	mu              sync.RWMutex
}

// Config holds risk management configuration
type Config struct {
	MaxRiskPerTrade        float64 // Percentage of account to risk per trade
	MaxDailyDrawdown       float64 // Max daily loss percentage before stopping
	MaxOpenPositions       int     // Maximum concurrent positions
	PositionSizeMethod     string  // "fixed", "percent", "kelly", "atr"
	FixedPositionSize      float64 // Fixed position size in quote currency
	UseTrailingStop        bool    // Enable trailing stop loss
	TrailingStopPercent    float64 // Trailing stop distance percentage
	TrailingStopActivation float64 // Profit % to activate trailing stop
}

// NewRiskManager creates a new risk manager
func NewRiskManager(config *Config) *RiskManager {
	return &RiskManager{
		config:        config,
		dailyPnLReset: time.Now().Truncate(24 * time.Hour),
	}
}

// UpdateAccountBalance updates the current account balance
func (rm *RiskManager) UpdateAccountBalance(balance float64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.accountBalance = balance
}

// GetAccountBalance returns the current account balance
func (rm *RiskManager) GetAccountBalance() float64 {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.accountBalance
}

// CanOpenPosition checks if a new position can be opened
func (rm *RiskManager) CanOpenPosition() (bool, string) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Check max positions
	if rm.openPositions >= rm.config.MaxOpenPositions {
		return false, fmt.Sprintf("max positions reached (%d/%d)", rm.openPositions, rm.config.MaxOpenPositions)
	}

	// Check daily drawdown
	rm.checkDailyReset()
	if rm.accountBalance > 0 {
		dailyDrawdownPercent := (rm.dailyPnL / rm.accountBalance) * 100
		if dailyDrawdownPercent <= -rm.config.MaxDailyDrawdown {
			return false, fmt.Sprintf("daily drawdown limit reached (%.2f%%)", dailyDrawdownPercent)
		}
	}

	return true, ""
}

// CalculatePositionSize calculates the appropriate position size
func (rm *RiskManager) CalculatePositionSize(entryPrice, stopLoss float64) float64 {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	switch rm.config.PositionSizeMethod {
	case "fixed":
		return rm.calculateFixedSize(entryPrice)
	case "percent":
		return rm.calculatePercentSize(entryPrice, stopLoss)
	case "kelly":
		return rm.calculateKellySize(entryPrice, stopLoss)
	case "atr":
		return rm.calculateATRSize(entryPrice, stopLoss)
	default:
		return rm.calculatePercentSize(entryPrice, stopLoss)
	}
}

// calculateFixedSize returns a fixed position size
func (rm *RiskManager) calculateFixedSize(entryPrice float64) float64 {
	if entryPrice <= 0 {
		return 0
	}
	// Convert fixed dollar amount to quantity
	return rm.config.FixedPositionSize / entryPrice
}

// calculatePercentSize calculates position size based on risk percentage
func (rm *RiskManager) calculatePercentSize(entryPrice, stopLoss float64) float64 {
	if entryPrice <= 0 || stopLoss <= 0 || rm.accountBalance <= 0 {
		return 0
	}

	// Risk amount in dollars
	riskAmount := rm.accountBalance * (rm.config.MaxRiskPerTrade / 100)

	// Risk per unit (distance to stop loss)
	riskPerUnit := math.Abs(entryPrice - stopLoss)
	if riskPerUnit == 0 {
		return 0
	}

	// Position size = Risk Amount / Risk Per Unit
	positionSize := riskAmount / riskPerUnit

	log.Printf("Position sizing: Balance=%.2f, Risk%%=%.2f%%, RiskAmt=%.2f, Entry=%.4f, SL=%.4f, Size=%.8f",
		rm.accountBalance, rm.config.MaxRiskPerTrade, riskAmount, entryPrice, stopLoss, positionSize)

	return positionSize
}

// calculateKellySize uses Kelly Criterion for position sizing
func (rm *RiskManager) calculateKellySize(entryPrice, stopLoss float64) float64 {
	// Kelly Criterion: f* = (bp - q) / b
	// where b = odds, p = win probability, q = loss probability
	// Using conservative half-Kelly

	// Default values (should be calculated from historical performance)
	winRate := 0.55        // 55% win rate
	avgWin := 1.5          // Average win is 1.5x the risk
	avgLoss := 1.0         // Average loss is 1x the risk

	b := avgWin / avgLoss
	p := winRate
	q := 1 - p

	kelly := (b*p - q) / b
	if kelly < 0 {
		kelly = 0
	}

	// Use half-Kelly for safety
	halfKelly := kelly / 2
	if halfKelly > 0.25 {
		halfKelly = 0.25 // Cap at 25%
	}

	// Calculate position size
	riskAmount := rm.accountBalance * halfKelly
	riskPerUnit := math.Abs(entryPrice - stopLoss)
	if riskPerUnit == 0 {
		return 0
	}

	return riskAmount / riskPerUnit
}

// calculateATRSize uses ATR for position sizing
func (rm *RiskManager) calculateATRSize(entryPrice, stopLoss float64) float64 {
	// This would require ATR input - for now use percent method
	return rm.calculatePercentSize(entryPrice, stopLoss)
}

// RegisterPositionOpen registers a new position opening
func (rm *RiskManager) RegisterPositionOpen() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.openPositions++
}

// RegisterPositionClose registers a position closing
func (rm *RiskManager) RegisterPositionClose(pnl float64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.openPositions--
	if rm.openPositions < 0 {
		rm.openPositions = 0
	}

	rm.checkDailyReset()
	rm.dailyPnL += pnl
}

// GetDailyPnL returns the current daily P&L
func (rm *RiskManager) GetDailyPnL() float64 {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.dailyPnL
}

// GetOpenPositionCount returns the number of open positions
func (rm *RiskManager) GetOpenPositionCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.openPositions
}

// checkDailyReset resets daily P&L if it's a new day
func (rm *RiskManager) checkDailyReset() {
	today := time.Now().Truncate(24 * time.Hour)
	if today.After(rm.dailyPnLReset) {
		rm.dailyPnL = 0
		rm.dailyPnLReset = today
	}
}

// GetRiskMetrics returns current risk metrics
func (rm *RiskManager) GetRiskMetrics() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	dailyDrawdownPercent := 0.0
	if rm.accountBalance > 0 {
		dailyDrawdownPercent = (rm.dailyPnL / rm.accountBalance) * 100
	}

	return map[string]interface{}{
		"account_balance":       rm.accountBalance,
		"daily_pnl":             rm.dailyPnL,
		"daily_drawdown_percent": dailyDrawdownPercent,
		"open_positions":        rm.openPositions,
		"max_positions":         rm.config.MaxOpenPositions,
		"max_risk_per_trade":    rm.config.MaxRiskPerTrade,
		"max_daily_drawdown":    rm.config.MaxDailyDrawdown,
		"position_size_method":  rm.config.PositionSizeMethod,
		"can_trade":             rm.openPositions < rm.config.MaxOpenPositions && dailyDrawdownPercent > -rm.config.MaxDailyDrawdown,
	}
}
