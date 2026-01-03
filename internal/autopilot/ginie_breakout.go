package autopilot

import (
	"fmt"
	"math"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/logging"
)

// BreakoutDetector handles all breakout detection logic for catching rallies early
type BreakoutDetector struct {
	futuresClient binance.FuturesClient
	config        *BreakoutConfig
	logger        *logging.Logger
}

// NewBreakoutDetector creates a new breakout detector
func NewBreakoutDetector(client binance.FuturesClient, config *BreakoutConfig, logger *logging.Logger) *BreakoutDetector {
	if config == nil {
		config = DefaultBreakoutConfig()
	}
	return &BreakoutDetector{
		futuresClient: client,
		config:        config,
		logger:        logger,
	}
}

// AnalyzeBreakout performs complete breakout analysis for a symbol
func (bd *BreakoutDetector) AnalyzeBreakout(symbol string, klines []binance.Kline, ticker *binance.Futures24hrTicker) (*BreakoutAnalysis, error) {
	if !bd.config.Enabled {
		return nil, nil
	}

	if len(klines) < 30 {
		return nil, fmt.Errorf("insufficient klines for breakout analysis: need 30, got %d", len(klines))
	}

	// Detect individual breakout signals
	volumeBreakout := bd.DetectVolumeBreakout(klines)
	priceBreakout := bd.DetectPriceBreakout(klines, ticker)
	momentumBreakout := bd.DetectMomentumBreakout(klines)

	// Order book analysis is optional (requires additional API call)
	var orderBookBreakout *OrderBookBreakout
	// Skip order book for now to reduce API calls - can be enabled later
	// orderBookBreakout = bd.DetectOrderBookBreakout(symbol)

	// Get current price
	currentPrice := klines[len(klines)-1].Close

	// Aggregate all signals
	analysis := bd.AggregateBreakoutSignals(symbol, volumeBreakout, priceBreakout, momentumBreakout, orderBookBreakout, currentPrice)

	if bd.logger != nil && analysis.BreakoutDetected {
		bd.logger.Info("[BREAKOUT] Detected",
			"symbol", symbol,
			"direction", analysis.BreakoutDirection,
			"score", fmt.Sprintf("%.1f", analysis.BreakoutScore),
			"confluence", analysis.Confluence,
			"strength", analysis.BreakoutStrength)
	}

	return analysis, nil
}

// DetectVolumeBreakout analyzes volume for breakout signals
func (bd *BreakoutDetector) DetectVolumeBreakout(klines []binance.Kline) *VolumeBreakout {
	vb := &VolumeBreakout{}

	if len(klines) < 21 {
		return vb
	}

	// 1. Volume spike detection
	currentVol := klines[len(klines)-1].Volume
	avgVol := bd.calculateAverageVolume(klines, 20)
	vb.CurrentVolume = currentVol
	vb.AverageVolume20 = avgVol

	if avgVol > 0 {
		vb.VolumeRatio = currentVol / avgVol
	}
	vb.IsSpiking = vb.VolumeRatio >= bd.config.VolumeSpikeMultiplier
	vb.SpikeMultiplier = vb.VolumeRatio

	// 2. Volume profile (simplified - bucket by price levels)
	vb.VolumeProfile = bd.calculateVolumeProfile(klines, 10)
	vb.HighVolumeNodes, vb.LowVolumeNodes = bd.findVolumeNodes(vb.VolumeProfile)

	// 3. Volume-price divergence
	priceChange := klines[len(klines)-1].Close - klines[len(klines)-6].Close
	volumeChange := currentVol - klines[len(klines)-6].Volume

	if priceChange > 0 {
		vb.PriceTrend = "up"
	} else if priceChange < 0 {
		vb.PriceTrend = "down"
	} else {
		vb.PriceTrend = "flat"
	}

	if volumeChange > 0 {
		vb.VolumeTrend = "up"
	} else if volumeChange < 0 {
		vb.VolumeTrend = "down"
	} else {
		vb.VolumeTrend = "flat"
	}

	// Bullish divergence: price down + volume up (accumulation)
	// Bearish divergence: price up + volume down (distribution)
	if vb.PriceTrend == "down" && vb.VolumeTrend == "up" {
		vb.HasDivergence = true
		vb.DivergenceType = "bullish" // Accumulation
	} else if vb.PriceTrend == "up" && vb.VolumeTrend == "down" {
		vb.HasDivergence = true
		vb.DivergenceType = "bearish" // Distribution
	} else {
		vb.DivergenceType = "none"
	}

	// 4. Cumulative volume delta (buying vs selling pressure)
	vb.CumulativeDelta = bd.calculateCumulativeDelta(klines, 20)
	if vb.CumulativeDelta > 0.1 {
		vb.DeltaTrend = "accumulation"
	} else if vb.CumulativeDelta < -0.1 {
		vb.DeltaTrend = "distribution"
	} else {
		vb.DeltaTrend = "neutral"
	}

	// 5. Calculate volume score
	vb.VolumeScore = bd.calculateVolumeScore(vb)
	vb.SignalStrength = bd.classifyStrength(vb.VolumeScore)

	return vb
}

