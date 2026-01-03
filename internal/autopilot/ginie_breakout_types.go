package autopilot

import "time"

// BreakoutType represents the type of breakout detected
type BreakoutType string

const (
	BreakoutTypePrice24hHigh    BreakoutType = "price_24h_high"
	BreakoutTypePrice24hLow     BreakoutType = "price_24h_low"
	BreakoutTypePriceResistance BreakoutType = "price_resistance"
	BreakoutTypePriceSupport    BreakoutType = "price_support"
	BreakoutTypeVolumeSpike     BreakoutType = "volume_spike"
	BreakoutTypeMomentumAccel   BreakoutType = "momentum_acceleration"
	BreakoutTypeOrderBookImbal  BreakoutType = "order_book_imbalance"
	BreakoutTypeMultiSignal     BreakoutType = "multi_signal_confluence"
)

// BreakoutSignal represents a single breakout indicator signal
type BreakoutSignal struct {
	Type           BreakoutType `json:"type"`
	Direction      string       `json:"direction"`       // "LONG" or "SHORT"
	Strength       float64      `json:"strength"`        // 0-100
	CurrentValue   float64      `json:"current_value"`   // Current metric value
	ThresholdValue float64      `json:"threshold_value"` // Threshold that was exceeded
	Description    string       `json:"description"`
	DetectedAt     time.Time    `json:"detected_at"`
	Confidence     float64      `json:"confidence"` // 0-100
	Weight         float64      `json:"weight"`     // Signal weight in aggregation
}

