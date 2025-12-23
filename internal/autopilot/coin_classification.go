package autopilot

import (
	"time"
)

// ==================== CLASSIFICATION ENUMS ====================

// VolatilityClass represents coin volatility based on ATR
type VolatilityClass string

const (
	VolatilityStable  VolatilityClass = "stable"  // ATR < 3%
	VolatilityMedium  VolatilityClass = "medium"  // ATR 3-6%
	VolatilityHigh    VolatilityClass = "high"    // ATR > 6%
)

// MarketCapClass represents coin market cap category
type MarketCapClass string

const (
	MarketCapBlueChip MarketCapClass = "blue_chip" // BTC, ETH
	MarketCapLarge    MarketCapClass = "large_cap" // SOL, BNB, XRP, ADA, etc.
	MarketCapMidSmall MarketCapClass = "mid_small" // Others
)

// MomentumClass represents 24h momentum category
type MomentumClass string

const (
	MomentumGainer  MomentumClass = "gainer"  // > +5% in 24h
	MomentumNeutral MomentumClass = "neutral" // -5% to +5%
	MomentumLoser   MomentumClass = "loser"   // < -5% in 24h
)

// ==================== CLASSIFICATION RESULTS ====================

// CoinClassification represents the complete classification of a coin
type CoinClassification struct {
	Symbol          string          `json:"symbol"`
	LastPrice       float64         `json:"last_price"`

	// Volatility classification
	Volatility      VolatilityClass `json:"volatility"`
	VolatilityATR   float64         `json:"volatility_atr"`     // ATR as % of price

	// Market cap classification
	MarketCap       MarketCapClass  `json:"market_cap"`

	// Momentum classification
	Momentum        MomentumClass   `json:"momentum"`
	Momentum24hPct  float64         `json:"momentum_24h_pct"`   // 24h change %

	// Additional metrics
	Volume24h       float64         `json:"volume_24h"`
	QuoteVolume24h  float64         `json:"quote_volume_24h"`

	// Computed scores
	RiskScore       float64         `json:"risk_score"`         // 0-1 combined risk
	OpportunityScore float64        `json:"opportunity_score"`  // 0-1 potential

	// Status
	Enabled         bool            `json:"enabled"`            // User preference
	LastUpdated     time.Time       `json:"last_updated"`
}

// ==================== USER SETTINGS ====================

// CategoryAllocation defines allocation settings for a category
type CategoryAllocation struct {
	Enabled           bool    `json:"enabled"`
	AllocationPercent float64 `json:"allocation_percent"` // % of total allocation
	MaxPositions      int     `json:"max_positions"`      // Max simultaneous positions in this category
}

// CoinPreference defines per-coin user preferences
type CoinPreference struct {
	Symbol   string `json:"symbol"`
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"` // Higher = more preferred (0 = default)
}

// CoinClassificationSettings holds all classification-related settings
type CoinClassificationSettings struct {
	// Volatility thresholds (% of price as ATR)
	VolatilityStableMax float64 `json:"volatility_stable_max"` // Default: 3.0
	VolatilityMediumMax float64 `json:"volatility_medium_max"` // Default: 6.0

	// Momentum thresholds (24h % change)
	MomentumGainerMin   float64 `json:"momentum_gainer_min"`   // Default: 5.0
	MomentumLoserMax    float64 `json:"momentum_loser_max"`    // Default: -5.0

	// Minimum requirements
	MinVolume24h        float64 `json:"min_volume_24h"`        // Min 24h quote volume in USDT

	// ATR calculation
	ATRPeriod           int     `json:"atr_period"`            // Default: 14
	ATRTimeframe        string  `json:"atr_timeframe"`         // Default: "1d"

	// Cache refresh
	RefreshIntervalSecs int     `json:"refresh_interval_secs"` // Default: 300 (5 min)

	// Category allocations
	VolatilityAllocations map[VolatilityClass]*CategoryAllocation `json:"volatility_allocations"`
	MarketCapAllocations  map[MarketCapClass]*CategoryAllocation  `json:"market_cap_allocations"`
	MomentumAllocations   map[MomentumClass]*CategoryAllocation   `json:"momentum_allocations"`

	// Per-coin preferences
	CoinPreferences       map[string]*CoinPreference             `json:"coin_preferences,omitempty"`
}

