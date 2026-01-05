package autopilot

import (
	"math"
	"sort"
	"time"

	"binance-trading-bot/internal/binance"
)

// ============ CHART PATTERN DETECTION ============

// detectChartPatterns orchestrates all chart pattern detection
func (g *GinieAnalyzer) detectChartPatterns(klines []binance.Kline, currentPrice float64) ChartPatternAnalysis {
	if len(klines) < 30 {
		return ChartPatternAnalysis{
			PatternBias: "neutral",
		}
	}

	// Find swing points for pattern detection
	swingHighs, swingLows := g.findSwingPointsWithIndex(klines, 5)

	// Detect all pattern types
	headAndShoulders := g.detectHeadAndShoulders(klines, swingHighs, swingLows)
	doubleTopsBottoms := g.detectDoubleTopsBottoms(klines, swingHighs, swingLows)
	triangles := g.detectTriangles(klines, swingHighs, swingLows)
	wedges := g.detectWedges(klines, swingHighs, swingLows)
	flagsPennants := g.detectFlagsPennants(klines, currentPrice)

	// Build analysis result
	analysis := ChartPatternAnalysis{
		HeadAndShoulders:  headAndShoulders,
		DoubleTopsBottoms: doubleTopsBottoms,
		Triangles:         triangles,
		Wedges:            wedges,
		FlagsPennants:     flagsPennants,
	}

	// Calculate pattern counts
	analysis.TotalPatterns = len(headAndShoulders) + len(doubleTopsBottoms) + len(triangles) + len(wedges) + len(flagsPennants)
	analysis.ReversalPatterns = len(headAndShoulders) + len(doubleTopsBottoms)
	analysis.ContinuationPatterns = len(flagsPennants)
	analysis.ConsolidationPatterns = len(triangles) + len(wedges)

	// Determine pattern bias and find active pattern
	analysis.PatternBias, analysis.ActivePattern = g.determinePatternBias(analysis, currentPrice)
	analysis.HasBullishPattern = g.hasBullishPattern(analysis)
	analysis.HasBearishPattern = g.hasBearishPattern(analysis)
	analysis.NearBreakout = g.isNearBreakout(analysis, currentPrice)

	// Calculate pattern score
	analysis.PatternScore = g.calculatePatternScore(analysis)

	// Set estimated target from active pattern
	if analysis.ActivePattern != nil {
		analysis.EstimatedTarget = analysis.ActivePattern.TargetPrice
	}

	return analysis
}

// findSwingPointsWithIndex finds swing highs and lows with full context
func (g *GinieAnalyzer) findSwingPointsWithIndex(klines []binance.Kline, lookback int) ([]SwingPoint, []SwingPoint) {
	var highs, lows []SwingPoint

	if len(klines) < lookback*2+1 {
		return highs, lows
	}

	for i := lookback; i < len(klines)-lookback; i++ {
		isHigh := true
		isLow := true

		for j := i - lookback; j <= i+lookback; j++ {
			if j == i {
				continue
			}
			if klines[j].High >= klines[i].High {
				isHigh = false
			}
			if klines[j].Low <= klines[i].Low {
				isLow = false
			}
		}

		if isHigh {
			highs = append(highs, SwingPoint{
				Price:     klines[i].High,
				Index:     i,
				Timestamp: time.UnixMilli(klines[i].OpenTime),
				Volume:    klines[i].Volume,
			})
		}
		if isLow {
			lows = append(lows, SwingPoint{
				Price:     klines[i].Low,
				Index:     i,
				Timestamp: time.UnixMilli(klines[i].OpenTime),
				Volume:    klines[i].Volume,
			})
		}
	}

	return highs, lows
}

// ============ HEAD AND SHOULDERS DETECTION ============

func (g *GinieAnalyzer) detectHeadAndShoulders(klines []binance.Kline, swingHighs, swingLows []SwingPoint) []HeadAndShouldersPattern {
	var patterns []HeadAndShouldersPattern

	// Detect regular Head and Shoulders (bearish reversal)
	patterns = append(patterns, g.detectHnSPatterns(klines, swingHighs, swingLows, false)...)

	// Detect Inverse Head and Shoulders (bullish reversal)
	patterns = append(patterns, g.detectHnSPatterns(klines, swingLows, swingHighs, true)...)

	return patterns
}

func (g *GinieAnalyzer) detectHnSPatterns(klines []binance.Kline, peaks, troughs []SwingPoint, inverse bool) []HeadAndShouldersPattern {
	var patterns []HeadAndShouldersPattern

	if len(peaks) < 3 || len(troughs) < 2 {
		return patterns
	}

	// Look for potential H&S patterns
	for i := 1; i < len(peaks)-1; i++ {
		leftShoulder := peaks[i-1]
		head := peaks[i]
		rightShoulder := peaks[i+1]

		// Minimum separation check
		if rightShoulder.Index-leftShoulder.Index < 10 {
			continue
		}

		// For regular H&S: head must be highest
		// For inverse H&S: head must be lowest
		if !inverse {
			if head.Price <= leftShoulder.Price || head.Price <= rightShoulder.Price {
				continue
			}
		} else {
			if head.Price >= leftShoulder.Price || head.Price >= rightShoulder.Price {
				continue
			}
		}

		// Shoulders should be within 15% of each other
		shoulderDiff := math.Abs(leftShoulder.Price-rightShoulder.Price) / math.Min(leftShoulder.Price, rightShoulder.Price)
		if shoulderDiff > 0.15 {
			continue
		}

		// Find neckline points (troughs between shoulders and head)
		necklineLeft := g.findTroughBetween(troughs, leftShoulder.Index, head.Index)
		necklineRight := g.findTroughBetween(troughs, head.Index, rightShoulder.Index)

		if necklineLeft == nil || necklineRight == nil {
			continue
		}

		// Calculate neckline slope
		necklineSlope := 0.0
		if necklineRight.Index != necklineLeft.Index {
			necklineSlope = (necklineRight.Price - necklineLeft.Price) / float64(necklineRight.Index-necklineLeft.Index)
		}

		// Calculate pattern metrics
		avgNeckline := (necklineLeft.Price + necklineRight.Price) / 2
		var patternHeight float64
		if !inverse {
			patternHeight = head.Price - avgNeckline
		} else {
			patternHeight = avgNeckline - head.Price
		}
		patternPercent := patternHeight / avgNeckline * 100

		// Calculate symmetry score
		leftDistance := head.Index - leftShoulder.Index
		rightDistance := rightShoulder.Index - head.Index
		symmetryRatio := math.Min(float64(leftDistance), float64(rightDistance)) / math.Max(float64(leftDistance), float64(rightDistance))
		symmetryScore := symmetryRatio * 100

		// Add price symmetry
		priceDiff := 1.0 - shoulderDiff
		symmetryScore = (symmetryScore + priceDiff*100) / 2

		// Calculate target price
		var targetPrice float64
		necklineAtRightShoulder := necklineLeft.Price + necklineSlope*float64(rightShoulder.Index-necklineLeft.Index)
		if !inverse {
			targetPrice = necklineAtRightShoulder - patternHeight
		} else {
			targetPrice = necklineAtRightShoulder + patternHeight
		}

		// Determine strength
		strength := "weak"
		if symmetryScore >= 80 && patternPercent >= 5 {
			strength = "strong"
		} else if symmetryScore >= 60 && patternPercent >= 3 {
			strength = "moderate"
		}

		patternType := "head_and_shoulders"
		if inverse {
			patternType = "inverse_head_and_shoulders"
		}

		pattern := HeadAndShouldersPattern{
			Type:            patternType,
			LeftShoulder:    leftShoulder,
			Head:            head,
			RightShoulder:   rightShoulder,
			NecklineLeft:    *necklineLeft,
			NecklineRight:   *necklineRight,
			NecklineSlope:   necklineSlope,
			NecklinePrice:   necklineAtRightShoulder,
			TargetPrice:     targetPrice,
			PatternHeight:   patternHeight,
			PatternPercent:  patternPercent,
			SymmetryScore:   symmetryScore,
			VolumeConfirmed: g.checkHnSVolume(klines, leftShoulder.Index, head.Index, rightShoulder.Index),
			Completed:       false,
			CandleIndex:     rightShoulder.Index,
			Timestamp:       rightShoulder.Timestamp,
			Strength:        strength,
		}

		// Check if neckline is broken
		if len(klines) > rightShoulder.Index {
			lastPrice := klines[len(klines)-1].Close
			if !inverse && lastPrice < necklineAtRightShoulder {
				pattern.Completed = true
			} else if inverse && lastPrice > necklineAtRightShoulder {
				pattern.Completed = true
			}
		}

		patterns = append(patterns, pattern)
	}

	return patterns
}

