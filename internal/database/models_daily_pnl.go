// Package database provides models for daily P&L summaries.
// Epic 13 Story 13.1: PNL Summary Caching & Historical Navigation
package database

import (
	"time"
)

// UserDailyPnLSummary represents a cached daily P&L summary for a user
// Historical records (past days) are immutable once created
type UserDailyPnLSummary struct {
	ID         int       `json:"id"`
	UserID     string    `json:"user_id"`
	Date       time.Time `json:"date"`

	// P&L components
	PnL        float64 `json:"pnl"`        // Gross realized P&L
	Commission float64 `json:"commission"` // Commission fees paid (positive)
	Funding    float64 `json:"funding"`    // Net funding fee (positive = paid, negative = received)
	NetPnL     float64 `json:"net_pnl"`    // Final P&L after all fees

	// Trade metrics
	TradeCount int `json:"trade_count"` // Number of REALIZED_PNL records

	// Additional fee details
	Rebate float64 `json:"rebate"` // Commission rebates
	Other  float64 `json:"other"`  // Other income types

	// Metadata
	FetchedAt time.Time `json:"fetched_at"` // When fetched from Binance
	CreatedAt time.Time `json:"created_at"`
}

// DailyPnLBreakdown is the format used by the frontend for the 7-day calendar view
// This matches the existing daily_breakdown structure in handleGetPnLSummary
type DailyPnLBreakdown struct {
	Date       string  `json:"date"`
	Day        int     `json:"day"`
	DayName    string  `json:"day_name"`
	PnL        float64 `json:"pnl"`
	Commission float64 `json:"commission"`
	Rebate     float64 `json:"rebate"`
	Funding    float64 `json:"funding"`
	Other      float64 `json:"other"`
	NetPnL     float64 `json:"net_pnl"`
	TradeCount int     `json:"trade_count"`
	IsProfit   bool    `json:"is_profit"`
	IsToday    bool    `json:"is_today"`
	IsCached   bool    `json:"is_cached"` // True if loaded from DB cache
}

// ToDailyBreakdown converts a UserDailyPnLSummary to the frontend DailyPnLBreakdown format
func (s *UserDailyPnLSummary) ToDailyBreakdown(isToday bool) DailyPnLBreakdown {
	return DailyPnLBreakdown{
		Date:       s.Date.Format("2006-01-02"),
		Day:        s.Date.Day(),
		DayName:    s.Date.Format("Mon"),
		PnL:        s.PnL,
		Commission: s.Commission,
		Rebate:     s.Rebate,
		Funding:    s.Funding,
		Other:      s.Other,
		NetPnL:     s.NetPnL,
		TradeCount: s.TradeCount,
		IsProfit:   s.NetPnL >= 0,
		IsToday:    isToday,
		IsCached:   true,
	}
}

// NewUserDailyPnLSummary creates a new summary record from fetched data
func NewUserDailyPnLSummary(userID string, date time.Time, pnl, commission, funding, rebate, other, netPnL float64, tradeCount int) *UserDailyPnLSummary {
	return &UserDailyPnLSummary{
		UserID:     userID,
		Date:       date,
		PnL:        pnl,
		Commission: commission,
		Funding:    funding,
		Rebate:     rebate,
		Other:      other,
		NetPnL:     netPnL,
		TradeCount: tradeCount,
		FetchedAt:  time.Now(),
	}
}