// DetectPriceBreakout analyzes price action for breakout signals
func (bd *BreakoutDetector) DetectPriceBreakout(klines []binance.Kline, ticker *binance.Futures24hrTicker) *PriceBreakout {
	pb := &PriceBreakout{}

	if len(klines) < 21 {
		return pb
	}

	currentPrice := klines[len(klines)-1].Close
	pb.CurrentPrice = currentPrice

	// 1. 24h price level analysis
	if ticker != nil {
		pb.High24h = ticker.HighPrice
		pb.Low24h = ticker.LowPrice

		if pb.High24h > 0 {
			pb.DistanceToHigh = ((pb.High24h - currentPrice) / currentPrice) * 100
			pb.NearHighPercent = 1 - (pb.DistanceToHigh / 5) // Normalize to 0-1 for 5% range
			if pb.NearHighPercent < 0 {
				pb.NearHighPercent = 0
			}
			if pb.NearHighPercent > 1 {
				pb.NearHighPercent = 1
			}
			pb.Breaking24hHigh = pb.DistanceToHigh <= bd.config.Near24hHighPercent
		}

		if pb.Low24h > 0 {
			pb.DistanceToLow = ((currentPrice - pb.Low24h) / currentPrice) * 100
			pb.Breaking24hLow = pb.DistanceToLow <= bd.config.Near24hHighPercent
		}
	}

	// 2. Find key resistance/support from recent price action
	pb.KeyResistances, pb.KeySupports = bd.findKeyLevels(klines)
	if len(pb.KeyResistances) > 0 {
		pb.NearestResistance = pb.KeyResistances[0]
		distToRes := ((pb.NearestResistance - currentPrice) / currentPrice) * 100
		pb.BreakingResistance = distToRes <= 0.5 || currentPrice >= pb.NearestResistance
	}
	if len(pb.KeySupports) > 0 {
		pb.NearestSupport = pb.KeySupports[0]
		distToSup := ((currentPrice - pb.NearestSupport) / currentPrice) * 100
		pb.BreakingSupport = distToSup <= 0.5 || currentPrice <= pb.NearestSupport
	}

	// 3. Candle pattern acceleration
	avgBodySize := 0.0
	for i := len(klines) - 21; i < len(klines)-1; i++ {
		avgBodySize += math.Abs(klines[i].Close - klines[i].Open)
	}
	avgBodySize /= 20.0

	lastCandle := klines[len(klines)-1]
	pb.AvgCandleBodySize = avgBodySize
	pb.LastCandleBodySize = math.Abs(lastCandle.Close - lastCandle.Open)

	if avgBodySize > 0 {
		pb.BodySizeRatio = pb.LastCandleBodySize / avgBodySize
		pb.IsAccelerating = pb.BodySizeRatio >= bd.config.CandleAccelerationRatio
	}

	// Count consecutive candles in same direction
	pb.ConsecutiveDir = bd.countConsecutiveDirection(klines, 10)

	// 4. Range contraction detection
	recentATR := bd.calculateATRSimple(klines[len(klines)-14:], 14)
	historicalATR := bd.calculateATRSimple(klines[:len(klines)-14], 14)
	if historicalATR > 0 {
		pb.RangeContraction = recentATR / historicalATR
		pb.IsContracting = pb.RangeContraction < bd.config.RangeContractionThreshold
	}

	// 5. Calculate price score
	pb.PriceScore = bd.calculatePriceScore(pb)
	pb.SignalStrength = bd.classifyStrength(pb.PriceScore)

	return pb
}