// NewDefaultCoinClassificationSettings creates settings with sensible defaults
func NewDefaultCoinClassificationSettings() *CoinClassificationSettings {
	return &CoinClassificationSettings{
		// Volatility thresholds
		VolatilityStableMax: 3.0,
		VolatilityMediumMax: 6.0,

		// Momentum thresholds
		MomentumGainerMin: 5.0,
		MomentumLoserMax:  -5.0,

		// Minimum requirements
		MinVolume24h: 1000000, // 1M USDT minimum

		// ATR settings
		ATRPeriod:    14,
		ATRTimeframe: "1d",

		// Refresh interval
		RefreshIntervalSecs: 300, // 5 minutes

		// Default volatility allocations
		VolatilityAllocations: map[VolatilityClass]*CategoryAllocation{
			VolatilityStable: {
				Enabled:           true,
				AllocationPercent: 50.0,
				MaxPositions:      5,
			},
			VolatilityMedium: {
				Enabled:           true,
				AllocationPercent: 35.0,
				MaxPositions:      3,
			},
			VolatilityHigh: {
				Enabled:           true,
				AllocationPercent: 15.0,
				MaxPositions:      2,
			},
		},

		// Default market cap allocations
		MarketCapAllocations: map[MarketCapClass]*CategoryAllocation{
			MarketCapBlueChip: {
				Enabled:           true,
				AllocationPercent: 50.0,
				MaxPositions:      2,
			},
			MarketCapLarge: {
				Enabled:           true,
				AllocationPercent: 35.0,
				MaxPositions:      4,
			},
			MarketCapMidSmall: {
				Enabled:           true,
				AllocationPercent: 15.0,
				MaxPositions:      3,
			},
		},

		// Default momentum allocations
		MomentumAllocations: map[MomentumClass]*CategoryAllocation{
			MomentumGainer: {
				Enabled:           true,
				AllocationPercent: 40.0,
				MaxPositions:      3,
			},
			MomentumNeutral: {
				Enabled:           true,
				AllocationPercent: 50.0,
				MaxPositions:      4,
			},
			MomentumLoser: {
				Enabled:           false, // Disabled by default
				AllocationPercent: 10.0,
				MaxPositions:      1,
			},
		},

		// Empty coin preferences (defaults to enabled)
		CoinPreferences: make(map[string]*CoinPreference),
	}
}

// ==================== MARKET CAP LISTS ====================

// BlueChipSymbols are the top tier coins (BTC, ETH)
var BlueChipSymbols = []string{"BTCUSDT", "ETHUSDT"}

// LargeCapSymbols are established large cap altcoins
var LargeCapSymbols = []string{
	"BNBUSDT", "SOLUSDT", "XRPUSDT", "ADAUSDT", "AVAXUSDT",
	"DOTUSDT", "LINKUSDT", "LTCUSDT", "ATOMUSDT", "UNIUSDT",
	"NEARUSDT", "APTUSDT", "ARBUSDT", "OPUSDT", "ICPUSDT",
}

// GetMarketCapClass returns the market cap class for a symbol
func GetMarketCapClass(symbol string) MarketCapClass {
	for _, s := range BlueChipSymbols {
		if s == symbol {
			return MarketCapBlueChip
		}
	}
	for _, s := range LargeCapSymbols {
		if s == symbol {
			return MarketCapLarge
		}
	}
	return MarketCapMidSmall
}

// ==================== HELPER METHODS ====================

// GetVolatilityClass determines volatility class from ATR percentage
func GetVolatilityClass(atrPercent float64, settings *CoinClassificationSettings) VolatilityClass {
	if atrPercent < settings.VolatilityStableMax {
		return VolatilityStable
	} else if atrPercent < settings.VolatilityMediumMax {
		return VolatilityMedium
	}
	return VolatilityHigh
}

