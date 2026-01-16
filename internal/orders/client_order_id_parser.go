// Package orders provides client order ID generation and parsing for Binance futures trading.
// Epic 7: Client Order ID & Trade Lifecycle Tracking
// Story 7.4: Client Order ID Parser
package orders

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParsedOrderId contains extracted components from a clientOrderId
type ParsedOrderId struct {
	Mode       TradingMode // ModeScalp, ModeSwing, etc.
	Date       time.Time   // Parsed date (only valid for non-fallback IDs)
	DateStr    string      // Original date string "06JAN" or "FALLBACK"
	Sequence   int         // Sequence number (0 for fallback IDs)
	OrderType  OrderType   // E, SL, TP1, etc.
	ChainId    string      // Base ID without type suffix "SCA-06JAN-00001"
	Raw        string      // Original full ID
	IsFallback bool        // True if this is a fallback ID
}

// codeToMode maps 3-character mode codes back to TradingMode
var codeToMode = map[string]TradingMode{
	"ULT": ModeUltraFast,
	"SCA": ModeScalp,
	"SWI": ModeSwing,
	"POS": ModePosition,
}

// validOrderTypes maps order type strings to OrderType constants
var validOrderTypes = map[string]OrderType{
	"E":    OrderTypeEntry,
	"SL":   OrderTypeSL,
	"TP1":  OrderTypeTP1,
	"TP2":  OrderTypeTP2,
	"TP3":  OrderTypeTP3,
	"RB":   OrderTypeRebuy,
	"DCA1": OrderTypeDCA1,
	"DCA2": OrderTypeDCA2,
	"DCA3": OrderTypeDCA3,
	"H":    OrderTypeHedge,
	"HSL":  OrderTypeHedgeSL,
	"HTP":  OrderTypeHedgeTP,
}

// Regular expressions for parsing
var (
	// Normal format: MODE-DDMMM-NNNNN-TYPE (e.g., "SCA-06JAN-00001-E")
	// Note: Input is normalized to uppercase before matching
	normalIDRegex = regexp.MustCompile(`^([A-Z]{3})-(\d{2}[A-Z]{3})-(\d{5})-([A-Z0-9]+)$`)

	// Fallback format: MODE-FALLBACK-8CHAR-TYPE (e.g., "SCA-FALLBACK-a3f7c2e9-E")
	// Note: Input is normalized to uppercase before matching, so hex chars become A-F
	fallbackIDRegex = regexp.MustCompile(`^([A-Z]{3})-FALLBACK-([A-F0-9]{8})-([A-Z0-9]+)$`)

	// Month abbreviations for date parsing
	monthMap = map[string]time.Month{
		"JAN": time.January,
		"FEB": time.February,
		"MAR": time.March,
		"APR": time.April,
		"MAY": time.May,
		"JUN": time.June,
		"JUL": time.July,
		"AUG": time.August,
		"SEP": time.September,
		"OCT": time.October,
		"NOV": time.November,
		"DEC": time.December,
	}
)

// ParseClientOrderId parses a structured clientOrderId.
// Returns nil if not our format (legacy/unstructured IDs).
//
// Supported formats:
//   - Normal: [MODE]-[DDMMM]-[NNNNN]-[TYPE] e.g., "SCA-06JAN-00001-E"
//   - Fallback: [MODE]-FALLBACK-[8CHAR]-[TYPE] e.g., "SCA-FALLBACK-a3f7c2e9-E"
//
// Mode codes: ULT, SCA, SWI, POS
// Order types: E, SL, TP1, TP2, TP3, RB, DCA1, DCA2, DCA3, H, HSL, HTP
func ParseClientOrderId(clientOrderId string) *ParsedOrderId {
	if clientOrderId == "" {
		return nil
	}

	// Normalize to uppercase for matching
	normalized := strings.ToUpper(clientOrderId)

	// Try fallback format first (more specific)
	if matches := fallbackIDRegex.FindStringSubmatch(normalized); matches != nil {
		return parseFallbackID(clientOrderId, matches)
	}

	// Try normal format
	if matches := normalIDRegex.FindStringSubmatch(normalized); matches != nil {
		return parseNormalID(clientOrderId, matches)
	}

	// Not our format
	return nil
}