// DetectMomentumBreakout analyzes momentum indicators for breakout signals
func (bd *BreakoutDetector) DetectMomentumBreakout(klines []binance.Kline) *MomentumBreakout {
	mb := &MomentumBreakout{}

	if len(klines) < 20 {
		return mb
	}

	// 1. Rate of Change (ROC)
	mb.ROC10 = bd.calculateROC(klines, 10)
	mb.ROC5 = bd.calculateROC(klines, 5)
	mb.ROCAccelerating = mb.ROC5 > mb.ROC10 && mb.ROC5 > 0

	if mb.ROC5 > bd.config.ROCAccelerationThreshold {
		mb.ROCDirection = "up"
	} else if mb.ROC5 < -bd.config.ROCAccelerationThreshold {
		mb.ROCDirection = "down"
	} else {
		mb.ROCDirection = "flat"
	}

	// 2. Price acceleration (second derivative)
	mb.PriceVelocity = bd.calculateVelocity(klines, 5)
	mb.PriceAcceleration = bd.calculateAcceleration(klines, 5)
	mb.IsAccelerating = mb.PriceAcceleration > 0

	if mb.PriceAcceleration > 0.0001 {
		mb.AccelerationPhase = "accelerating"
	} else if mb.PriceAcceleration < -0.0001 {
		mb.AccelerationPhase = "decelerating"
	} else {
		mb.AccelerationPhase = "constant"
	}

	// 3. Simplified momentum (using same klines for now)
	mb.Momentum1m = bd.calculateMomentum(klines, 14)
	mb.Momentum5m = mb.Momentum1m // Same as we only have one timeframe
	mb.Momentum15m = mb.Momentum1m
	mb.Momentum1h = mb.Momentum1m

	// Count aligned timeframes (simplified - all point same direction)
	mb.MTFConsensus = 0
	longCount := 0
	shortCount := 0

	momentums := []float64{mb.Momentum1m, mb.Momentum5m, mb.Momentum15m, mb.Momentum1h}
	for _, m := range momentums {
		if m > 0 {
			longCount++
		} else if m < 0 {
			shortCount++
		}
	}

	mb.MTFConsensus = max(longCount, shortCount)
	mb.MTFAligned = mb.MTFConsensus >= bd.config.MTFConsensusRequired

	if longCount > shortCount {
		mb.MTFDirection = "LONG"
	} else if shortCount > longCount {
		mb.MTFDirection = "SHORT"
	} else {
		mb.MTFDirection = "NEUTRAL"
	}

	// 4. RSI momentum (crossing 50 line)
	mb.RSI14 = bd.calculateRSI(klines, 14)
	mb.RSI7 = bd.calculateRSI(klines, 7)

	prevRSI := bd.calculateRSIWithOffset(klines, 14, 1)
	if mb.RSI14 > 50 && prevRSI <= 50 {
		mb.RSICrossing50 = true
		mb.RSIDirection = "up"
	} else if mb.RSI14 < 50 && prevRSI >= 50 {
		mb.RSICrossing50 = true
		mb.RSIDirection = "down"
	}

	// 5. MACD momentum (simplified)
	mb.MACDHistogram = bd.calculateMACDHistogram(klines)
	mb.MACDHistogramPrev = bd.calculateMACDHistogramWithOffset(klines, 1)
	mb.MACDExpanding = math.Abs(mb.MACDHistogram) > math.Abs(mb.MACDHistogramPrev) &&
		(mb.MACDHistogram > 0) == (mb.MACDHistogramPrev > 0)

	// 6. Calculate momentum score
	mb.MomentumScore = bd.calculateMomentumScore(mb)
	mb.SignalStrength = bd.classifyStrength(mb.MomentumScore)

	return mb
}