// BreakoutAnalysis contains all breakout detection results
type BreakoutAnalysis struct {
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`

	// Individual signal categories
	VolumeBreakout    *VolumeBreakout    `json:"volume_breakout"`
	PriceBreakout     *PriceBreakout     `json:"price_breakout"`
	MomentumBreakout  *MomentumBreakout  `json:"momentum_breakout"`
	OrderBookBreakout *OrderBookBreakout `json:"order_book_breakout,omitempty"`

	// Aggregated signals
	Signals []BreakoutSignal `json:"signals"`

	// Overall assessment
	BreakoutDetected  bool    `json:"breakout_detected"`
	BreakoutDirection string  `json:"breakout_direction"` // "LONG", "SHORT", "NEUTRAL"
	BreakoutScore     float64 `json:"breakout_score"`     // 0-100 composite score
	BreakoutStrength  string  `json:"breakout_strength"`  // "weak", "moderate", "strong", "very_strong"
	Confluence        int     `json:"confluence"`         // Number of aligned signals

	// Trade recommendations
	SuggestedEntry float64 `json:"suggested_entry"`
	SuggestedSL    float64 `json:"suggested_sl"`
	SuggestedTP    float64 `json:"suggested_tp"`
	RiskReward     float64 `json:"risk_reward"`
}

// VolumeBreakout contains volume-based breakout signals
type VolumeBreakout struct {
	// Volume spike detection
	CurrentVolume   float64 `json:"current_volume"`    // Current candle volume
	AverageVolume20 float64 `json:"average_volume_20"` // 20-period average
	VolumeRatio     float64 `json:"volume_ratio"`      // Current / Average
	IsSpiking       bool    `json:"is_spiking"`        // Ratio > 2.0
	SpikeMultiplier float64 `json:"spike_multiplier"`  // How many times above average

	// Volume profile
	VolumeProfile   []VolumeLevel `json:"volume_profile"`
	HighVolumeNodes []float64     `json:"high_volume_nodes"` // Price levels with high volume
	LowVolumeNodes  []float64     `json:"low_volume_nodes"`  // Price levels with low volume (breakout candidates)

	// Volume-price divergence
	PriceTrend     string `json:"price_trend"`     // "up", "down", "flat"
	VolumeTrend    string `json:"volume_trend"`    // "up", "down", "flat"
	HasDivergence  bool   `json:"has_divergence"`  // Price up + volume down = bearish divergence
	DivergenceType string `json:"divergence_type"` // "bullish", "bearish", "none"

	// Cumulative volume delta
	CumulativeDelta float64 `json:"cumulative_delta"` // Buying pressure - selling pressure
	DeltaTrend      string  `json:"delta_trend"`      // "accumulation", "distribution", "neutral"

	// Scoring
	VolumeScore    float64 `json:"volume_score"`    // 0-100
	SignalStrength string  `json:"signal_strength"` // "weak", "moderate", "strong"
}

// VolumeLevel represents a price level in the volume profile
type VolumeLevel struct {
	PriceLow   float64 `json:"price_low"`
	PriceHigh  float64 `json:"price_high"`
	Volume     float64 `json:"volume"`
	Percentage float64 `json:"percentage"` // % of total volume
}

// PriceBreakout contains price action breakout signals
type PriceBreakout struct {
	// 24h price levels
	High24h        float64 `json:"high_24h"`
	Low24h         float64 `json:"low_24h"`
	CurrentPrice   float64 `json:"current_price"`
	DistanceToHigh float64 `json:"distance_to_high"` // % from current to 24h high
	DistanceToLow  float64 `json:"distance_to_low"`  // % from current to 24h low

	// Key level breaks
	Breaking24hHigh bool    `json:"breaking_24h_high"` // Price approaching/breaking 24h high
	Breaking24hLow  bool    `json:"breaking_24h_low"`  // Price approaching/breaking 24h low
	NearHighPercent float64 `json:"near_high_percent"` // How close to 24h high (0-1)

	// Resistance/Support levels
	KeyResistances     []float64 `json:"key_resistances"`
	KeySupports        []float64 `json:"key_supports"`
	NearestResistance  float64   `json:"nearest_resistance"`
	NearestSupport     float64   `json:"nearest_support"`
	BreakingResistance bool      `json:"breaking_resistance"`
	BreakingSupport    bool      `json:"breaking_support"`

	// Candle pattern acceleration
	AvgCandleBodySize  float64 `json:"avg_candle_body_size"`  // Average body size (20 candles)
	LastCandleBodySize float64 `json:"last_candle_body_size"`
	BodySizeRatio      float64 `json:"body_size_ratio"` // Last / Average
	IsAccelerating     bool    `json:"is_accelerating"` // Ratio > 1.5
	ConsecutiveDir     int     `json:"consecutive_dir"` // Consecutive candles in same direction

	// Range contraction before breakout
	RangeContraction float64 `json:"range_contraction"` // Recent ATR / Historical ATR
	IsContracting    bool    `json:"is_contracting"`    // Range getting tighter

	// Scoring
	PriceScore     float64 `json:"price_score"` // 0-100
	SignalStrength string  `json:"signal_strength"`
}

// MomentumBreakout contains momentum-based breakout signals
type MomentumBreakout struct {
	// Rate of Change (ROC)
	ROC10           float64 `json:"roc_10"`           // 10-period ROC
	ROC5            float64 `json:"roc_5"`            // 5-period ROC (faster)
	ROCAccelerating bool    `json:"roc_accelerating"` // ROC5 > ROC10 (momentum building)
	ROCDirection    string  `json:"roc_direction"`    // "up", "down", "flat"

	// Price acceleration (second derivative)
	PriceVelocity     float64 `json:"price_velocity"`     // First derivative (rate of change)
	PriceAcceleration float64 `json:"price_acceleration"` // Second derivative (change of change)
	IsAccelerating    bool    `json:"is_accelerating"`    // Positive second derivative
	AccelerationPhase string  `json:"acceleration_phase"` // "accelerating", "decelerating", "constant"

	// Multi-timeframe momentum alignment
	Momentum1m   float64 `json:"momentum_1m"`
	Momentum5m   float64 `json:"momentum_5m"`
	Momentum15m  float64 `json:"momentum_15m"`
	Momentum1h   float64 `json:"momentum_1h"`
	MTFAligned   bool    `json:"mtf_aligned"`   // All timeframes pointing same direction
	MTFConsensus int     `json:"mtf_consensus"` // Count of aligned timeframes
	MTFDirection string  `json:"mtf_direction"` // Dominant direction

	// RSI momentum (not for overbought/oversold - for momentum shift)
	RSI14         float64 `json:"rsi_14"`
	RSI7          float64 `json:"rsi_7"`
	RSICrossing50 bool    `json:"rsi_crossing_50"` // RSI crossing middle line
	RSIDirection  string  `json:"rsi_direction"`   // Direction of cross

	// MACD momentum
	MACDHistogram     float64 `json:"macd_histogram"`
	MACDHistogramPrev float64 `json:"macd_histogram_prev"`
	MACDExpanding     bool    `json:"macd_expanding"` // Histogram growing

	// Scoring
	MomentumScore  float64 `json:"momentum_score"` // 0-100
	SignalStrength string  `json:"signal_strength"`
}

// OrderBookBreakout contains order book-based breakout signals
type OrderBookBreakout struct {
	// Bid/Ask imbalance
	TotalBidVolume     float64 `json:"total_bid_volume"`
	TotalAskVolume     float64 `json:"total_ask_volume"`
	BidAskRatio        float64 `json:"bid_ask_ratio"`        // Bids / Asks
	ImbalancePercent   float64 `json:"imbalance_percent"`    // (Bids - Asks) / Total
	ImbalanceDirection string  `json:"imbalance_direction"`  // "buy", "sell", "neutral"

	// Large order clustering
	LargeBidClusters []OrderCluster `json:"large_bid_clusters"`
	LargeAskClusters []OrderCluster `json:"large_ask_clusters"`
	BidWallDetected  bool           `json:"bid_wall_detected"`
	AskWallDetected  bool           `json:"ask_wall_detected"`
	NearestBidWall   float64        `json:"nearest_bid_wall"`
	NearestAskWall   float64        `json:"nearest_ask_wall"`

	// Spread analysis
	CurrentSpread     float64 `json:"current_spread"`
	AverageSpread     float64 `json:"average_spread"`
	SpreadContraction bool    `json:"spread_contraction"` // Tightening spread = breakout imminent

	// Depth analysis
	DepthImbalanceAt1 float64 `json:"depth_imbalance_at_1"` // Imbalance within 1%
	DepthImbalanceAt2 float64 `json:"depth_imbalance_at_2"` // Imbalance within 2%

	// Scoring
	OrderBookScore float64 `json:"order_book_score"` // 0-100
	SignalStrength string  `json:"signal_strength"`

	// Availability
	DataAvailable bool `json:"data_available"` // Whether order book data was fetched
}

// OrderCluster represents a cluster of large orders
type OrderCluster struct {
	PriceLevel    float64 `json:"price_level"`
	TotalVolume   float64 `json:"total_volume"`
	OrderCount    int     `json:"order_count"`
	IsSignificant bool    `json:"is_significant"` // > 2x average level volume
}

// BreakoutConfig contains configuration for breakout detection
type BreakoutConfig struct {
	// Enable/disable breakout detection
	Enabled bool `json:"enabled"`

	// Volume thresholds
	VolumeSpikeMultiplier float64 `json:"volume_spike_multiplier"` // Default: 2.0
	VolumeProfilePeriod   int     `json:"volume_profile_period"`   // Default: 24

	// Price thresholds
	Near24hHighPercent        float64 `json:"near_24h_high_percent"`        // Default: 0.5 (0.5% from high)
	CandleAccelerationRatio   float64 `json:"candle_acceleration_ratio"`    // Default: 1.5
	RangeContractionThreshold float64 `json:"range_contraction_threshold"`  // Default: 0.7

	// Momentum thresholds
	ROCAccelerationThreshold float64 `json:"roc_acceleration_threshold"` // Default: 0.5
	MTFConsensusRequired     int     `json:"mtf_consensus_required"`     // Default: 3

	// Order book thresholds
	BidAskImbalanceThreshold float64 `json:"bid_ask_imbalance_threshold"` // Default: 1.5 (150%)
	LargeOrderMultiplier     float64 `json:"large_order_multiplier"`      // Default: 5.0

	// Scoring weights
	VolumeWeight    float64 `json:"volume_weight"`     // Default: 0.30
	PriceWeight     float64 `json:"price_weight"`      // Default: 0.25
	MomentumWeight  float64 `json:"momentum_weight"`   // Default: 0.25
	OrderBookWeight float64 `json:"order_book_weight"` // Default: 0.20

	// Confluence requirements
	MinSignalsForBreakout int     `json:"min_signals_for_breakout"` // Default: 2
	MinBreakoutScore      float64 `json:"min_breakout_score"`       // Default: 60
}

// DefaultBreakoutConfig returns the default breakout detection configuration
func DefaultBreakoutConfig() *BreakoutConfig {
	return &BreakoutConfig{
		Enabled:                   true,
		VolumeSpikeMultiplier:     2.0,
		VolumeProfilePeriod:       24,
		Near24hHighPercent:        0.5,
		CandleAccelerationRatio:   1.5,
		RangeContractionThreshold: 0.7,
		ROCAccelerationThreshold:  0.5,
		MTFConsensusRequired:      3,
		BidAskImbalanceThreshold:  1.5,
		LargeOrderMultiplier:      5.0,
		VolumeWeight:              0.30,
		PriceWeight:               0.25,
		MomentumWeight:            0.25,
		OrderBookWeight:           0.20,
		MinSignalsForBreakout:     2,
		MinBreakoutScore:          60,
	}
}