func (g *GinieAnalyzer) findTroughBetween(troughs []SwingPoint, startIdx, endIdx int) *SwingPoint {
	var best *SwingPoint
	for i := range troughs {
		if troughs[i].Index > startIdx && troughs[i].Index < endIdx {
			if best == nil || troughs[i].Price < best.Price {
				best = &troughs[i]
			}
		}
	}
	return best
}

func (g *GinieAnalyzer) checkHnSVolume(klines []binance.Kline, leftIdx, headIdx, rightIdx int) bool {
	if leftIdx >= len(klines) || headIdx >= len(klines) || rightIdx >= len(klines) {
		return false
	}

	leftVol := klines[leftIdx].Volume
	headVol := klines[headIdx].Volume
	rightVol := klines[rightIdx].Volume

	// Classic H&S: volume decreases from left shoulder to head to right shoulder
	return headVol <= leftVol && rightVol <= headVol
}

// ============ DOUBLE TOP/BOTTOM DETECTION ============

func (g *GinieAnalyzer) detectDoubleTopsBottoms(klines []binance.Kline, swingHighs, swingLows []SwingPoint) []DoubleTopBottomPattern {
	var patterns []DoubleTopBottomPattern

	// Detect Double Tops
	patterns = append(patterns, g.detectDoublePatterns(klines, swingHighs, swingLows, true)...)

	// Detect Double Bottoms
	patterns = append(patterns, g.detectDoublePatterns(klines, swingLows, swingHighs, false)...)

	return patterns
}

func (g *GinieAnalyzer) detectDoublePatterns(klines []binance.Kline, peaks, troughs []SwingPoint, isTop bool) []DoubleTopBottomPattern {
	var patterns []DoubleTopBottomPattern

	if len(peaks) < 2 {
		return patterns
	}

	for i := 0; i < len(peaks)-1; i++ {
		firstPeak := peaks[i]
		secondPeak := peaks[i+1]

		// Minimum separation
		barsBetween := secondPeak.Index - firstPeak.Index
		if barsBetween < 6 || barsBetween > 50 {
			continue
		}

		// Peaks should be within 3% of each other
		peakDiff := math.Abs(firstPeak.Price-secondPeak.Price) / math.Min(firstPeak.Price, secondPeak.Price) * 100
		if peakDiff > 3.0 {
			continue
		}

		// Find the neckline (trough/peak between the two peaks)
		var neckline *SwingPoint
		for j := range troughs {
			if troughs[j].Index > firstPeak.Index && troughs[j].Index < secondPeak.Index {
				if neckline == nil {
					neckline = &troughs[j]
				} else if isTop && troughs[j].Price < neckline.Price {
					neckline = &troughs[j]
				} else if !isTop && troughs[j].Price > neckline.Price {
					neckline = &troughs[j]
				}
			}
		}

		if neckline == nil {
			continue
		}

		// Calculate pattern height
		avgPeak := (firstPeak.Price + secondPeak.Price) / 2
		var patternHeight float64
		if isTop {
			patternHeight = avgPeak - neckline.Price
		} else {
			patternHeight = neckline.Price - avgPeak
		}
		patternPercent := patternHeight / avgPeak * 100

		// Minimum height requirement
		if patternPercent < 1.5 {
			continue
		}

		// Calculate target
		var targetPrice float64
		if isTop {
			targetPrice = neckline.Price - patternHeight
		} else {
			targetPrice = neckline.Price + patternHeight
		}

		// Check volume confirmation (first peak should have higher volume)
		volumeConfirmed := firstPeak.Volume > secondPeak.Volume

		// Determine strength
		strength := "weak"
		if patternPercent >= 5 && volumeConfirmed {
			strength = "strong"
		} else if patternPercent >= 3 || volumeConfirmed {
			strength = "moderate"
		}

		patternType := "double_top"
		if !isTop {
			patternType = "double_bottom"
		}

		pattern := DoubleTopBottomPattern{
			Type:            patternType,
			FirstPeak:       firstPeak,
			SecondPeak:      secondPeak,
			Neckline:        *neckline,
			NecklinePrice:   neckline.Price,
			TargetPrice:     targetPrice,
			PatternHeight:   patternHeight,
			PatternPercent:  patternPercent,
			PeakDifference:  peakDiff,
			BarsBetween:     barsBetween,
			VolumeConfirmed: volumeConfirmed,
			Status:          "forming",
			Completed:       false,
			CandleIndex:     secondPeak.Index,
			Timestamp:       secondPeak.Timestamp,
			Strength:        strength,
		}

		// Check if neckline is broken
		if len(klines) > secondPeak.Index {
			lastPrice := klines[len(klines)-1].Close
			if isTop && lastPrice < neckline.Price {
				pattern.Completed = true
				pattern.Status = "confirmed"
			} else if !isTop && lastPrice > neckline.Price {
				pattern.Completed = true
				pattern.Status = "confirmed"
			}
		}

		patterns = append(patterns, pattern)
	}

	return patterns
}

