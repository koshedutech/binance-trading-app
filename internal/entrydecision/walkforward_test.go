package entrydecision

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"testing"
	"time"
)

// ==============================================================================
// WALK-FORWARD SIMULATION TEST
// Feb 11, 2026 - 3m candles - 10 symbols - Volume-Match Entry Logic
// ==============================================================================
//
// Run with: docker exec binance-trading-bot-dev go test -run TestWalkForwardFeb11 -v -timeout 120s ./internal/entrydecision/

// SimPosition represents an open simulated position
type SimPosition struct {
	Symbol     string
	Direction  string // "long" or "short"
	EntryPrice float64
	SLPrice    float64
	TPPrice    float64
	Quantity   float64
	Capital1x  float64 // Pre-leverage capital used
	EntryTime  time.Time
	CloseTime  time.Time
	ClosePrice float64
	PnL        float64
	Result     string // "TP", "SL", "OPEN"
	RefVolume  float64
	EntryVol   float64
}

// SimConfig holds simulation parameters
type SimConfig struct {
	StartingEquity  float64
	MaxConcurrent   int
	Leverage        int
	MinNotional     float64
	RRRatio         float64
	Symbols         []string
	Timeframe       string
	StartTime       time.Time
	EndTime         time.Time
}

func TestWalkForwardFeb11(t *testing.T) {
	config := SimConfig{
		StartingEquity: 10.0,
		MaxConcurrent:  4,
		Leverage:       10,
		MinNotional:    5.0,
		RRRatio:        4.0,
		Symbols: []string{
			"BTCUSDT", "ETHUSDT", "SOLUSDT", "DOGEUSDT", "XRPUSDT",
			"ADAUSDT", "LINKUSDT", "AVAXUSDT", "BNBUSDT",
		},
		Timeframe: "1m",
		StartTime: time.Date(2026, 2, 11, 0, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC),
	}

	// Pattern matcher config matching user's settings
	patternConfig := &PatternMatcherConfig{
		Direction:                   "both",
		MinVolumeSpikeMultiplier:    3.0,
		LookbackPeriod:              5,
		MinConsolidationCandles:     10,
		MaxConsolidationCandles:     999,
		ConsolidationRangeTolerance: 0.01,
		BreakoutVolumeSurge:         1.0, // Match reference candle volume
		PatternExpirationMinutes:    60,
		ReadyExpirationSeconds:      300, // Give more time in simulation
		RequirePreTrendDown:         false,
		MinReferenceBodyRatio:       0.0,
		RequireEntryBreakout:        false,
		PreferredHourUTC:            -1,
		MaxSLPercent:                1.5,
	}

	t.Logf("=== WALK-FORWARD SIMULATION ===")
	t.Logf("Date: %s", config.StartTime.Format("2006-01-02"))
	t.Logf("Timeframe: %s", config.Timeframe)
	t.Logf("Symbols: %v", config.Symbols)
	t.Logf("Starting Equity: $%.2f", config.StartingEquity)
	t.Logf("Max Concurrent: %d", config.MaxConcurrent)
	t.Logf("Leverage: %dx", config.Leverage)
	t.Logf("R:R Ratio: 1:%.0f", config.RRRatio)
	t.Logf("Min Consolidation Candles: %d", patternConfig.MinConsolidationCandles)
	t.Logf("Breakout Volume Surge: %.1f (vs reference candle)", patternConfig.BreakoutVolumeSurge)
	t.Logf("Volume Spike Threshold: %.1fx", patternConfig.MinVolumeSpikeMultiplier)
	t.Logf("")

	// Step 1: Fetch candles for all symbols
	// We need extra candles before the simulation date for lookback
	fetchStart := config.StartTime.Add(-2 * time.Hour) // 2 hours of pre-data for lookback
	allCandles := make(map[string][]Candle)

	for _, symbol := range config.Symbols {
		candles, err := fetchBinanceFuturesKlines(symbol, config.Timeframe, fetchStart, config.EndTime)
		if err != nil {
			t.Logf("WARNING: Failed to fetch %s: %v - skipping", symbol, err)
			continue
		}
		allCandles[symbol] = candles
		t.Logf("Fetched %d candles for %s (from %s to %s)",
			len(candles), symbol,
			candles[0].OpenTime.Format("15:04"),
			candles[len(candles)-1].OpenTime.Format("15:04"))
	}

	if len(allCandles) == 0 {
		t.Fatal("No candle data fetched - cannot run simulation")
	}

	// Step 2: Build time-sorted candle timeline
	// Collect all unique candle close times
	timeSet := make(map[int64]bool)
	for _, candles := range allCandles {
		for _, c := range candles {
			if !c.Time.Before(config.StartTime) && c.Time.Before(config.EndTime) {
				timeSet[c.Time.Unix()] = true
			}
		}
	}

	times := make([]int64, 0, len(timeSet))
	for ts := range timeSet {
		times = append(times, ts)
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })

	t.Logf("\nSimulation timeline: %d candle closes from %s to %s",
		len(times),
		time.Unix(times[0], 0).UTC().Format("15:04"),
		time.Unix(times[len(times)-1], 0).UTC().Format("15:04"))

	// Step 3: Run simulation
	matcher := NewVolumeImbalancePatternMatcher(patternConfig)
	equity := config.StartingEquity
	var positions []*SimPosition
	var closedPositions []*SimPosition
	tradeLog := []string{}

	for _, ts := range times {
		currentTime := time.Unix(ts, 0).UTC()

		// Process each symbol at this time
		for symbol, allSymCandles := range allCandles {
			// Build candle slice up to current time (inclusive)
			var candlesUpTo []Candle
			for _, c := range allSymCandles {
				if !c.Time.After(currentTime) {
					candlesUpTo = append(candlesUpTo, c)
				}
			}

			if len(candlesUpTo) < patternConfig.LookbackPeriod+5 {
				continue
			}

			// Run pattern matching
			coinMatch := matcher.MatchPattern(symbol, "scalp", config.Timeframe, candlesUpTo)
			if coinMatch == nil {
				continue
			}

			// Check if breakout detected (status = ready)
			if coinMatch.Status != PatternStatusReady {
				continue
			}

			// Get the pattern state for entry details
			state := matcher.states[matcher.patternKey(symbol, "scalp", config.Timeframe)]
			if state == nil || state.BreakoutCandle == nil {
				continue
			}

			// Check if we already have this symbol open
			alreadyOpen := false
			for _, p := range positions {
				if p.Symbol == symbol {
					alreadyOpen = true
					break
				}
			}
			if alreadyOpen {
				continue
			}

			// Check concurrent position limit
			if len(positions) >= config.MaxConcurrent {
				tradeLog = append(tradeLog, fmt.Sprintf("%s [BLOCKED] %s %s breakout - max concurrent reached (%d/%d)",
					currentTime.Format("15:04"), symbol, state.Direction, len(positions), config.MaxConcurrent))
				// Reset pattern so it can detect again
				matcher.ResetPattern(symbol, "scalp", config.Timeframe)
				continue
			}

			// Calculate position size
			remainingSlots := config.MaxConcurrent - len(positions)
			capital1x := equity / float64(remainingSlots)
			positionSizeUSD := capital1x * float64(config.Leverage)
			entryPrice := state.EntryPrice

			// Check min notional
			if positionSizeUSD < config.MinNotional || entryPrice <= 0 {
				tradeLog = append(tradeLog, fmt.Sprintf("%s [SKIP] %s notional $%.2f < $%.2f min",
					currentTime.Format("15:04"), symbol, positionSizeUSD, config.MinNotional))
				matcher.ResetPattern(symbol, "scalp", config.Timeframe)
				continue
			}

			// Calculate SL and TP
			var slPrice, tpPrice, riskAmt float64
			if state.Direction == "long" {
				slBase := state.ConsolidationLow
				if slBase <= 0 {
					slBase = state.ReferenceCandle.Low
				}
				slPrice = slBase * 0.999
				riskAmt = entryPrice - slPrice
				tpPrice = entryPrice + (riskAmt * config.RRRatio)
			} else {
				slBase := state.ConsolidationHigh
				if slBase <= 0 {
					slBase = state.ReferenceCandle.High
				}
				slPrice = slBase * 1.001
				riskAmt = slPrice - entryPrice
				tpPrice = entryPrice - (riskAmt * config.RRRatio)
			}

			// Validate SL %
			slPct := (riskAmt / entryPrice) * 100
			if slPct > patternConfig.MaxSLPercent {
				tradeLog = append(tradeLog, fmt.Sprintf("%s [REJECT] %s SL %.2f%% > max %.2f%%",
					currentTime.Format("15:04"), symbol, slPct, patternConfig.MaxSLPercent))
				matcher.ResetPattern(symbol, "scalp", config.Timeframe)
				continue
			}

			quantity := positionSizeUSD / entryPrice

			refVol := 0.0
			if state.ReferenceCandle != nil {
				refVol = state.ReferenceCandle.Volume
			}

			pos := &SimPosition{
				Symbol:     symbol,
				Direction:  state.Direction,
				EntryPrice: entryPrice,
				SLPrice:    slPrice,
				TPPrice:    tpPrice,
				Quantity:   quantity,
				Capital1x:  capital1x,
				EntryTime:  currentTime,
				Result:     "OPEN",
				RefVolume:  refVol,
				EntryVol:   state.BreakoutCandle.Volume,
			}
			positions = append(positions, pos)

			tradeLog = append(tradeLog, fmt.Sprintf("%s [ENTRY] %s %s @ %.4f | SL=%.4f TP=%.4f | Risk=%.2f%% | Cap=$%.2f | Eq=$%.2f | Vol=%.0f/%.0f",
				currentTime.Format("15:04"), symbol, state.Direction, entryPrice,
				slPrice, tpPrice, slPct, capital1x, equity, state.BreakoutCandle.Volume, refVol))

			// Suppress pattern for this symbol (position open)
			matcher.SetPatternPositionRunning(symbol, "scalp", config.Timeframe)
		}

		// Check all open positions for SL/TP hit
		var stillOpen []*SimPosition
		for _, pos := range positions {
			hit := false
			// Check current candle for this position's symbol
			if symCandles, ok := allCandles[pos.Symbol]; ok {
				for _, c := range symCandles {
					if c.Time.Unix() != ts {
						continue
					}
					// Check SL/TP
					if pos.Direction == "long" {
						if c.Low <= pos.SLPrice {
							// SL hit
							pos.ClosePrice = pos.SLPrice
							pos.CloseTime = currentTime
							pos.PnL = (pos.ClosePrice - pos.EntryPrice) * pos.Quantity
							pos.Result = "SL"
							hit = true
						} else if c.High >= pos.TPPrice {
							// TP hit
							pos.ClosePrice = pos.TPPrice
							pos.CloseTime = currentTime
							pos.PnL = (pos.ClosePrice - pos.EntryPrice) * pos.Quantity
							pos.Result = "TP"
							hit = true
						}
					} else {
						if c.High >= pos.SLPrice {
							// SL hit (SHORT)
							pos.ClosePrice = pos.SLPrice
							pos.CloseTime = currentTime
							pos.PnL = (pos.EntryPrice - pos.ClosePrice) * pos.Quantity
							pos.Result = "SL"
							hit = true
						} else if c.Low <= pos.TPPrice {
							// TP hit (SHORT)
							pos.ClosePrice = pos.TPPrice
							pos.CloseTime = currentTime
							pos.PnL = (pos.EntryPrice - pos.ClosePrice) * pos.Quantity
							pos.Result = "TP"
							hit = true
						}
					}
				}
			}

			if hit {
				// Update equity
				equity += pos.PnL
				closedPositions = append(closedPositions, pos)

				duration := pos.CloseTime.Sub(pos.EntryTime)
				pnlPct := (pos.PnL / pos.Capital1x) * 100

				tradeLog = append(tradeLog, fmt.Sprintf("%s [%s] %s %s @ %.4f → %.4f | PnL=$%.4f (%.2f%%) | Duration=%s | Eq=$%.4f",
					currentTime.Format("15:04"), pos.Result, pos.Symbol, pos.Direction,
					pos.EntryPrice, pos.ClosePrice, pos.PnL, pnlPct, formatDuration(duration), equity))

				// Reset pattern for this symbol
				matcher.ResetPattern(pos.Symbol, "scalp", config.Timeframe)
			} else {
				stillOpen = append(stillOpen, pos)
			}
		}
		positions = stillOpen
	}

	// Close remaining open positions at last candle close
	for _, pos := range positions {
		if symCandles, ok := allCandles[pos.Symbol]; ok && len(symCandles) > 0 {
			lastCandle := symCandles[len(symCandles)-1]
			pos.ClosePrice = lastCandle.Close
			pos.CloseTime = lastCandle.Time
			if pos.Direction == "long" {
				pos.PnL = (pos.ClosePrice - pos.EntryPrice) * pos.Quantity
			} else {
				pos.PnL = (pos.EntryPrice - pos.ClosePrice) * pos.Quantity
			}
			pos.Result = "OPEN(EOD)"
			equity += pos.PnL
			closedPositions = append(closedPositions, pos)

			pnlPct := (pos.PnL / pos.Capital1x) * 100
			tradeLog = append(tradeLog, fmt.Sprintf("EOD   [CLOSE] %s %s @ %.4f → %.4f | PnL=$%.4f (%.2f%%) | Eq=$%.4f",
				pos.Symbol, pos.Direction, pos.EntryPrice, pos.ClosePrice, pos.PnL, pnlPct, equity))
		}
	}

	// === RESULTS ===
	t.Logf("\n" + "=" + "=================================================================")
	t.Logf("TRADE LOG")
	t.Logf("==================================================================")
	for _, log := range tradeLog {
		t.Log(log)
	}

	t.Logf("\n" + "=" + "=================================================================")
	t.Logf("TRADE SUMMARY")
	t.Logf("==================================================================")

	wins, losses, opens := 0, 0, 0
	totalPnL := 0.0
	var winPnL, lossPnL float64
	for _, pos := range closedPositions {
		totalPnL += pos.PnL
		switch pos.Result {
		case "TP":
			wins++
			winPnL += pos.PnL
		case "SL":
			losses++
			lossPnL += pos.PnL
		default:
			opens++
		}
	}

	t.Logf("")
	t.Logf("Total Trades:     %d", len(closedPositions))
	t.Logf("  Wins (TP):      %d", wins)
	t.Logf("  Losses (SL):    %d", losses)
	t.Logf("  Still Open:     %d", opens)
	if wins+losses > 0 {
		t.Logf("  Win Rate:       %.1f%%", float64(wins)/float64(wins+losses)*100)
	}
	t.Logf("")
	t.Logf("Starting Equity:  $%.4f", config.StartingEquity)
	t.Logf("Final Equity:     $%.4f", equity)
	t.Logf("Total PnL:        $%.4f (%.2f%%)", totalPnL, (totalPnL/config.StartingEquity)*100)
	t.Logf("  Win PnL:        $%.4f", winPnL)
	t.Logf("  Loss PnL:       $%.4f", lossPnL)
	if losses > 0 {
		t.Logf("  Avg Win:        $%.4f", winPnL/math.Max(float64(wins), 1))
		t.Logf("  Avg Loss:       $%.4f", lossPnL/float64(losses))
	}

	// Per-symbol breakdown
	t.Logf("\n" + "=" + "=================================================================")
	t.Logf("PER-SYMBOL BREAKDOWN")
	t.Logf("==================================================================")
	symbolPnL := make(map[string]float64)
	symbolTrades := make(map[string]int)
	symbolWins := make(map[string]int)
	for _, pos := range closedPositions {
		symbolPnL[pos.Symbol] += pos.PnL
		symbolTrades[pos.Symbol]++
		if pos.Result == "TP" {
			symbolWins[pos.Symbol]++
		}
	}

	for _, sym := range config.Symbols {
		trades := symbolTrades[sym]
		if trades == 0 {
			t.Logf("  %-12s  No trades", sym)
			continue
		}
		pnl := symbolPnL[sym]
		wr := float64(symbolWins[sym]) / float64(trades) * 100
		t.Logf("  %-12s  %d trades, WR=%.0f%%, PnL=$%.4f", sym, trades, wr, pnl)
	}

	t.Logf("\n" + "=" + "=================================================================")
}