// AggregateBreakoutSignals combines all signals into final analysis
func (bd *BreakoutDetector) AggregateBreakoutSignals(
	symbol string,
	volume *VolumeBreakout,
	price *PriceBreakout,
	momentum *MomentumBreakout,
	orderBook *OrderBookBreakout,
	currentPrice float64,
) *BreakoutAnalysis {
	analysis := &BreakoutAnalysis{
		Symbol:            symbol,
		Timestamp:         time.Now(),
		VolumeBreakout:    volume,
		PriceBreakout:     price,
		MomentumBreakout:  momentum,
		OrderBookBreakout: orderBook,
		Signals:           []BreakoutSignal{},
	}

	confluence := 0
	longSignals := 0
	shortSignals := 0

	// Volume signals
	if volume != nil && volume.IsSpiking {
		sig := BreakoutSignal{
			Type:           BreakoutTypeVolumeSpike,
			Strength:       volume.VolumeScore,
			CurrentValue:   volume.VolumeRatio,
			ThresholdValue: bd.config.VolumeSpikeMultiplier,
			Description:    fmt.Sprintf("Volume spike: %.1fx average", volume.SpikeMultiplier),
			DetectedAt:     time.Now(),
			Confidence:     volume.VolumeScore,
			Weight:         bd.config.VolumeWeight,
		}

		if volume.DeltaTrend == "accumulation" {
			sig.Direction = "LONG"
			longSignals++
		} else if volume.DeltaTrend == "distribution" {
			sig.Direction = "SHORT"
			shortSignals++
		} else {
			sig.Direction = "NEUTRAL"
		}

		analysis.Signals = append(analysis.Signals, sig)
		confluence++
	}

	// Price signals - Breaking 24h high
	if price != nil && price.Breaking24hHigh {
		analysis.Signals = append(analysis.Signals, BreakoutSignal{
			Type:           BreakoutTypePrice24hHigh,
			Direction:      "LONG",
			Strength:       price.PriceScore,
			CurrentValue:   price.CurrentPrice,
			ThresholdValue: price.High24h,
			Description:    fmt.Sprintf("Breaking 24h high: %.4f", price.High24h),
			DetectedAt:     time.Now(),
			Confidence:     price.PriceScore,
			Weight:         bd.config.PriceWeight,
		})
		longSignals++
		confluence++
	}

	// Price signals - Breaking 24h low
	if price != nil && price.Breaking24hLow {
		analysis.Signals = append(analysis.Signals, BreakoutSignal{
			Type:           BreakoutTypePrice24hLow,
			Direction:      "SHORT",
			Strength:       price.PriceScore,
			CurrentValue:   price.CurrentPrice,
			ThresholdValue: price.Low24h,
			Description:    fmt.Sprintf("Breaking 24h low: %.4f", price.Low24h),
			DetectedAt:     time.Now(),
			Confidence:     price.PriceScore,
			Weight:         bd.config.PriceWeight,
		})
		shortSignals++
		confluence++
	}

	// Price signals - Breaking resistance
	if price != nil && price.BreakingResistance && !price.Breaking24hHigh {
		analysis.Signals = append(analysis.Signals, BreakoutSignal{
			Type:           BreakoutTypePriceResistance,
			Direction:      "LONG",
			Strength:       price.PriceScore,
			CurrentValue:   price.CurrentPrice,
			ThresholdValue: price.NearestResistance,
			Description:    fmt.Sprintf("Breaking resistance: %.4f", price.NearestResistance),
			DetectedAt:     time.Now(),
			Confidence:     price.PriceScore,
			Weight:         bd.config.PriceWeight,
		})
		longSignals++
		confluence++
	}

	// Momentum signals
	if momentum != nil && momentum.MTFAligned && momentum.IsAccelerating {
		analysis.Signals = append(analysis.Signals, BreakoutSignal{
			Type:           BreakoutTypeMomentumAccel,
			Direction:      momentum.MTFDirection,
			Strength:       momentum.MomentumScore,
			CurrentValue:   momentum.PriceAcceleration,
			ThresholdValue: 0,
			Description:    fmt.Sprintf("MTF momentum aligned: %d/4 TFs, %s", momentum.MTFConsensus, momentum.AccelerationPhase),
			DetectedAt:     time.Now(),
			Confidence:     momentum.MomentumScore,
			Weight:         bd.config.MomentumWeight,
		})

		if momentum.MTFDirection == "LONG" {
			longSignals++
		} else if momentum.MTFDirection == "SHORT" {
			shortSignals++
		}
		confluence++
	}

	// Order book signals (if available)
	if orderBook != nil && orderBook.DataAvailable && orderBook.ImbalanceDirection != "neutral" {
		direction := "LONG"
		if orderBook.ImbalanceDirection == "sell" {
			direction = "SHORT"
		}

		analysis.Signals = append(analysis.Signals, BreakoutSignal{
			Type:           BreakoutTypeOrderBookImbal,
			Direction:      direction,
			Strength:       orderBook.OrderBookScore,
			CurrentValue:   orderBook.BidAskRatio,
			ThresholdValue: bd.config.BidAskImbalanceThreshold,
			Description:    fmt.Sprintf("Order book imbalance: %.1f%% %s", math.Abs(orderBook.ImbalancePercent), orderBook.ImbalanceDirection),
			DetectedAt:     time.Now(),
			Confidence:     orderBook.OrderBookScore,
			Weight:         bd.config.OrderBookWeight,
		})

		if orderBook.ImbalanceDirection == "buy" {
			longSignals++
		} else {
			shortSignals++
		}
		confluence++
	}

	// Calculate composite score
	totalWeight := 0.0
	weightedScore := 0.0

	for _, sig := range analysis.Signals {
		weightedScore += sig.Confidence * sig.Weight
		totalWeight += sig.Weight
	}

	if totalWeight > 0 {
		analysis.BreakoutScore = weightedScore / totalWeight
	}

	// Determine overall direction
	if longSignals > shortSignals {
		analysis.BreakoutDirection = "LONG"
	} else if shortSignals > longSignals {
		analysis.BreakoutDirection = "SHORT"
	} else {
		analysis.BreakoutDirection = "NEUTRAL"
	}

	// Set confluence and detection status
	analysis.Confluence = confluence
	analysis.BreakoutDetected = confluence >= bd.config.MinSignalsForBreakout &&
		analysis.BreakoutScore >= bd.config.MinBreakoutScore

	// Classify strength
	if analysis.BreakoutScore >= 80 {
		analysis.BreakoutStrength = "very_strong"
	} else if analysis.BreakoutScore >= 60 {
		analysis.BreakoutStrength = "strong"
	} else if analysis.BreakoutScore >= 40 {
		analysis.BreakoutStrength = "moderate"
	} else {
		analysis.BreakoutStrength = "weak"
	}

	// Calculate trade recommendations
	if analysis.BreakoutDetected && analysis.BreakoutDirection != "NEUTRAL" {
		analysis.SuggestedEntry = currentPrice
		atrPercent := 1.5 // Default ATR-based SL/TP

		if analysis.BreakoutDirection == "LONG" {
			analysis.SuggestedSL = currentPrice * (1 - atrPercent/100)
			analysis.SuggestedTP = currentPrice * (1 + (atrPercent*2)/100)
		} else {
			analysis.SuggestedSL = currentPrice * (1 + atrPercent/100)
			analysis.SuggestedTP = currentPrice * (1 - (atrPercent*2)/100)
		}

		analysis.RiskReward = 2.0 // Fixed 2:1 for breakout trades
	}

	return analysis
}