// ============ TRIANGLE DETECTION ============

func (g *GinieAnalyzer) detectTriangles(klines []binance.Kline, swingHighs, swingLows []SwingPoint) []TrianglePattern {
	var patterns []TrianglePattern

	if len(swingHighs) < 3 || len(swingLows) < 3 {
		return patterns
	}

	// Need at least 3 recent swing points on each side
	recentHighs := g.getRecentSwingPoints(swingHighs, 5)
	recentLows := g.getRecentSwingPoints(swingLows, 5)

	if len(recentHighs) < 2 || len(recentLows) < 2 {
		return patterns
	}

	// Fit trendlines
	upperTrendline := g.fitTrendline(recentHighs)
	lowerTrendline := g.fitTrendline(recentLows)

	// Check for convergence
	if !g.linesConverge(upperTrendline, lowerTrendline) {
		return patterns
	}

	// Classify triangle type
	upperSlope := upperTrendline.Slope
	lowerSlope := lowerTrendline.Slope

	var triangleType, breakoutBias string

	// Ascending: flat top, rising bottom
	if math.Abs(upperSlope) < 0.0001 && lowerSlope > 0.0001 {
		triangleType = "ascending"
		breakoutBias = "up"
	} else if math.Abs(lowerSlope) < 0.0001 && upperSlope < -0.0001 {
		// Descending: falling top, flat bottom
		triangleType = "descending"
		breakoutBias = "down"
	} else if upperSlope < -0.0001 && lowerSlope > 0.0001 {
		// Symmetrical: falling top, rising bottom
		triangleType = "symmetrical"
		breakoutBias = "neutral"
	} else {
		return patterns // Not a valid triangle
	}

	// Calculate apex
	apexIndex, apexPrice := g.findConvergencePoint(upperTrendline, lowerTrendline)

	// Calculate pattern metrics
	patternStart := int(math.Min(float64(upperTrendline.StartIndex), float64(lowerTrendline.StartIndex)))
	patternWidth := len(klines) - 1 - patternStart
	baseHeight := upperTrendline.StartPrice - lowerTrendline.StartPrice
	currentHeight := upperTrendline.EndPrice - lowerTrendline.EndPrice
	contraction := (1 - currentHeight/baseHeight) * 100

	// Count trendline touches
	touchesUpper := g.countTrendlineTouches(klines, upperTrendline, 0.005)
	touchesLower := g.countTrendlineTouches(klines, lowerTrendline, 0.005)

	// Check volume decline
	volumeDecline := g.checkVolumeDecline(klines, patternStart)

	// Calculate breakout target
	var breakoutTarget float64
	lastUpperPrice := upperTrendline.EndPrice
	lastLowerPrice := lowerTrendline.EndPrice
	if breakoutBias == "up" {
		breakoutTarget = lastUpperPrice + baseHeight
	} else if breakoutBias == "down" {
		breakoutTarget = lastLowerPrice - baseHeight
	} else {
		breakoutTarget = lastUpperPrice + baseHeight/2 // Conservative for symmetrical
	}

	// Determine strength
	strength := "weak"
	if touchesUpper >= 3 && touchesLower >= 3 && contraction > 50 {
		strength = "strong"
	} else if touchesUpper >= 2 && touchesLower >= 2 && contraction > 30 {
		strength = "moderate"
	}

	pattern := TrianglePattern{
		Type:           triangleType,
		UpperTrendline: upperTrendline,
		LowerTrendline: lowerTrendline,
		ApexPrice:      apexPrice,
		ApexIndex:      apexIndex,
		PatternStart:   patternStart,
		PatternWidth:   patternWidth,
		BaseHeight:     baseHeight,
		BasePercent:    baseHeight / upperTrendline.StartPrice * 100,
		CurrentHeight:  currentHeight,
		Contraction:    contraction,
		VolumeDecline:  volumeDecline,
		BreakoutBias:   breakoutBias,
		BreakoutTarget: breakoutTarget,
		TouchesUpper:   touchesUpper,
		TouchesLower:   touchesLower,
		Completed:      false,
		CandleIndex:    len(klines) - 1,
		Timestamp:      time.UnixMilli(klines[len(klines)-1].OpenTime),
		Strength:       strength,
	}

	// Check for breakout
	if len(klines) > 0 {
		lastClose := klines[len(klines)-1].Close
		if lastClose > upperTrendline.EndPrice*1.005 {
			pattern.Completed = true
			pattern.BreakoutDir = "up"
		} else if lastClose < lowerTrendline.EndPrice*0.995 {
			pattern.Completed = true
			pattern.BreakoutDir = "down"
		}
	}

	patterns = append(patterns, pattern)
	return patterns
}

// ============ WEDGE DETECTION ============

func (g *GinieAnalyzer) detectWedges(klines []binance.Kline, swingHighs, swingLows []SwingPoint) []WedgePattern {
	var patterns []WedgePattern

	if len(swingHighs) < 3 || len(swingLows) < 3 {
		return patterns
	}

	recentHighs := g.getRecentSwingPoints(swingHighs, 5)
	recentLows := g.getRecentSwingPoints(swingLows, 5)

	if len(recentHighs) < 2 || len(recentLows) < 2 {
		return patterns
	}

	upperTrendline := g.fitTrendline(recentHighs)
	lowerTrendline := g.fitTrendline(recentLows)

	// Wedges: both lines slope in same direction but converge
	if !g.linesConverge(upperTrendline, lowerTrendline) {
		return patterns
	}

	upperSlope := upperTrendline.Slope
	lowerSlope := lowerTrendline.Slope

	var wedgeType, breakoutBias string

	// Rising Wedge: both slopes positive, lower slope > upper slope (converging upward)
	if upperSlope > 0.0001 && lowerSlope > upperSlope {
		wedgeType = "rising_wedge"
		breakoutBias = "down" // Bearish reversal
	} else if upperSlope < -0.0001 && lowerSlope < 0 && upperSlope < lowerSlope {
		// Falling Wedge: both slopes negative, upper slope more negative (converging downward)
		wedgeType = "falling_wedge"
		breakoutBias = "up" // Bullish reversal
	} else {
		return patterns // Not a wedge
	}

	// Calculate metrics
	apexIndex, apexPrice := g.findConvergencePoint(upperTrendline, lowerTrendline)
	patternStart := int(math.Min(float64(upperTrendline.StartIndex), float64(lowerTrendline.StartIndex)))
	patternWidth := len(klines) - 1 - patternStart
	baseHeight := math.Abs(upperTrendline.StartPrice - lowerTrendline.StartPrice)
	slopeRatio := math.Abs(upperSlope / lowerSlope)

	touchesUpper := g.countTrendlineTouches(klines, upperTrendline, 0.005)
	touchesLower := g.countTrendlineTouches(klines, lowerTrendline, 0.005)
	volumeDecline := g.checkVolumeDecline(klines, patternStart)

	// Breakout target
	var breakoutTarget float64
	if breakoutBias == "down" {
		breakoutTarget = lowerTrendline.EndPrice - baseHeight
	} else {
		breakoutTarget = upperTrendline.EndPrice + baseHeight
	}

	strength := "weak"
	if touchesUpper >= 3 && touchesLower >= 3 {
		strength = "strong"
	} else if touchesUpper >= 2 && touchesLower >= 2 {
		strength = "moderate"
	}

	pattern := WedgePattern{
		Type:           wedgeType,
		UpperTrendline: upperTrendline,
		LowerTrendline: lowerTrendline,
		ApexPrice:      apexPrice,
		ApexIndex:      apexIndex,
		PatternStart:   patternStart,
		PatternWidth:   patternWidth,
		SlopeRatio:     slopeRatio,
		BaseHeight:     baseHeight,
		BasePercent:    baseHeight / upperTrendline.StartPrice * 100,
		BreakoutBias:   breakoutBias,
		BreakoutTarget: breakoutTarget,
		TouchesUpper:   touchesUpper,
		TouchesLower:   touchesLower,
		VolumeDecline:  volumeDecline,
		Completed:      false,
		CandleIndex:    len(klines) - 1,
		Timestamp:      time.UnixMilli(klines[len(klines)-1].OpenTime),
		Strength:       strength,
	}

	// Check for breakout
	if len(klines) > 0 {
		lastClose := klines[len(klines)-1].Close
		if wedgeType == "rising_wedge" && lastClose < lowerTrendline.EndPrice*0.995 {
			pattern.Completed = true
			pattern.BreakoutDir = "down"
		} else if wedgeType == "falling_wedge" && lastClose > upperTrendline.EndPrice*1.005 {
			pattern.Completed = true
			pattern.BreakoutDir = "up"
		}
	}

	patterns = append(patterns, pattern)
	return patterns
}

