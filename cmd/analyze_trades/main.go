package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"binance-trading-bot/internal/binance"

	"github.com/joho/godotenv"
)

type SymbolStats struct {
	Symbol        string
	TotalTrades   int
	WinningTrades int
	LosingTrades  int
	TotalPnL      float64
	TotalWins     float64
	TotalLosses   float64
	WinRate       float64
	AvgPnL        float64
	Commission    float64
}

func main() {
	// Get the executable directory and try loading .env from there
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)

	// Try multiple locations for .env
	godotenv.Load()
	godotenv.Load(".env")
	godotenv.Load(filepath.Join(exeDir, ".env"))
	godotenv.Load(filepath.Join(exeDir, "..", "..", ".env"))
	godotenv.Load("D:\\Apps\\binance-trading-bot\\.env")

	apiKey := os.Getenv("BINANCE_API_KEY")
	apiSecret := os.Getenv("BINANCE_SECRET_KEY") // Note: .env uses SECRET_KEY not API_SECRET
	testnet := os.Getenv("BINANCE_TESTNET") == "true" || os.Getenv("FUTURES_TESTNET") == "true"

	if apiKey == "" || apiSecret == "" {
		fmt.Println("❌ BINANCE_API_KEY and BINANCE_SECRET_KEY required in .env")
		fmt.Printf("   API Key found: %v, Secret found: %v\n", apiKey != "", apiSecret != "")
		os.Exit(1)
	}

	fmt.Println("=" + string(make([]byte, 79)))
	fmt.Println("📊 BINANCE FUTURES TRADE HISTORY ANALYSIS")
	fmt.Println("=" + string(make([]byte, 79)))

	if testnet {
		fmt.Println("⚠️  Running on TESTNET")
	} else {
		fmt.Println("🔴 Running on LIVE account")
	}

	// Create futures client
	client := binance.NewFuturesClient(apiKey, apiSecret, testnet)

	// Get account info first
	account, err := client.GetFuturesAccountInfo()
	if err != nil {
		fmt.Printf("❌ Failed to get account info: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n💰 Account Balance: $%.2f USDT\n", account.TotalWalletBalance)
	fmt.Printf("📈 Unrealized PnL: $%.2f\n", account.TotalUnrealizedProfit)

	// Get current positions
	positions, err := client.GetPositions()
	if err != nil {
		fmt.Printf("❌ Failed to get positions: %v\n", err)
	} else {
		openCount := 0
		for _, p := range positions {
			if p.PositionAmt != 0 {
				openCount++
			}
		}
		fmt.Printf("📍 Open Positions: %d\n", openCount)
	}

	// Get trade history per symbol
	fmt.Println("\n🔄 Fetching trade history...")

	// Common futures symbols to check
	symbols := []string{
		"BTCUSDT", "ETHUSDT", "BNBUSDT", "XRPUSDT", "SOLUSDT",
		"DOGEUSDT", "ADAUSDT", "AVAXUSDT", "LINKUSDT", "DOTUSDT",
		"MATICUSDT", "LTCUSDT", "BCHUSDT", "ATOMUSDT", "NEARUSDT",
		"FILUSDT", "APTUSDT", "ARBUSDT", "OPUSDT", "SUIUSDT",
		"PEPEUSDT", "SHIBUSDT", "WIFUSDT", "BONKUSDT", "FLOKIUSDT",
		"TRXUSDT", "UNIUSDT", "AAVEUSDT", "MKRUSDT", "INJUSDT",
	}

	// Add symbols from current positions
	if positions != nil {
		for _, p := range positions {
			if p.PositionAmt != 0 || p.Symbol != "" {
				found := false
				for _, s := range symbols {
					if s == p.Symbol {
						found = true
						break
					}
				}
				if !found && p.Symbol != "" {
					symbols = append(symbols, p.Symbol)
				}
			}
		}
	}

	symbolStats := make(map[string]*SymbolStats)
	totalChecked := 0

	for _, symbol := range symbols {
		trades, err := client.GetTradeHistory(symbol, 500)
		if err != nil {
			continue
		}
		totalChecked++

		if len(trades) == 0 {
			continue
		}

		if _, exists := symbolStats[symbol]; !exists {
			symbolStats[symbol] = &SymbolStats{Symbol: symbol}
		}
		stats := symbolStats[symbol]

		for _, t := range trades {
			stats.TotalTrades++
			stats.TotalPnL += t.RealizedPnl
			stats.Commission += t.Commission

			if t.RealizedPnl > 0 {
				stats.WinningTrades++
				stats.TotalWins += t.RealizedPnl
			} else if t.RealizedPnl < 0 {
				stats.LosingTrades++
				stats.TotalLosses += t.RealizedPnl
			}
		}
	}

	fmt.Printf("   Checked %d symbols\n", totalChecked)

	if len(symbolStats) == 0 {
		fmt.Println("\n❌ No trade history found")
		return
	}

	// Calculate stats and sort
	var sortedStats []*SymbolStats
	for _, s := range symbolStats {
		if s.TotalTrades > 0 {
			s.WinRate = float64(s.WinningTrades) / float64(s.TotalTrades) * 100
			s.AvgPnL = s.TotalPnL / float64(s.TotalTrades)
		}
		if s.TotalTrades > 0 { // Only include symbols with trades
			sortedStats = append(sortedStats, s)
		}
	}

	sort.Slice(sortedStats, func(i, j int) bool {
		return sortedStats[i].TotalPnL > sortedStats[j].TotalPnL
	})

	// Print table
	fmt.Println("\n" + "=" + string(make([]byte, 79)))
	fmt.Println("📈 TRADE PERFORMANCE BY SYMBOL")
	fmt.Println("=" + string(make([]byte, 79)))

	fmt.Println("┌──────────────┬────────┬─────────┬─────────┬──────────────┬──────────────┬──────────┐")
	fmt.Println("│ Symbol       │ Trades │ Winners │ Losers  │ Total PnL    │ Avg PnL      │ Win Rate │")
	fmt.Println("├──────────────┼────────┼─────────┼─────────┼──────────────┼──────────────┼──────────┤")

	var grandTotal float64
	var grandTrades, grandWins, grandLosses int
	var grandCommission float64

	for _, s := range sortedStats {
		emoji := "🟢"
		if s.TotalPnL < 0 {
			emoji = "🔴"
		}
		fmt.Printf("│ %s %-10s │ %6d │ %7d │ %7d │ %+12.2f │ %+12.2f │ %7.1f%% │\n",
			emoji, truncate(s.Symbol, 10),
			s.TotalTrades, s.WinningTrades, s.LosingTrades,
			s.TotalPnL, s.AvgPnL, s.WinRate)

		grandTotal += s.TotalPnL
		grandTrades += s.TotalTrades
		grandWins += s.WinningTrades
		grandLosses += s.LosingTrades
		grandCommission += s.Commission
	}

	fmt.Println("├──────────────┼────────┼─────────┼─────────┼──────────────┼──────────────┼──────────┤")
	grandWinRate := 0.0
	if grandTrades > 0 {
		grandWinRate = float64(grandWins) / float64(grandTrades) * 100
	}
	fmt.Printf("│ 📊 TOTAL     │ %6d │ %7d │ %7d │ %+12.2f │ %+12.2f │ %7.1f%% │\n",
		grandTrades, grandWins, grandLosses,
		grandTotal, grandTotal/float64(maxInt(grandTrades, 1)), grandWinRate)
	fmt.Println("└──────────────┴────────┴─────────┴─────────┴──────────────┴──────────────┴──────────┘")

	fmt.Printf("\n💸 Total Commission Paid: $%.2f\n", grandCommission)
	fmt.Printf("📊 Net PnL (after commission): $%.2f\n", grandTotal-grandCommission)

	// Show worst performers
	fmt.Println("\n" + "=" + string(make([]byte, 79)))
	fmt.Println("🔴 WORST PERFORMING SYMBOLS")
	fmt.Println("=" + string(make([]byte, 79)))

	worstCount := 0
	for i := len(sortedStats) - 1; i >= 0 && worstCount < 5; i-- {
		s := sortedStats[i]
		if s.TotalPnL < 0 {
			avgLoss := 0.0
			if s.LosingTrades > 0 {
				avgLoss = s.TotalLosses / float64(s.LosingTrades)
			}
			fmt.Printf("   🔴 %s: $%.2f total loss | %d losses | Avg loss: $%.2f | Win rate: %.1f%%\n",
				s.Symbol, s.TotalPnL, s.LosingTrades, avgLoss, s.WinRate)
			worstCount++
		}
	}

	// Show best performers
	fmt.Println("\n" + "=" + string(make([]byte, 79)))
	fmt.Println("🟢 BEST PERFORMING SYMBOLS")
	fmt.Println("=" + string(make([]byte, 79)))

	bestCount := 0
	for _, s := range sortedStats {
		if s.TotalPnL > 0 && bestCount < 5 {
			avgWin := 0.0
			if s.WinningTrades > 0 {
				avgWin = s.TotalWins / float64(s.WinningTrades)
			}
			fmt.Printf("   🟢 %s: $%.2f total profit | %d wins | Avg win: $%.2f | Win rate: %.1f%%\n",
				s.Symbol, s.TotalPnL, s.WinningTrades, avgWin, s.WinRate)
			bestCount++
		}
	}

	// Recommendations
	fmt.Println("\n" + "=" + string(make([]byte, 79)))
	fmt.Println("💡 INSIGHTS & RECOMMENDATIONS")
	fmt.Println("=" + string(make([]byte, 79)))

	if grandWinRate < 50 {
		fmt.Printf("\n   ⚠️  Overall win rate is %.1f%% - BELOW 50%%\n", grandWinRate)
		fmt.Println("   → Consider raising minimum confidence threshold")
		fmt.Println("   → Current Ginie aggressive: 55%, consider 60-65%")
	} else {
		fmt.Printf("\n   ✅ Overall win rate is %.1f%% - above 50%%\n", grandWinRate)
	}

	// Find symbols to blacklist
	fmt.Println("\n   🚫 BLACKLIST CANDIDATES (negative PnL + low win rate):")
	blacklistCount := 0
	for i := len(sortedStats) - 1; i >= 0; i-- {
		s := sortedStats[i]
		if s.TotalPnL < -20 && s.WinRate < 45 && s.TotalTrades >= 3 {
			fmt.Printf("      - %s (PnL: $%.2f, Win rate: %.1f%%, Trades: %d)\n",
				s.Symbol, s.TotalPnL, s.WinRate, s.TotalTrades)
			blacklistCount++
		}
	}
	if blacklistCount == 0 {
		fmt.Println("      None identified")
	}

	// Confidence correlation note
	fmt.Println("\n   📝 NOTE: This analysis shows PnL by symbol but NOT by confidence level.")
	fmt.Println("      To analyze confidence correlation, we need to persist Ginie decisions to DB.")
	fmt.Println("      Implementing database persistence for future analysis...")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
