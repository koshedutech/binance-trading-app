package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"binance-trading-bot/internal/license"
)

func main() {
	fmt.Println("========================================")
	fmt.Println(" License Administration Tool")
	fmt.Println("========================================")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\nOptions:")
		fmt.Println("  1. Generate single license key")
		fmt.Println("  2. Generate batch license keys")
		fmt.Println("  3. Validate a license key")
		fmt.Println("  4. Show license type info")
		fmt.Println("  5. Exit")
		fmt.Print("\nSelect option: ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1":
			generateSingleKey(reader)
		case "2":
			generateBatchKeys(reader)
		case "3":
			validateKey(reader)
		case "4":
			showLicenseInfo()
		case "5":
			fmt.Println("Goodbye!")
			os.Exit(0)
		default:
			fmt.Println("Invalid option")
		}
	}
}

func generateSingleKey(reader *bufio.Reader) {
	fmt.Println("\n--- Generate License Key ---")
	fmt.Println("License types:")
	fmt.Println("  1. Personal  (10 symbols, basic features)")
	fmt.Println("  2. Pro       (50 symbols, all features)")
	fmt.Println("  3. Enterprise (unlimited, white label)")
	fmt.Print("Select type (1-3): ")

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	var licenseType license.LicenseType
	switch input {
	case "1":
		licenseType = license.LicenseTypePersonal
	case "2":
		licenseType = license.LicenseTypePro
	case "3":
		licenseType = license.LicenseTypeEnterprise
	default:
		fmt.Println("Invalid type, defaulting to Personal")
		licenseType = license.LicenseTypePersonal
	}

	key := license.GenerateLicenseKey(licenseType)

	fmt.Println("\n========================================")
	fmt.Printf("  License Type: %s\n", licenseType)
	fmt.Printf("  License Key:  %s\n", key)
	fmt.Printf("  Generated:    %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("========================================")

	// Optionally save to file
	fmt.Print("\nSave to file? (y/n): ")
	save, _ := reader.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(save)) == "y" {
		filename := fmt.Sprintf("license_%s_%s.txt", licenseType, time.Now().Format("20060102_150405"))
		content := fmt.Sprintf("License Type: %s\nLicense Key: %s\nGenerated: %s\n",
			licenseType, key, time.Now().Format("2006-01-02 15:04:05"))
		os.WriteFile(filename, []byte(content), 0644)
		fmt.Printf("Saved to: %s\n", filename)
	}
}

func generateBatchKeys(reader *bufio.Reader) {
	fmt.Println("\n--- Generate Batch License Keys ---")
	fmt.Println("License types:")
	fmt.Println("  1. Personal")
	fmt.Println("  2. Pro")
	fmt.Println("  3. Enterprise")
	fmt.Print("Select type (1-3): ")

	typeInput, _ := reader.ReadString('\n')
	typeInput = strings.TrimSpace(typeInput)

	var licenseType license.LicenseType
	switch typeInput {
	case "1":
		licenseType = license.LicenseTypePersonal
	case "2":
		licenseType = license.LicenseTypePro
	case "3":
		licenseType = license.LicenseTypeEnterprise
	default:
		fmt.Println("Invalid type")
		return
	}

	fmt.Print("How many keys to generate? ")
	countInput, _ := reader.ReadString('\n')
	count, err := strconv.Atoi(strings.TrimSpace(countInput))
	if err != nil || count < 1 || count > 100 {
		fmt.Println("Invalid count (1-100)")
		return
	}

	fmt.Printf("\nGenerating %d %s license keys...\n", count, licenseType)
	fmt.Println("========================================")

	keys := make([]string, count)
	for i := 0; i < count; i++ {
		keys[i] = license.GenerateLicenseKey(licenseType)
		fmt.Printf("  %d. %s\n", i+1, keys[i])
		time.Sleep(10 * time.Millisecond) // Small delay for better randomness
	}
	fmt.Println("========================================")

	// Save to file
	filename := fmt.Sprintf("licenses_%s_%s.txt", licenseType, time.Now().Format("20060102_150405"))
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# %s License Keys\n", licenseType))
	content.WriteString(fmt.Sprintf("# Generated: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("# Count: %d\n\n", count))
	for i, key := range keys {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, key))
	}
	os.WriteFile(filename, []byte(content.String()), 0644)
	fmt.Printf("\nSaved to: %s\n", filename)
}

func validateKey(reader *bufio.Reader) {
	fmt.Println("\n--- Validate License Key ---")
	fmt.Print("Enter license key: ")

	key, _ := reader.ReadString('\n')
	key = strings.TrimSpace(key)

	validator := license.NewValidator("")
	info, err := validator.ValidateLicense(key)

	fmt.Println("\n========================================")
	if err != nil {
		fmt.Printf("  Status:  INVALID\n")
		fmt.Printf("  Error:   %s\n", err)
	} else {
		fmt.Printf("  Status:  %s\n", map[bool]string{true: "VALID", false: "INVALID"}[info.IsValid])
		fmt.Printf("  Type:    %s\n", info.Type)
		fmt.Printf("  Symbols: %d max\n", info.MaxSymbols)
		fmt.Printf("  Message: %s\n", info.Message)
		if info.IsValid {
			fmt.Printf("  Features:\n")
			for _, f := range info.Features {
				fmt.Printf("    - %s\n", f)
			}
		}
	}
	fmt.Println("========================================")
}

func showLicenseInfo() {
	fmt.Println("\n========================================")
	fmt.Println(" License Types Overview")
	fmt.Println("========================================")

	types := []struct {
		Type     license.LicenseType
		Symbols  int
		Features []string
	}{
		{
			Type:    license.LicenseTypeTrial,
			Symbols: 3,
			Features: []string{
				"spot_trading",
				"basic_signals",
			},
		},
		{
			Type:    license.LicenseTypePersonal,
			Symbols: 10,
			Features: []string{
				"spot_trading",
				"futures_trading",
				"basic_signals",
				"ai_analysis",
			},
		},
		{
			Type:    license.LicenseTypePro,
			Symbols: 50,
			Features: []string{
				"spot_trading",
				"futures_trading",
				"basic_signals",
				"ai_analysis",
				"ginie_autopilot",
				"advanced_signals",
				"custom_strategies",
			},
		},
		{
			Type:    license.LicenseTypeEnterprise,
			Symbols: 999,
			Features: []string{
				"spot_trading",
				"futures_trading",
				"basic_signals",
				"ai_analysis",
				"ginie_autopilot",
				"advanced_signals",
				"custom_strategies",
				"api_access",
				"priority_support",
				"white_label",
			},
		},
	}

	for _, t := range types {
		fmt.Printf("\n%s (max %d symbols)\n", strings.ToUpper(string(t.Type)), t.Symbols)
		fmt.Println("  Features:")
		for _, f := range t.Features {
			fmt.Printf("    - %s\n", f)
		}
	}
	fmt.Println()
}