// ============ FLAG/PENNANT DETECTION ============

func (g *GinieAnalyzer) detectFlagsPennants(klines []binance.Kline, currentPrice float64) []FlagPennantPattern {
	var patterns []FlagPennantPattern

	if len(klines) < 20 {
		return patterns
	}

	// Look for strong impulse moves (flagpoles) followed by consolidation
	for i := 10; i < len(klines)-5; i++ {
		// Check for bullish flagpole (strong up move)
		bullishPole := g.detectFlagpole(klines, i, true)
		if bullishPole != nil {
			if pattern := g.analyzeConsolidation(klines, *bullishPole, true); pattern != nil {
				patterns = append(patterns, *pattern)
			}
		}

		// Check for bearish flagpole (strong down move)
		bearishPole := g.detectFlagpole(klines, i, false)
		if bearishPole != nil {
			if pattern := g.analyzeConsolidation(klines, *bearishPole, false); pattern != nil {
				patterns = append(patterns, *pattern)
			}
		}
	}

	return patterns
}

type flagpoleInfo struct {
	start    SwingPoint
	end      SwingPoint
	height   float64
	percent  float64
	bars     int
	avgVol   float64
	endIndex int
}

func (g *GinieAnalyzer) detectFlagpole(klines []binance.Kline, endIdx int, bullish bool) *flagpoleInfo {
	minPolePercent := 3.0 // Minimum 3% move for flagpole
	maxPoleBars := 10     // Flagpole should form quickly

	// Look back for flagpole start
	for startIdx := endIdx - 3; startIdx >= endIdx-maxPoleBars && startIdx >= 0; startIdx-- {
		var height, percent float64

		if bullish {
			height = klines[endIdx].High - klines[startIdx].Low
			percent = height / klines[startIdx].Low * 100
		} else {
			height = klines[startIdx].High - klines[endIdx].Low
			percent = height / klines[startIdx].High * 100
		}

		if percent >= minPolePercent {
			// Calculate average volume during pole
			var totalVol float64
			for j := startIdx; j <= endIdx; j++ {
				totalVol += klines[j].Volume
			}
			avgVol := totalVol / float64(endIdx-startIdx+1)

			return &flagpoleInfo{
				start: SwingPoint{
					Price:     ternary(bullish, klines[startIdx].Low, klines[startIdx].High),
					Index:     startIdx,
					Timestamp: time.UnixMilli(klines[startIdx].OpenTime),
				},
				end: SwingPoint{
					Price:     ternary(bullish, klines[endIdx].High, klines[endIdx].Low),
					Index:     endIdx,
					Timestamp: time.UnixMilli(klines[endIdx].OpenTime),
				},
				height:   height,
				percent:  percent,
				bars:     endIdx - startIdx + 1,
				avgVol:   avgVol,
				endIndex: endIdx,
			}
		}
	}

	return nil
}