// GetMomentumClass determines momentum class from 24h change
func GetMomentumClass(change24h float64, settings *CoinClassificationSettings) MomentumClass {
	if change24h >= settings.MomentumGainerMin {
		return MomentumGainer
	} else if change24h <= settings.MomentumLoserMax {
		return MomentumLoser
	}
	return MomentumNeutral
}

// CalculateRiskScore computes a combined risk score (0-1)
// Higher = more risky
func CalculateRiskScore(c *CoinClassification) float64 {
	score := 0.0

	// Volatility component (40% weight)
	switch c.Volatility {
	case VolatilityStable:
		score += 0.1
	case VolatilityMedium:
		score += 0.25
	case VolatilityHigh:
		score += 0.4
	}

	// Market cap component (30% weight)
	switch c.MarketCap {
	case MarketCapBlueChip:
		score += 0.05
	case MarketCapLarge:
		score += 0.15
	case MarketCapMidSmall:
		score += 0.3
	}

	// Momentum component (30% weight)
	switch c.Momentum {
	case MomentumLoser:
		score += 0.25 // Losers are risky
	case MomentumNeutral:
		score += 0.1
	case MomentumGainer:
		score += 0.2 // Gainers can be risky (extended)
	}

	return score
}

// CalculateOpportunityScore computes potential opportunity (0-1)
// Higher = more opportunity
func CalculateOpportunityScore(c *CoinClassification) float64 {
	score := 0.0

	// Volatility component (40% weight) - higher volatility = more opportunity
	switch c.Volatility {
	case VolatilityStable:
		score += 0.15
	case VolatilityMedium:
		score += 0.3
	case VolatilityHigh:
		score += 0.4
	}

	// Momentum component (40% weight)
	switch c.Momentum {
	case MomentumLoser:
		score += 0.15 // Potential bounce
	case MomentumNeutral:
		score += 0.2
	case MomentumGainer:
		score += 0.35 // Trend continuation
	}

	// Volume boost (20% weight)
	if c.QuoteVolume24h > 100000000 { // > 100M volume
		score += 0.2
	} else if c.QuoteVolume24h > 10000000 { // > 10M volume
		score += 0.15
	} else {
		score += 0.1
	}

	return score
}

// IsEligible checks if a coin passes all allocation filters
func (c *CoinClassification) IsEligible(settings *CoinClassificationSettings) bool {
	// Check coin preference
	if pref, exists := settings.CoinPreferences[c.Symbol]; exists {
		if !pref.Enabled {
			return false
		}
	}

	// Check volatility allocation
	if alloc, exists := settings.VolatilityAllocations[c.Volatility]; exists {
		if !alloc.Enabled {
			return false
		}
	}

	// Check market cap allocation
	if alloc, exists := settings.MarketCapAllocations[c.MarketCap]; exists {
		if !alloc.Enabled {
			return false
		}
	}

	// Check momentum allocation
	if alloc, exists := settings.MomentumAllocations[c.Momentum]; exists {
		if !alloc.Enabled {
			return false
		}
	}

	// Check minimum volume
	if c.QuoteVolume24h < settings.MinVolume24h {
		return false
	}

	return true
}

// ==================== CLASSIFICATION SUMMARY ====================

// ClassificationSummary provides aggregated view of all classifications
type ClassificationSummary struct {
	TotalSymbols     int                                    `json:"total_symbols"`
	EnabledSymbols   int                                    `json:"enabled_symbols"`

	ByVolatility     map[VolatilityClass][]string          `json:"by_volatility"`
	ByMarketCap      map[MarketCapClass][]string           `json:"by_market_cap"`
	ByMomentum       map[MomentumClass][]string            `json:"by_momentum"`

	TopGainers       []CoinClassification                  `json:"top_gainers"`
	TopLosers        []CoinClassification                  `json:"top_losers"`
	TopVolume        []CoinClassification                  `json:"top_volume"`

	LastUpdated      time.Time                             `json:"last_updated"`
}