// fetchBinanceFuturesKlines fetches klines from Binance Futures public API
func fetchBinanceFuturesKlines(symbol, interval string, startTime, endTime time.Time) ([]Candle, error) {
	startMs := startTime.UnixMilli()
	endMs := endTime.UnixMilli()

	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/klines?symbol=%s&interval=%s&startTime=%d&endTime=%d&limit=1500",
		symbol, interval, startMs, endMs)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body failed: %w", err)
	}

	// Parse the response - Binance returns array of arrays
	var rawKlines [][]json.RawMessage
	if err := json.Unmarshal(body, &rawKlines); err != nil {
		return nil, fmt.Errorf("JSON parse failed: %w", err)
	}

	candles := make([]Candle, 0, len(rawKlines))
	for _, raw := range rawKlines {
		if len(raw) < 11 {
			continue
		}

		var openTimeMs int64
		json.Unmarshal(raw[0], &openTimeMs)

		var closeTimeMs int64
		json.Unmarshal(raw[6], &closeTimeMs)

		open := parseJsonFloat(raw[1])
		high := parseJsonFloat(raw[2])
		low := parseJsonFloat(raw[3])
		closeP := parseJsonFloat(raw[4])
		volume := parseJsonFloat(raw[5])
		takerBuyVol := parseJsonFloat(raw[9])

		candles = append(candles, Candle{
			OpenTime:       time.UnixMilli(openTimeMs).UTC(),
			Time:           time.UnixMilli(closeTimeMs).UTC(),
			Open:           open,
			High:           high,
			Low:            low,
			Close:          closeP,
			Volume:         volume,
			TakerBuyVolume: takerBuyVol,
		})
	}

	// Rate limit: brief pause between requests
	time.Sleep(200 * time.Millisecond)

	return candles, nil
}

func parseJsonFloat(raw json.RawMessage) float64 {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}
	var f float64
	json.Unmarshal(raw, &f)
	return f
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