// ============ HELPER FUNCTIONS ============

func (bd *BreakoutDetector) calculateAverageVolume(klines []binance.Kline, period int) float64 {
	if len(klines) < period {
		period = len(klines)
	}
	sum := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		sum += klines[i].Volume
	}
	if period == 0 {
		return 0
	}
	return sum / float64(period)
}

func (bd *BreakoutDetector) calculateVolumeProfile(klines []binance.Kline, buckets int) []VolumeLevel {
	if len(klines) == 0 || buckets <= 0 {
		return nil
	}

	minPrice := klines[0].Low
	maxPrice := klines[0].High
	for _, k := range klines {
		if k.Low < minPrice {
			minPrice = k.Low
		}
		if k.High > maxPrice {
			maxPrice = k.High
		}
	}

	if maxPrice == minPrice {
		return nil
	}

	bucketSize := (maxPrice - minPrice) / float64(buckets)
	levels := make([]VolumeLevel, buckets)
	totalVolume := 0.0

	for i := 0; i < buckets; i++ {
		levels[i].PriceLow = minPrice + float64(i)*bucketSize
		levels[i].PriceHigh = minPrice + float64(i+1)*bucketSize
	}

	for _, k := range klines {
		avgPrice := (k.High + k.Low + k.Close) / 3
		bucketIdx := int((avgPrice - minPrice) / bucketSize)
		if bucketIdx >= buckets {
			bucketIdx = buckets - 1
		}
		if bucketIdx < 0 {
			bucketIdx = 0
		}
		levels[bucketIdx].Volume += k.Volume
		totalVolume += k.Volume
	}

	for i := range levels {
		if totalVolume > 0 {
			levels[i].Percentage = levels[i].Volume / totalVolume * 100
		}
	}

	return levels
}

