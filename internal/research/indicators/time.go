// Package indicators provides technical indicator calculations for research infrastructure.
// Story 15.4: Feature Calculator Engine - Time Features
package indicators

import (
	"time"
)

// TradingSession represents different trading sessions.
type TradingSession string

const (
	SessionAsian   TradingSession = "asian"    // Tokyo session: 00:00-09:00 UTC
	SessionEurope  TradingSession = "europe"   // London session: 08:00-16:00 UTC
	SessionAmerica TradingSession = "america"  // New York session: 13:00-22:00 UTC
	SessionOverlap TradingSession = "overlap"  // Session overlap periods
	SessionOff     TradingSession = "off"      // Off-market hours (weekend)
)

// TimeFeatures holds all time-related features for a candle.
// These features capture temporal patterns and market timing.
type TimeFeatures struct {
	HourOfDay   int            `json:"hour_of_day"`   // Hour in UTC (0-23)
	DayOfWeek   int            `json:"day_of_week"`   // Day of week (0=Sunday, 6=Saturday)
	IsWeekend   bool           `json:"is_weekend"`    // True if Saturday or Sunday
	Session     TradingSession `json:"session"`       // Trading session
	DayOfMonth  int            `json:"day_of_month"`  // Day of month (1-31)
	IsMonthEnd  bool           `json:"is_month_end"`  // True if last 3 days of month
}

// GetTradingSession determines the current trading session based on UTC hour.
// Crypto markets are 24/7, but traditional session times still influence volume.
func GetTradingSession(utcHour int, dayOfWeek int) TradingSession {
	// Weekend check (lower activity)
	if dayOfWeek == 0 || dayOfWeek == 6 {
		return SessionOff
	}

	// Session overlaps (highest volume typically)
	// London-New York overlap: 13:00-16:00 UTC
	if utcHour >= 13 && utcHour < 16 {
		return SessionOverlap
	}
	// Tokyo-London overlap: 08:00-09:00 UTC
	if utcHour >= 8 && utcHour < 9 {
		return SessionOverlap
	}

	// Individual sessions
	// Asian/Tokyo: 00:00-09:00 UTC
	if utcHour >= 0 && utcHour < 9 {
		return SessionAsian
	}
	// European/London: 08:00-16:00 UTC (excluding overlap periods counted above)
	if utcHour >= 9 && utcHour < 16 {
		return SessionEurope
	}
	// American/New York: 13:00-22:00 UTC (excluding overlap)
	if utcHour >= 16 && utcHour < 22 {
		return SessionAmerica
	}
	// Late night (22:00-00:00) - low activity
	return SessionOff
}

// IsMonthEnd checks if the date is in the last 3 days of the month.
// Month-end often sees rebalancing and increased volatility.
func IsMonthEnd(t time.Time) bool {
	// Get the last day of the current month
	year, month, _ := t.Date()
	firstOfNextMonth := time.Date(year, month+1, 1, 0, 0, 0, 0, t.Location())
	lastDayOfMonth := firstOfNextMonth.AddDate(0, 0, -1).Day()

	currentDay := t.Day()
	return currentDay >= lastDayOfMonth-2 // Last 3 days
}

// CalculateTimeFeatures calculates all time features for a candle.
func CalculateTimeFeatures(candleTime time.Time) *TimeFeatures {
	// Convert to UTC for consistency
	utcTime := candleTime.UTC()

	hour := utcTime.Hour()
	dayOfWeek := int(utcTime.Weekday())
	dayOfMonth := utcTime.Day()

	isWeekend := dayOfWeek == 0 || dayOfWeek == 6

	return &TimeFeatures{
		HourOfDay:   hour,
		DayOfWeek:   dayOfWeek,
		IsWeekend:   isWeekend,
		Session:     GetTradingSession(hour, dayOfWeek),
		DayOfMonth:  dayOfMonth,
		IsMonthEnd:  IsMonthEnd(utcTime),
	}
}

// SessionToInt converts TradingSession to an integer for ML models.
// 0=off, 1=asian, 2=europe, 3=america, 4=overlap
func SessionToInt(session TradingSession) int {
	switch session {
	case SessionAsian:
		return 1
	case SessionEurope:
		return 2
	case SessionAmerica:
		return 3
	case SessionOverlap:
		return 4
	default:
		return 0 // SessionOff
	}
}

// SessionFromInt converts an integer back to TradingSession.
func SessionFromInt(i int) TradingSession {
	switch i {
	case 1:
		return SessionAsian
	case 2:
		return SessionEurope
	case 3:
		return SessionAmerica
	case 4:
		return SessionOverlap
	default:
		return SessionOff
	}
}

// CyclicalHourSin returns the sine encoding of hour for cyclical representation.
// This captures the cyclical nature of time (hour 23 is close to hour 0).
func CyclicalHourSin(hour int) float64 {
	// Map hour (0-23) to angle (0-2*pi)
	angle := float64(hour) * 2 * pi / 24
	return sin(angle)
}

// CyclicalHourCos returns the cosine encoding of hour for cyclical representation.
func CyclicalHourCos(hour int) float64 {
	// Map hour (0-23) to angle (0-2*pi)
	angle := float64(hour) * 2 * pi / 24
	return cos(angle)
}

// CyclicalDaySin returns the sine encoding of day of week.
func CyclicalDaySin(dayOfWeek int) float64 {
	// Map day (0-6) to angle (0-2*pi)
	angle := float64(dayOfWeek) * 2 * pi / 7
	return sin(angle)
}

// CyclicalDayCos returns the cosine encoding of day of week.
func CyclicalDayCos(dayOfWeek int) float64 {
	// Map day (0-6) to angle (0-2*pi)
	angle := float64(dayOfWeek) * 2 * pi / 7
	return cos(angle)
}

// pi and trig functions from math package
const pi = 3.14159265358979323846

func sin(x float64) float64 {
	// Use Taylor series for accurate sin calculation
	// Normalize x to [-pi, pi]
	for x > pi {
		x -= 2 * pi
	}
	for x < -pi {
		x += 2 * pi
	}

	// Taylor series: sin(x) = x - x^3/3! + x^5/5! - x^7/7! + ...
	result := x
	term := x
	for i := 1; i <= 10; i++ {
		term *= -x * x / float64((2*i)*(2*i+1))
		result += term
	}
	return result
}

func cos(x float64) float64 {
	// Use Taylor series for accurate cos calculation
	// Normalize x to [-pi, pi]
	for x > pi {
		x -= 2 * pi
	}
	for x < -pi {
		x += 2 * pi
	}

	// Taylor series: cos(x) = 1 - x^2/2! + x^4/4! - x^6/6! + ...
	result := 1.0
	term := 1.0
	for i := 1; i <= 10; i++ {
		term *= -x * x / float64((2*i-1)*(2*i))
		result += term
	}
	return result
}
