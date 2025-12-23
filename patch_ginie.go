// +build ignore

package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	filePath := "D:/Apps/binance-trading-bot/internal/autopilot/ginie_autopilot.go"

	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Println("Error reading file:", err)
		os.Exit(1)
	}

	oldText := `	// Determine the side for closing orders
	closeSide := "SELL"`

	newText := `	// CRITICAL: Cancel ALL existing algo orders for this symbol FIRST
	// This prevents accumulation of orphan orders when updating SL/TP
	log.Printf("[GINIE] %s: Cancelling existing algo orders before placing new SL/TP", pos.Symbol)
	ga.cancelAllAlgoOrdersForSymbol(pos.Symbol)

	// Clear stored algo IDs since we cancelled all orders
	pos.StopLossAlgoID = 0
	pos.TakeProfitAlgoIDs = nil

	// Determine the side for closing orders
	closeSide := "SELL"`

	newContent := strings.Replace(string(content), oldText, newText, 1)

	if newContent == string(content) {
		fmt.Println("Pattern not found or already replaced")
		os.Exit(0)
	}

	err = os.WriteFile(filePath, []byte(newContent), 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
		os.Exit(1)
	}

	fmt.Println("File patched successfully")
}
