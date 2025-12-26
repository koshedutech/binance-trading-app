package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TradeWithConfidence struct {
	Symbol       string
	Confidence   float64
	RealizedPnL  float64
	PnLPercent   float64
	EntryTime    time.Time
	PositionSide string
}

type ConfidenceBucket struct {
	MinConf       float64
	MaxConf       float64
	TotalTrades   int
	WinningTrades int
	LosingTrades  int
	TotalPnL      float64
	AvgPnL        float64
	WinRate       float64
}

func main() {
	// Read from environment variables directly
	// Build connection string
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPass := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "binance_trading")

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPass, dbHost, dbPort, dbName)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	fmt.Println("=" + string(make([]byte, 79)))
	fmt.Println("🧪 CONFIDENCE THRESHOLD BACKTEST ANALYSIS")
	fmt.Println("=" + string(make([]byte, 79)))

	// Query trades with AI decision confidence
	query := `
		SELECT
			ft.symbol,
			COALESCE(ad.confidence, 0) as confidence,
			COALESCE(ft.realized_pnl, 0) as realized_pnl,
			COALESCE(ft.realized_pnl_percent, 0) as pnl_percent,
			ft.entry_time,
			ft.position_side
		FROM futures_trades ft
		LEFT JOIN ai_decisions ad ON ft.ai_decision_id = ad.id
		WHERE ft.status = 'CLOSED'
		  AND ft.realized_pnl IS NOT NULL
		ORDER BY ft.entry_time DESC
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		fmt.Printf("Query failed: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var trades []TradeWithConfidence
	for rows.Next() {
		var t TradeWithConfidence
		err := rows.Scan(&t.Symbol, &t.Confidence, &t.RealizedPnL, &t.PnLPercent, &t.EntryTime, &t.PositionSide)
		if err != nil {
			fmt.Printf("Scan error: %v\n", err)
			continue
		}
		trades = append(trades, t)
	}

	if len(trades) == 0 {
		fmt.Println("\n❌ No closed trades with AI decisions found in database.")
		fmt.Println("   Make sure trades have ai_decision_id linked.")

		// Try to get basic trade stats without AI linkage
		var totalTrades, closedTrades int
		pool.QueryRow(ctx, `SELECT COUNT(*) FROM futures_trades`).Scan(&totalTrades)
		pool.QueryRow(ctx, `SELECT COUNT(*) FROM futures_trades WHERE status = 'CLOSED'`).Scan(&closedTrades)

		var linkedTrades int
		pool.QueryRow(ctx, `SELECT COUNT(*) FROM futures_trades WHERE ai_decision_id IS NOT NULL`).Scan(&linkedTrades)

		fmt.Printf("\n   Total trades: %d\n", totalTrades)
		fmt.Printf("   Closed trades: %d\n", closedTrades)
		fmt.Printf("   Trades with AI decision link: %d\n", linkedTrades)
		return
	}

	fmt.Printf("\n📊 Analyzing %d closed trades with AI decisions...\n\n", len(trades))

	// Define confidence buckets
	buckets := []ConfidenceBucket{
		{MinConf: 0.00, MaxConf: 0.35, TotalTrades: 0},  // Very low (would be rejected at 35%)
		{MinConf: 0.35, MaxConf: 0.50, TotalTrades: 0},  // Low (35-50%)
		{MinConf: 0.50, MaxConf: 0.55, TotalTrades: 0},  // Medium-Low (50-55%)
		{MinConf: 0.55, MaxConf: 0.65, TotalTrades: 0},  // Medium (55-65%)
		{MinConf: 0.65, MaxConf: 0.75, TotalTrades: 0},  // Medium-High (65-75%)
		{MinConf: 0.75, MaxConf: 1.00, TotalTrades: 0},  // High (75%+)
	}

	// Categorize trades into buckets
	for _, t := range trades {
		for i := range buckets {
			if t.Confidence >= buckets[i].MinConf && t.Confidence < buckets[i].MaxConf {
				buckets[i].TotalTrades++
				buckets[i].TotalPnL += t.RealizedPnL
				if t.RealizedPnL > 0 {
					buckets[i].WinningTrades++
				} else if t.RealizedPnL < 0 {
					buckets[i].LosingTrades++
				}
				break
			}
		}
	}

	// Calculate stats
	for i := range buckets {
		if buckets[i].TotalTrades > 0 {
			buckets[i].AvgPnL = buckets[i].TotalPnL / float64(buckets[i].TotalTrades)
			buckets[i].WinRate = float64(buckets[i].WinningTrades) / float64(buckets[i].TotalTrades) * 100
		}
	}

	// Print bucket analysis
	fmt.Println("┌─────────────────┬────────┬─────────┬─────────┬──────────────┬──────────────┬──────────┐")
	fmt.Println("│ Confidence      │ Trades │ Winners │ Losers  │ Total PnL    │ Avg PnL      │ Win Rate │")
	fmt.Println("├─────────────────┼────────┼─────────┼─────────┼──────────────┼──────────────┼──────────┤")

	for _, b := range buckets {
		fmt.Printf("│ %5.0f%% - %5.0f%% │ %6d │ %7d │ %7d │ %+12.2f │ %+12.2f │ %7.1f%% │\n",
			b.MinConf*100, b.MaxConf*100,
			b.TotalTrades, b.WinningTrades, b.LosingTrades,
			b.TotalPnL, b.AvgPnL, b.WinRate)
	}
	fmt.Println("└─────────────────┴────────┴─────────┴─────────┴──────────────┴──────────────┴──────────┘")

	// Threshold comparison analysis
	fmt.Println("\n" + "=" + string(make([]byte, 79)))
	fmt.Println("📈 THRESHOLD COMPARISON ANALYSIS")
	fmt.Println("=" + string(make([]byte, 79)))

	thresholds := []float64{0.35, 0.50, 0.55, 0.65, 0.75}

	for _, threshold := range thresholds {
		var included, excluded int
		var includedPnL, excludedPnL float64
		var includedWins, excludedWins int
		var includedLosses, excludedLosses int

		for _, t := range trades {
			if t.Confidence >= threshold {
				included++
				includedPnL += t.RealizedPnL
				if t.RealizedPnL > 0 {
					includedWins++
				} else if t.RealizedPnL < 0 {
					includedLosses++
				}
			} else {
				excluded++
				excludedPnL += t.RealizedPnL
				if t.RealizedPnL > 0 {
					excludedWins++
				} else if t.RealizedPnL < 0 {
					excludedLosses++
				}
			}
		}

		includedWinRate := 0.0
		if included > 0 {
			includedWinRate = float64(includedWins) / float64(included) * 100
		}

		excludedWinRate := 0.0
		if excluded > 0 {
			excludedWinRate = float64(excludedWins) / float64(excluded) * 100
		}

		fmt.Printf("\n🎯 Threshold: %.0f%%\n", threshold*100)
		fmt.Printf("   ├── INCLUDED (≥%.0f%%): %d trades, PnL: $%.2f, Win Rate: %.1f%%\n",
			threshold*100, included, includedPnL, includedWinRate)
		fmt.Printf("   └── EXCLUDED (<%.0f%%): %d trades, PnL: $%.2f, Win Rate: %.1f%%\n",
			threshold*100, excluded, excludedPnL, excludedWinRate)

		if excludedPnL < 0 {
			fmt.Printf("   💰 AVOIDED LOSS: $%.2f by using %.0f%% threshold\n", -excludedPnL, threshold*100)
		} else if excludedPnL > 0 {
			fmt.Printf("   ⚠️  MISSED PROFIT: $%.2f by using %.0f%% threshold\n", excludedPnL, threshold*100)
		}
	}

	// Recommendation
	fmt.Println("\n" + "=" + string(make([]byte, 79)))
	fmt.Println("🏆 RECOMMENDATION")
	fmt.Println("=" + string(make([]byte, 79)))

	// Find best threshold based on excluded PnL being negative (avoided losses)
	bestThreshold := 0.55
	bestAvoidedLoss := 0.0

	for _, threshold := range thresholds {
		var excludedPnL float64
		for _, t := range trades {
			if t.Confidence < threshold {
				excludedPnL += t.RealizedPnL
			}
		}
		if excludedPnL < bestAvoidedLoss {
			bestAvoidedLoss = excludedPnL
			bestThreshold = threshold
		}
	}

	if bestAvoidedLoss < 0 {
		fmt.Printf("\n✅ Optimal threshold: %.0f%% (would have avoided $%.2f in losses)\n",
			bestThreshold*100, -bestAvoidedLoss)
	} else {
		fmt.Println("\n⚠️  No clear optimal threshold - confidence doesn't correlate with outcomes.")
		fmt.Println("   Consider: Is the confidence scoring system accurate?")
	}

	// Symbol breakdown for losing trades
	fmt.Println("\n" + "=" + string(make([]byte, 79)))
	fmt.Println("📉 TOP LOSING TRADES (by confidence)")
	fmt.Println("=" + string(make([]byte, 79)))

	losingCount := 0
	for _, t := range trades {
		if t.RealizedPnL < -10 && losingCount < 10 { // Show top 10 losses > $10
			fmt.Printf("   %s | Conf: %.1f%% | PnL: $%.2f | %s | %s\n",
				t.Symbol, t.Confidence*100, t.RealizedPnL, t.PositionSide, t.EntryTime.Format("2006-01-02 15:04"))
			losingCount++
		}
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