func (g *GinieAnalyzer) analyzeConsolidation(klines []binance.Kline, pole flagpoleInfo, bullish bool) *FlagPennantPattern {
	consolidationStart := pole.endIndex + 1
	if consolidationStart >= len(klines)-3 {
		return nil
	}

	// Find consolidation end (current or where consolidation breaks)
	consolidationEnd := len(klines) - 1
	consolidationBars := consolidationEnd - consolidationStart + 1

	if consolidationBars < 3 || consolidationBars > 30 {
		return nil
	}

	// Calculate consolidation range
	var consHigh, consLow float64 = klines[consolidationStart].High, klines[consolidationStart].Low
	var totalVol float64

	for j := consolidationStart; j <= consolidationEnd; j++ {
		if klines[j].High > consHigh {
			consHigh = klines[j].High
		}
		if klines[j].Low < consLow {
			consLow = klines[j].Low
		}
		totalVol += klines[j].Volume
	}
	consAvgVol := totalVol / float64(consolidationBars)

	// Calculate retracement
	var retracement float64
	if bullish {
		retracement = (pole.end.Price - consLow) / pole.height * 100
	} else {
		retracement = (consHigh - pole.end.Price) / pole.height * 100
	}

	// Retracement should be 30-61.8%
	if retracement > 61.8 {
		return nil
	}

	// Determine if flag or pennant
	// Flag: parallel channel (retracement is relatively uniform)
	// Pennant: converging (range contracts)
	consolidationType := "channel" // Default to flag
	if consolidationBars >= 5 {
		firstHalfRange := g.getRangeForPeriod(klines, consolidationStart, consolidationStart+consolidationBars/2)
		secondHalfRange := g.getRangeForPeriod(klines, consolidationStart+consolidationBars/2, consolidationEnd)
		if secondHalfRange < firstHalfRange*0.7 {
			consolidationType = "triangle" // Pennant
		}
	}

	// Determine pattern type
	var patternType, direction string
	if bullish {
		direction = "bullish"
		if consolidationType == "channel" {
			patternType = "bull_flag"
		} else {
			patternType = "bull_pennant"
		}
	} else {
		direction = "bearish"
		if consolidationType == "channel" {
			patternType = "bear_flag"
		} else {
			patternType = "bear_pennant"
		}
	}

	// Calculate targets
	var breakoutLevel, targetPrice, stopLoss float64
	if bullish {
		breakoutLevel = consHigh
		targetPrice = breakoutLevel + pole.height
		stopLoss = consLow
	} else {
		breakoutLevel = consLow
		targetPrice = breakoutLevel - pole.height
		stopLoss = consHigh
	}

	// Volume confirmation: consolidation volume should be lower than pole volume
	volumeConfirmed := consAvgVol < pole.avgVol*0.7

	// Determine strength
	strength := "weak"
	if volumeConfirmed && retracement <= 50 && consolidationBars <= 15 {
		strength = "strong"
	} else if volumeConfirmed || retracement <= 50 {
		strength = "moderate"
	}

	pattern := FlagPennantPattern{
		Type:              patternType,
		Direction:         direction,
		FlagpoleStart:     pole.start,
		FlagpoleEnd:       pole.end,
		FlagpoleHeight:    pole.height,
		FlagpolePercent:   pole.percent,
		FlagpoleBars:      pole.bars,
		FlagpoleVolume:    pole.avgVol,
		ConsolidationType: consolidationType,
		ConsolidationHigh: consHigh,
		ConsolidationLow:  consLow,
		ConsolidationBars: consolidationBars,
		RetracementPct:    retracement,
		ConsolidationVol:  consAvgVol,
		BreakoutLevel:     breakoutLevel,
		TargetPrice:       targetPrice,
		StopLoss:          stopLoss,
		Completed:         false,
		VolumeConfirmed:   volumeConfirmed,
		CandleIndex:       consolidationEnd,
		Timestamp:         time.UnixMilli(klines[consolidationEnd].OpenTime),
		Strength:          strength,
	}

	// Check if breakout occurred
	lastClose := klines[len(klines)-1].Close
	if bullish && lastClose > breakoutLevel*1.005 {
		pattern.Completed = true
	} else if !bullish && lastClose < breakoutLevel*0.995 {
		pattern.Completed = true
	}

	return &pattern
}

// ============ HELPER FUNCTIONS ============

func (g *GinieAnalyzer) getRecentSwingPoints(points []SwingPoint, count int) []SwingPoint {
	if len(points) <= count {
		return points
	}
	return points[len(points)-count:]
}