func (bd *BreakoutDetector) findVolumeNodes(profile []VolumeLevel) (high []float64, low []float64) {
	if len(profile) == 0 {
		return nil, nil
	}

	avgVol := 0.0
	for _, l := range profile {
		avgVol += l.Volume
	}
	avgVol /= float64(len(profile))

	for _, l := range profile {
		midPrice := (l.PriceLow + l.PriceHigh) / 2
		if l.Volume > avgVol*1.5 {
			high = append(high, midPrice)
		} else if l.Volume < avgVol*0.5 {
			low = append(low, midPrice)
		}
	}

	return high, low
}

func (bd *BreakoutDetector) calculateCumulativeDelta(klines []binance.Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	buyVolume := 0.0
	sellVolume := 0.0

	for i := len(klines) - period; i < len(klines); i++ {
		k := klines[i]
		totalRange := k.High - k.Low
		if totalRange == 0 {
			continue
		}

		if k.Close >= k.Open {
			buyRatio := (k.Close - k.Low) / totalRange
			buyVolume += k.Volume * buyRatio
			sellVolume += k.Volume * (1 - buyRatio)
		} else {
			sellRatio := (k.High - k.Close) / totalRange
			sellVolume += k.Volume * sellRatio
			buyVolume += k.Volume * (1 - sellRatio)
		}
	}

	total := buyVolume + sellVolume
	if total == 0 {
		return 0
	}
	return (buyVolume - sellVolume) / total
}

func (bd *BreakoutDetector) calculateVolumeScore(vb *VolumeBreakout) float64 {
	score := 0.0

	// Volume spike contribution (0-40 points)
	if vb.IsSpiking {
		spikeScore := math.Min(vb.SpikeMultiplier/4.0, 1.0) * 40
		score += spikeScore
	}

	// Volume trend contribution (0-25 points)
	if vb.VolumeTrend == "up" && vb.DeltaTrend == "accumulation" {
		score += 25
	} else if vb.VolumeTrend == "up" {
		score += 15
	}

	// Divergence contribution (0-20 points)
	if vb.HasDivergence {
		score += 20
	}

	// Delta contribution (0-15 points)
	deltaScore := math.Abs(vb.CumulativeDelta) * 15
	score += deltaScore

	return math.Min(score, 100)
}

func (bd *BreakoutDetector) findKeyLevels(klines []binance.Kline) (resistances []float64, supports []float64) {
	if len(klines) < 20 {
		return nil, nil
	}

	// Find local maxima and minima in the last 20 candles
	for i := len(klines) - 19; i < len(klines)-1; i++ {
		// Local maximum (resistance)
		if klines[i].High > klines[i-1].High && klines[i].High > klines[i+1].High {
			resistances = append(resistances, klines[i].High)
		}
		// Local minimum (support)
		if klines[i].Low < klines[i-1].Low && klines[i].Low < klines[i+1].Low {
			supports = append(supports, klines[i].Low)
		}
	}

	return resistances, supports
}

func (bd *BreakoutDetector) countConsecutiveDirection(klines []binance.Kline, maxLookback int) int {
	if len(klines) < 2 {
		return 0
	}

	lastDir := klines[len(klines)-1].Close >= klines[len(klines)-1].Open
	count := 1

	for i := len(klines) - 2; i >= 0 && i >= len(klines)-maxLookback; i-- {
		currDir := klines[i].Close >= klines[i].Open
		if currDir == lastDir {
			count++
		} else {
			break
		}
	}

	if lastDir {
		return count
	}
	return -count
}