// parseFallbackID parses a fallback format clientOrderId
func parseFallbackID(raw string, matches []string) *ParsedOrderId {
	// matches[0] = full match
	// matches[1] = mode code (e.g., "SCA")
	// matches[2] = unique ID (e.g., "a3f7c2e9")
	// matches[3] = order type (e.g., "E")

	modeCode := matches[1]
	orderTypeStr := matches[3]

	// Validate mode code
	mode, ok := codeToMode[modeCode]
	if !ok {
		return nil
	}

	// Validate order type
	orderType, ok := validOrderTypes[orderTypeStr]
	if !ok {
		return nil
	}

	// Build chain ID (base ID without order type)
	chainId := modeCode + "-" + FallbackMarker + "-" + strings.ToLower(matches[2])

	return &ParsedOrderId{
		Mode:       mode,
		Date:       time.Time{}, // Zero time for fallback IDs
		DateStr:    FallbackMarker,
		Sequence:   0, // No sequence for fallback IDs
		OrderType:  orderType,
		ChainId:    chainId,
		Raw:        raw,
		IsFallback: true,
	}
}

// parseNormalID parses a normal format clientOrderId
func parseNormalID(raw string, matches []string) *ParsedOrderId {
	// matches[0] = full match
	// matches[1] = mode code (e.g., "SCA")
	// matches[2] = date string (e.g., "06JAN")
	// matches[3] = sequence (e.g., "00001")
	// matches[4] = order type (e.g., "E")

	modeCode := matches[1]
	dateStr := matches[2]
	seqStr := matches[3]
	orderTypeStr := matches[4]

	// Validate mode code
	mode, ok := codeToMode[modeCode]
	if !ok {
		return nil
	}

	// Validate order type
	orderType, ok := validOrderTypes[orderTypeStr]
	if !ok {
		return nil
	}

	// Parse sequence number
	seq, err := strconv.Atoi(seqStr)
	if err != nil {
		return nil
	}

	// Parse and validate date (DDMMM format)
	// This also validates that the month is a valid month abbreviation
	parsedDate := parseDateStr(dateStr)
	if parsedDate.IsZero() {
		return nil // Invalid date format or invalid month
	}

	// Build chain ID (base ID without order type)
	chainId := modeCode + "-" + dateStr + "-" + seqStr

	return &ParsedOrderId{
		Mode:       mode,
		Date:       parsedDate,
		DateStr:    dateStr,
		Sequence:   seq,
		OrderType:  orderType,
		ChainId:    chainId,
		Raw:        raw,
		IsFallback: false,
	}
}

// parseDateStr parses a DDMMM date string (e.g., "06JAN") into a time.Time.
// Uses current year if the date would be in the future, otherwise uses current year.
// Returns zero time if parsing fails.
func parseDateStr(dateStr string) time.Time {
	if len(dateStr) != 5 {
		return time.Time{}
	}

	dayStr := dateStr[:2]
	monthStr := dateStr[2:]

	day, err := strconv.Atoi(dayStr)
	if err != nil || day < 1 || day > 31 {
		return time.Time{}
	}

	month, ok := monthMap[monthStr]
	if !ok {
		return time.Time{}
	}

	// Use current year
	now := time.Now()
	year := now.Year()

	// Create the date
	parsedDate := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)

	return parsedDate
}

// ParseChainId extracts just the chain base ID from a clientOrderId.
// Example: "SCA-06JAN-00001-TP1" -> "SCA-06JAN-00001"
// Example: "SCA-FALLBACK-a3f7c2e9-E" -> "SCA-FALLBACK-a3f7c2e9"
// Returns empty string if the ID cannot be parsed.
func ParseChainId(clientOrderId string) string {
	parsed := ParseClientOrderId(clientOrderId)
	if parsed == nil {
		return ""
	}
	return parsed.ChainId
}

// IsOurFormat checks if a clientOrderId matches our structured format.
// Returns true for both normal and fallback formats.
func IsOurFormat(clientOrderId string) bool {
	return ParseClientOrderId(clientOrderId) != nil
}

// ExtractOrderType extracts just the order type from a clientOrderId.
// Returns empty OrderType if the ID cannot be parsed.
func ExtractOrderType(clientOrderId string) OrderType {
	parsed := ParseClientOrderId(clientOrderId)
	if parsed == nil {
		return ""
	}
	return parsed.OrderType
}

// ExtractMode extracts just the trading mode from a clientOrderId.
// Returns empty TradingMode if the ID cannot be parsed.
func ExtractMode(clientOrderId string) TradingMode {
	parsed := ParseClientOrderId(clientOrderId)
	if parsed == nil {
		return ""
	}
	return parsed.Mode
}

// BelongsToSameChain checks if two clientOrderIds belong to the same order chain.
// Two IDs belong to the same chain if they have the same chain ID (base ID).
func BelongsToSameChain(id1, id2 string) bool {
	chainId1 := ParseChainId(id1)
	chainId2 := ParseChainId(id2)

	if chainId1 == "" || chainId2 == "" {
		return false
	}

	return chainId1 == chainId2
}