func (g *GinieAnalyzer) fitTrendline(points []SwingPoint) Trendline {
	if len(points) < 2 {
		return Trendline{}
	}

	// Simple linear regression
	n := float64(len(points))
	var sumX, sumY, sumXY, sumX2 float64

	for i, p := range points {
		x := float64(i)
		sumX += x
		sumY += p.Price
		sumXY += x * p.Price
		sumX2 += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / n

	startPrice := intercept
	endPrice := intercept + slope*float64(len(points)-1)

	var touchPoints []int
	for i, p := range points {
		expectedPrice := intercept + slope*float64(i)
		if math.Abs(p.Price-expectedPrice)/expectedPrice < 0.01 {
			touchPoints = append(touchPoints, p.Index)
		}
	}

	return Trendline{
		StartPrice:  startPrice,
		EndPrice:    endPrice,
		StartIndex:  points[0].Index,
		EndIndex:    points[len(points)-1].Index,
		Slope:       slope,
		TouchPoints: touchPoints,
	}
}

func (g *GinieAnalyzer) linesConverge(upper, lower Trendline) bool {
	// Lines converge if the gap is narrowing
	startGap := upper.StartPrice - lower.StartPrice
	endGap := upper.EndPrice - lower.EndPrice

	return endGap < startGap && endGap > 0
}

func (g *GinieAnalyzer) findConvergencePoint(upper, lower Trendline) (int, float64) {
	// Find where lines intersect
	// Upper: y = upper.StartPrice + upper.Slope * x
	// Lower: y = lower.StartPrice + lower.Slope * x

	if math.Abs(upper.Slope-lower.Slope) < 0.0001 {
		return 0, 0 // Parallel lines
	}

	x := (lower.StartPrice - upper.StartPrice) / (upper.Slope - lower.Slope)
	y := upper.StartPrice + upper.Slope*x

	return int(x) + upper.StartIndex, y
}

func (g *GinieAnalyzer) countTrendlineTouches(klines []binance.Kline, tl Trendline, tolerance float64) int {
	touches := 0
	barsPerPoint := float64(tl.EndIndex-tl.StartIndex) / float64(len(tl.TouchPoints))

	for i := tl.StartIndex; i <= tl.EndIndex && i < len(klines); i++ {
		relativeIdx := float64(i - tl.StartIndex)
		expectedPrice := tl.StartPrice + tl.Slope*relativeIdx/barsPerPoint

		high := klines[i].High
		low := klines[i].Low

		// Check if high or low touches the trendline
		if math.Abs(high-expectedPrice)/expectedPrice <= tolerance ||
			math.Abs(low-expectedPrice)/expectedPrice <= tolerance {
			touches++
		}
	}

	return touches
}

func (g *GinieAnalyzer) checkVolumeDecline(klines []binance.Kline, startIdx int) bool {
	if startIdx >= len(klines)-5 {
		return false
	}

	// Compare first half volume to second half
	mid := (startIdx + len(klines) - 1) / 2
	var firstHalf, secondHalf float64

	for i := startIdx; i < mid; i++ {
		firstHalf += klines[i].Volume
	}
	for i := mid; i < len(klines); i++ {
		secondHalf += klines[i].Volume
	}

	return secondHalf < firstHalf*0.8
}

func (g *GinieAnalyzer) getRangeForPeriod(klines []binance.Kline, start, end int) float64 {
	if start >= end || end >= len(klines) {
		return 0
	}

	high := klines[start].High
	low := klines[start].Low

	for i := start + 1; i <= end; i++ {
		if klines[i].High > high {
			high = klines[i].High
		}
		if klines[i].Low < low {
			low = klines[i].Low
		}
	}

	return high - low
}

// ============ SCORING AND BIAS FUNCTIONS ============

func (g *GinieAnalyzer) determinePatternBias(analysis ChartPatternAnalysis, currentPrice float64) (string, *PatternSummary) {
	bullishScore := 0
	bearishScore := 0
	var bestPattern *PatternSummary

	// Score Head and Shoulders patterns
	for _, p := range analysis.HeadAndShoulders {
		if p.Type == "inverse_head_and_shoulders" {
			bullishScore += g.patternStrengthScore(p.Strength)
			if bestPattern == nil || p.Strength == "strong" {
				bestPattern = &PatternSummary{
					Type:          p.Type,
					Direction:     "bullish",
					Strength:      p.Strength,
					TargetPrice:   p.TargetPrice,
					BreakoutLevel: p.NecklinePrice,
				}
			}
		} else {
			bearishScore += g.patternStrengthScore(p.Strength)
			if bestPattern == nil || p.Strength == "strong" {
				bestPattern = &PatternSummary{
					Type:          p.Type,
					Direction:     "bearish",
					Strength:      p.Strength,
					TargetPrice:   p.TargetPrice,
					BreakoutLevel: p.NecklinePrice,
				}
			}
		}
	}

	// Score Double Tops/Bottoms
	for _, p := range analysis.DoubleTopsBottoms {
		if p.Type == "double_bottom" {
			bullishScore += g.patternStrengthScore(p.Strength)
		} else {
			bearishScore += g.patternStrengthScore(p.Strength)
		}
	}

	// Score Triangles
	for _, p := range analysis.Triangles {
		switch p.BreakoutBias {
		case "up":
			bullishScore += g.patternStrengthScore(p.Strength)
		case "down":
			bearishScore += g.patternStrengthScore(p.Strength)
		}
	}

	// Score Wedges (reversal patterns)
	for _, p := range analysis.Wedges {
		if p.Type == "falling_wedge" {
			bullishScore += g.patternStrengthScore(p.Strength)
		} else {
			bearishScore += g.patternStrengthScore(p.Strength)
		}
	}

	// Score Flags/Pennants (continuation)
	for _, p := range analysis.FlagsPennants {
		if p.Direction == "bullish" {
			bullishScore += g.patternStrengthScore(p.Strength)
		} else {
			bearishScore += g.patternStrengthScore(p.Strength)
		}
	}

	bias := "neutral"
	if bullishScore > bearishScore && bullishScore >= 2 {
		bias = "bullish"
	} else if bearishScore > bullishScore && bearishScore >= 2 {
		bias = "bearish"
	}

	return bias, bestPattern
}

func (g *GinieAnalyzer) patternStrengthScore(strength string) int {
	switch strength {
	case "strong":
		return 3
	case "moderate":
		return 2
	case "weak":
		return 1
	default:
		return 0
	}
}

func (g *GinieAnalyzer) hasBullishPattern(analysis ChartPatternAnalysis) bool {
	for _, p := range analysis.HeadAndShoulders {
		if p.Type == "inverse_head_and_shoulders" {
			return true
		}
	}
	for _, p := range analysis.DoubleTopsBottoms {
		if p.Type == "double_bottom" {
			return true
		}
	}
	for _, p := range analysis.Triangles {
		if p.BreakoutBias == "up" {
			return true
		}
	}
	for _, p := range analysis.Wedges {
		if p.Type == "falling_wedge" {
			return true
		}
	}
	for _, p := range analysis.FlagsPennants {
		if p.Direction == "bullish" {
			return true
		}
	}
	return false
}

func (g *GinieAnalyzer) hasBearishPattern(analysis ChartPatternAnalysis) bool {
	for _, p := range analysis.HeadAndShoulders {
		if p.Type == "head_and_shoulders" {
			return true
		}
	}
	for _, p := range analysis.DoubleTopsBottoms {
		if p.Type == "double_top" {
			return true
		}
	}
	for _, p := range analysis.Triangles {
		if p.BreakoutBias == "down" {
			return true
		}
	}
	for _, p := range analysis.Wedges {
		if p.Type == "rising_wedge" {
			return true
		}
	}
	for _, p := range analysis.FlagsPennants {
		if p.Direction == "bearish" {
			return true
		}
	}
	return false
}

func (g *GinieAnalyzer) isNearBreakout(analysis ChartPatternAnalysis, currentPrice float64) bool {
	// Check triangles - near apex
	for _, p := range analysis.Triangles {
		if p.Contraction > 70 {
			return true
		}
	}

	// Check wedges - near apex
	for _, p := range analysis.Wedges {
		// If pattern has been forming for a while, breakout is near
		if p.PatternWidth > 15 {
			return true
		}
	}

	// Check flags/pennants - consolidation should be tight
	for _, p := range analysis.FlagsPennants {
		range_pct := (p.ConsolidationHigh - p.ConsolidationLow) / p.ConsolidationLow * 100
		if range_pct < 2 && p.ConsolidationBars >= 5 {
			return true
		}
	}

	return false
}

func (g *GinieAnalyzer) calculatePatternScore(analysis ChartPatternAnalysis) float64 {
	score := 0.0

	// Strong reversal patterns get highest scores
	for _, p := range analysis.HeadAndShoulders {
		score += float64(g.patternStrengthScore(p.Strength)) * 10
	}

	for _, p := range analysis.DoubleTopsBottoms {
		score += float64(g.patternStrengthScore(p.Strength)) * 8
	}

	// Continuation patterns
	for _, p := range analysis.FlagsPennants {
		score += float64(g.patternStrengthScore(p.Strength)) * 7
	}

	// Consolidation patterns
	for _, p := range analysis.Triangles {
		score += float64(g.patternStrengthScore(p.Strength)) * 6
	}

	for _, p := range analysis.Wedges {
		score += float64(g.patternStrengthScore(p.Strength)) * 6
	}

	// Near breakout bonus
	if analysis.NearBreakout {
		score += 15
	}

	// Cap at 100
	if score > 100 {
		score = 100
	}

	return score
}

// Helper function for ternary operation
func ternary(condition bool, a, b float64) float64 {
	if condition {
		return a
	}
	return b
}

// Sort swing points by index
func sortSwingPointsByIndex(points []SwingPoint) {
	sort.Slice(points, func(i, j int) bool {
		return points[i].Index < points[j].Index
	})
}

// ============ MULTI-TIMEFRAME (MTF) ANALYSIS ============

// MTFAnalysisResult holds the result of multi-timeframe trend analysis
type MTFAnalysisResult struct {
	Enabled             bool      `json:"mtf_enabled"`
	Mode                string    `json:"mode"`
	TrendBias           string    `json:"trend_bias"`            // LONG, SHORT, NEUTRAL
	WeightedStrength    float64   `json:"weighted_strength"`     // Combined weighted strength (0-100)
	Consensus           int       `json:"consensus"`             // Number of TFs that agree
	TrendAligned        bool      `json:"trend_aligned"`         // Meets min consensus and strength
	TrendStable         bool      `json:"trend_stable"`          // No flips in recent candles
	StabilityReason     string    `json:"stability_reason"`      // Explanation of stability check
	AlignmentReason     string    `json:"alignment_reason"`      // Explanation of alignment
	PrimaryTrend        string    `json:"primary_trend"`         // Primary TF trend bias
	PrimaryStrength     float64   `json:"primary_strength"`      // Primary TF strength
	SecondaryTrend      string    `json:"secondary_trend"`       // Secondary TF trend bias
	SecondaryStrength   float64   `json:"secondary_strength"`    // Secondary TF strength
	TertiaryTrend       string    `json:"tertiary_trend"`        // Tertiary TF trend bias
	TertiaryStrength    float64   `json:"tertiary_strength"`     // Tertiary TF strength
	AnalyzedAt          time.Time `json:"analyzed_at"`
}

// GetMTFConfigForMode returns the MTF config for a given trading mode
func GetMTFConfigForMode(mode GinieTradingMode) *ModeMTFConfig {
	settings := GetSettingsManager().GetCurrentSettings()

	// Get mode config from ModeConfigs
	modeStr := string(mode)
	if settings.ModeConfigs != nil {
		if cfg, ok := settings.ModeConfigs[modeStr]; ok && cfg.MTF != nil {
			return cfg.MTF
		}
	}

	// Return default MTF config based on mode
	switch mode {
	case GinieModeUltraFast:
		return &ModeMTFConfig{
			Enabled:             true,
			PrimaryTimeframe:    "5m",
			PrimaryWeight:       0.40,
			SecondaryTimeframe:  "3m",
			SecondaryWeight:     0.35,
			TertiaryTimeframe:   "1m",
			TertiaryWeight:      0.25,
			MinConsensus:        2,
			MinWeightedStrength: 65.0,
			TrendStabilityCheck: true,
		}
	case GinieModeScalp:
		return &ModeMTFConfig{
			Enabled:             true,
			PrimaryTimeframe:    "15m",
			PrimaryWeight:       0.50, // Increased - primary TF matters most for scalp
			SecondaryTimeframe:  "5m",
			SecondaryWeight:     0.30,
			TertiaryTimeframe:   "1m",
			TertiaryWeight:      0.20,
			MinConsensus:        1,    // Lowered - if primary TF aligns, that's enough for scalp
			MinWeightedStrength: 40.0, // Lowered - allow entries with moderate strength
			TrendStabilityCheck: false, // Disabled - too strict, blocks valid trades
		}
	case GinieModeSwing:
		return &ModeMTFConfig{
			Enabled:             true,
			PrimaryTimeframe:    "4h",
			PrimaryWeight:       0.50, // Increased - primary TF matters most
			SecondaryTimeframe:  "1h",
			SecondaryWeight:     0.30,
			TertiaryTimeframe:   "15m",
			TertiaryWeight:      0.20,
			MinConsensus:        1,    // Lowered - if primary TF aligns, that's enough
			MinWeightedStrength: 45.0, // Lowered - allow entries with moderate strength
			TrendStabilityCheck: false, // Disabled - too strict, blocks valid trades
		}
	case GinieModePosition:
		return &ModeMTFConfig{
			Enabled:             true,
			PrimaryTimeframe:    "1d",
			PrimaryWeight:       0.50,
			SecondaryTimeframe:  "4h",
			SecondaryWeight:     0.30,
			TertiaryTimeframe:   "1h",
			TertiaryWeight:      0.20,
			MinConsensus:        2,
			MinWeightedStrength: 50.0,
			TrendStabilityCheck: true,
		}
	default:
		return nil
	}
}

// GetDynamicAIExitConfigForMode returns the Dynamic AI Exit config for a given trading mode
func GetDynamicAIExitConfigForMode(mode GinieTradingMode) *ModeDynamicAIExitConfig {
	settings := GetSettingsManager().GetCurrentSettings()

	// Get mode config from ModeConfigs
	modeStr := string(mode)
	if settings.ModeConfigs != nil {
		if cfg, ok := settings.ModeConfigs[modeStr]; ok && cfg.DynamicAIExit != nil {
			return cfg.DynamicAIExit
		}
	}

	// Return default Dynamic AI Exit config based on mode
	switch mode {
	case GinieModeUltraFast:
		return &ModeDynamicAIExitConfig{
			Enabled:           true,
			MinHoldBeforeAIMS: 3000,    // 3 seconds
			AICheckIntervalMS: 5000,    // 5 seconds
			UseLLMForLoss:     true,
			UseLLMForProfit:   false,
			MaxHoldTimeMS:     0,       // No max
		}
	case GinieModeScalp:
		return &ModeDynamicAIExitConfig{
			Enabled:           true,
			MinHoldBeforeAIMS: 10000,   // 10 seconds
			AICheckIntervalMS: 30000,   // 30 seconds
			UseLLMForLoss:     true,
			UseLLMForProfit:   true,
			MaxHoldTimeMS:     14400000, // 4 hours
		}
	case GinieModeSwing:
		return &ModeDynamicAIExitConfig{
			Enabled:           true,
			MinHoldBeforeAIMS: 60000,   // 1 minute
			AICheckIntervalMS: 300000,  // 5 minutes
			UseLLMForLoss:     true,
			UseLLMForProfit:   true,
			MaxHoldTimeMS:     259200000, // 3 days
		}
	case GinieModePosition:
		return &ModeDynamicAIExitConfig{
			Enabled:           true,
			MinHoldBeforeAIMS: 300000,  // 5 minutes
			AICheckIntervalMS: 1800000, // 30 minutes
			UseLLMForLoss:     true,
			UseLLMForProfit:   true,
			MaxHoldTimeMS:     1209600000, // 14 days
		}
	default:
		return nil
	}
}

// AnalyzeMTF performs multi-timeframe analysis for a given symbol and mode
// Returns MTFAnalysisResult with trend alignment and stability information
func (g *GinieAnalyzer) AnalyzeMTF(symbol string, mode GinieTradingMode) *MTFAnalysisResult {
	result := &MTFAnalysisResult{
		Mode:       string(mode),
		AnalyzedAt: time.Now(),
	}

	// Get MTF config for this mode
	mtfConfig := GetMTFConfigForMode(mode)
	if mtfConfig == nil || !mtfConfig.Enabled {
		result.Enabled = false
		result.TrendAligned = true // Don't block if disabled
		result.TrendStable = true
		result.AlignmentReason = "MTF analysis disabled for this mode"
		return result
	}
	result.Enabled = true

	// Fetch klines for all three timeframes in parallel
	type tfResult struct {
		tf       string
		bias     string
		strength float64
		klines   []binance.Kline
		err      error
	}

	results := make(chan tfResult, 3)
	timeframes := []string{mtfConfig.PrimaryTimeframe, mtfConfig.SecondaryTimeframe, mtfConfig.TertiaryTimeframe}

	for _, tf := range timeframes {
		go func(timeframe string) {
			klines, err := g.futuresClient.GetFuturesKlines(symbol, timeframe, 10)
			if err != nil || len(klines) < 3 {
				results <- tfResult{tf: timeframe, err: err}
				return
			}

			// Calculate trend bias and strength from price movement
			closeNow := klines[len(klines)-1].Close
			closePrev := klines[len(klines)-2].Close
			priceDiffPct := ((closeNow - closePrev) / closePrev) * 100.0

			var bias string
			var strength float64

			// Determine thresholds based on timeframe (higher TF = less sensitive)
			var strongThreshold, weakThreshold float64
			switch timeframe {
			case "1d":
				strongThreshold, weakThreshold = 1.0, 0.3
			case "4h":
				strongThreshold, weakThreshold = 0.5, 0.15
			case "1h":
				strongThreshold, weakThreshold = 0.3, 0.1
			case "15m":
				strongThreshold, weakThreshold = 0.2, 0.06
			case "5m":
				strongThreshold, weakThreshold = 0.15, 0.05
			case "3m":
				strongThreshold, weakThreshold = 0.12, 0.04
			case "1m":
				strongThreshold, weakThreshold = 0.10, 0.03
			default:
				strongThreshold, weakThreshold = 0.2, 0.05
			}

			if priceDiffPct >= strongThreshold {
				bias, strength = "LONG", 80
			} else if priceDiffPct >= weakThreshold {
				bias, strength = "LONG", 55
			} else if priceDiffPct <= -strongThreshold {
				bias, strength = "SHORT", 80
			} else if priceDiffPct <= -weakThreshold {
				bias, strength = "SHORT", 55
			} else {
				bias, strength = "NEUTRAL", 40
			}

			results <- tfResult{tf: timeframe, bias: bias, strength: strength, klines: klines}
		}(tf)
	}

	// Collect results
	tfData := make(map[string]tfResult)
	for i := 0; i < 3; i++ {
		r := <-results
		if r.err == nil {
			tfData[r.tf] = r
		}
	}

	// Get weights from config and normalize if needed
	weights := map[string]float64{
		mtfConfig.PrimaryTimeframe:   mtfConfig.PrimaryWeight,
		mtfConfig.SecondaryTimeframe: mtfConfig.SecondaryWeight,
		mtfConfig.TertiaryTimeframe:  mtfConfig.TertiaryWeight,
	}

	totalWeight := mtfConfig.PrimaryWeight + mtfConfig.SecondaryWeight + mtfConfig.TertiaryWeight
	if totalWeight > 0 && totalWeight != 1.0 {
		for k := range weights {
			weights[k] /= totalWeight
		}
	}

	// Calculate weighted scores and consensus
	var longScore, shortScore float64
	var longConsensus, shortConsensus int
	var alignmentDetails []string

	for tf, data := range tfData {
		weight := weights[tf]

		// Store individual TF results
		if tf == mtfConfig.PrimaryTimeframe {
			result.PrimaryTrend = data.bias
			result.PrimaryStrength = data.strength
		} else if tf == mtfConfig.SecondaryTimeframe {
			result.SecondaryTrend = data.bias
			result.SecondaryStrength = data.strength
		} else if tf == mtfConfig.TertiaryTimeframe {
			result.TertiaryTrend = data.bias
			result.TertiaryStrength = data.strength
		}

		if data.bias == "LONG" {
			longScore += weight * data.strength
			if data.strength >= 50 {
				longConsensus++
			}
			alignmentDetails = append(alignmentDetails, tf+":LONG")
		} else if data.bias == "SHORT" {
			shortScore += weight * data.strength
			if data.strength >= 50 {
				shortConsensus++
			}
			alignmentDetails = append(alignmentDetails, tf+":SHORT")
		} else {
			alignmentDetails = append(alignmentDetails, tf+":NEUTRAL")
		}
	}

	// Determine final bias based on weighted scores and consensus
	minConsensus := mtfConfig.MinConsensus
	minStrength := mtfConfig.MinWeightedStrength

	// RELAXED: Use >= instead of > for score comparison, and allow equal scores to pass
	if longScore >= shortScore && longConsensus >= minConsensus && longScore >= minStrength {
		result.TrendBias = "LONG"
		result.WeightedStrength = longScore
		result.Consensus = longConsensus
		result.TrendAligned = true
		result.AlignmentReason = "MTF LONG consensus"
	} else if shortScore >= longScore && shortConsensus >= minConsensus && shortScore >= minStrength {
		result.TrendBias = "SHORT"
		result.WeightedStrength = shortScore
		result.Consensus = shortConsensus
		result.TrendAligned = true
		result.AlignmentReason = "MTF SHORT consensus"
	} else if longScore >= minStrength || shortScore >= minStrength {
		// Either direction has sufficient strength - allow trade (don't block on mixed signals)
		if longScore >= shortScore {
			result.TrendBias = "LONG"
			result.WeightedStrength = longScore
			result.Consensus = longConsensus
		} else {
			result.TrendBias = "SHORT"
			result.WeightedStrength = shortScore
			result.Consensus = shortConsensus
		}
		result.TrendAligned = true // ALLOW trade when strength is sufficient
		result.AlignmentReason = "MTF mixed but sufficient strength"
	} else {
		// No clear consensus and insufficient strength
		if longScore > shortScore {
			result.TrendBias = "LONG"
			result.WeightedStrength = longScore
			result.Consensus = longConsensus
		} else if shortScore > longScore {
			result.TrendBias = "SHORT"
			result.WeightedStrength = shortScore
			result.Consensus = shortConsensus
		} else {
			result.TrendBias = "NEUTRAL"
			result.WeightedStrength = 40
			result.Consensus = 0
		}
		result.TrendAligned = false
		result.AlignmentReason = "Weak consensus and insufficient strength"
	}

	// Trend stability check: ensure trend hasn't flipped in last 3 candles
	if mtfConfig.TrendStabilityCheck && result.TrendAligned {
		// Use the tertiary (shortest) timeframe for stability check
		tertiaryTF := mtfConfig.TertiaryTimeframe
		if data, ok := tfData[tertiaryTF]; ok && len(data.klines) >= 4 {
			klines := data.klines
			flipCount := 0
			var directions []string

			// Check last 3 candle directions
			for i := len(klines) - 3; i < len(klines); i++ {
				k := klines[i]
				if k.Close > k.Open {
					directions = append(directions, "↑")
				} else if k.Close < k.Open {
					directions = append(directions, "↓")
				} else {
					directions = append(directions, "→")
				}
			}

			// Count direction changes (flips)
			for i := 1; i < len(directions); i++ {
				if directions[i] != directions[i-1] && directions[i] != "→" && directions[i-1] != "→" {
					flipCount++
				}
			}

			result.TrendStable = flipCount == 0
			if !result.TrendStable {
				result.TrendAligned = false
				result.StabilityReason = "Trend flipped in last 3 candles"
			} else {
				result.StabilityReason = "Stable trend"
			}
		} else {
			result.TrendStable = true
			result.StabilityReason = "Insufficient data for stability check"
		}
	} else if !mtfConfig.TrendStabilityCheck {
		result.TrendStable = true
		result.StabilityReason = "Stability check disabled"
	}

	return result
}