func (bd *BreakoutDetector) calculateATRSimple(klines []binance.Kline, period int) float64 {
	if len(klines) < 2 {
		return 0
	}

	sum := 0.0
	count := 0

	for i := 1; i < len(klines) && i <= period; i++ {
		tr := math.Max(
			klines[i].High-klines[i].Low,
			math.Max(
				math.Abs(klines[i].High-klines[i-1].Close),
				math.Abs(klines[i].Low-klines[i-1].Close),
			),
		)
		sum += tr
		count++
	}

	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func (bd *BreakoutDetector) calculatePriceScore(pb *PriceBreakout) float64 {
	score := 0.0

	// Breaking 24h high (0-30 points)
	if pb.Breaking24hHigh {
		score += 30
	} else if pb.NearHighPercent > 0.7 {
		score += 20 * pb.NearHighPercent
	}

	// Breaking resistance (0-25 points)
	if pb.BreakingResistance {
		score += 25
	}

	// Candle acceleration (0-20 points)
	if pb.IsAccelerating {
		accelScore := math.Min(pb.BodySizeRatio/3.0, 1.0) * 20
		score += accelScore
	}

	// Consecutive direction (0-15 points)
	consecScore := math.Min(math.Abs(float64(pb.ConsecutiveDir))/5.0, 1.0) * 15
	score += consecScore

	// Range contraction before breakout (0-10 points)
	if pb.IsContracting {
		score += 10
	}

	return math.Min(score, 100)
}

func (bd *BreakoutDetector) calculateROC(klines []binance.Kline, period int) float64 {
	if len(klines) < period+1 {
		return 0
	}
	current := klines[len(klines)-1].Close
	past := klines[len(klines)-1-period].Close
	if past == 0 {
		return 0
	}
	return ((current - past) / past) * 100
}

func (bd *BreakoutDetector) calculateVelocity(klines []binance.Kline, period int) float64 {
	if len(klines) < period+1 {
		return 0
	}
	priceDiff := klines[len(klines)-1].Close - klines[len(klines)-period].Close
	return priceDiff / float64(period)
}

func (bd *BreakoutDetector) calculateAcceleration(klines []binance.Kline, period int) float64 {
	if len(klines) < 2*period+1 {
		return 0
	}
	velocity1 := bd.calculateVelocity(klines, period)
	olderKlines := klines[:len(klines)-period]
	velocity2 := bd.calculateVelocity(olderKlines, period)
	return velocity1 - velocity2
}

func (bd *BreakoutDetector) calculateMomentum(klines []binance.Kline, period int) float64 {
	if len(klines) < period+1 {
		return 0
	}
	current := klines[len(klines)-1].Close
	past := klines[len(klines)-1-period].Close
	return current - past
}

func (bd *BreakoutDetector) calculateRSI(klines []binance.Kline, period int) float64 {
	if len(klines) < period+1 {
		return 50
	}

	gains := 0.0
	losses := 0.0

	for i := len(klines) - period; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses -= change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

func (bd *BreakoutDetector) calculateRSIWithOffset(klines []binance.Kline, period int, offset int) float64 {
	if len(klines) < period+1+offset {
		return 50
	}
	return bd.calculateRSI(klines[:len(klines)-offset], period)
}

func (bd *BreakoutDetector) calculateMACDHistogram(klines []binance.Kline) float64 {
	if len(klines) < 26 {
		return 0
	}

	ema12 := bd.calculateEMA(klines, 12)
	ema26 := bd.calculateEMA(klines, 26)
	macdLine := ema12 - ema26

	// Signal line (9-period EMA of MACD line) - simplified
	signalLine := macdLine * 0.9 // Approximation

	return macdLine - signalLine
}

func (bd *BreakoutDetector) calculateMACDHistogramWithOffset(klines []binance.Kline, offset int) float64 {
	if len(klines) < 26+offset {
		return 0
	}
	return bd.calculateMACDHistogram(klines[:len(klines)-offset])
}

func (bd *BreakoutDetector) calculateEMA(klines []binance.Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	multiplier := 2.0 / float64(period+1)

	// Start with SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	ema := sum / float64(period)

	// Calculate EMA
	for i := period; i < len(klines); i++ {
		ema = (klines[i].Close-ema)*multiplier + ema
	}

	return ema
}

func (bd *BreakoutDetector) calculateMomentumScore(mb *MomentumBreakout) float64 {
	score := 0.0

	// ROC acceleration (0-25 points)
	if mb.ROCAccelerating {
		rocScore := math.Min(math.Abs(mb.ROC5)/5.0, 1.0) * 25
		score += rocScore
	}

	// Price acceleration (0-20 points)
	if mb.IsAccelerating {
		score += 20
	} else if mb.AccelerationPhase == "constant" {
		score += 5
	}

	// MTF alignment (0-25 points)
	if mb.MTFAligned {
		mtfScore := (float64(mb.MTFConsensus) / 4.0) * 25
		score += mtfScore
	}

	// RSI crossing 50 (0-15 points)
	if mb.RSICrossing50 {
		score += 15
	}

	// MACD expanding (0-15 points)
	if mb.MACDExpanding {
		score += 15
	}

	return math.Min(score, 100)
}

func (bd *BreakoutDetector) classifyStrength(score float64) string {
	if score >= 80 {
		return "very_strong"
	} else if score >= 60 {
		return "strong"
	} else if score >= 40 {
		return "moderate"
	}
	return "weak"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
