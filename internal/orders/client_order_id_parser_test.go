package orders

import (
	"testing"
	"time"
)

func TestParseClientOrderId_NormalFormat(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedMode     TradingMode
		expectedDateStr  string
		expectedSeq      int
		expectedType     OrderType
		expectedChain    string
		expectedFallback bool
	}{
		{
			name:             "scalp entry order",
			input:            "SCA-06JAN-00001-E",
			expectedMode:     ModeScalp,
			expectedDateStr:  "06JAN",
			expectedSeq:      1,
			expectedType:     OrderTypeEntry,
			expectedChain:    "SCA-06JAN-00001",
			expectedFallback: false,
		},
		{
			name:             "swing TP1 order",
			input:            "SWI-15FEB-00042-TP1",
			expectedMode:     ModeSwing,
			expectedDateStr:  "15FEB",
			expectedSeq:      42,
			expectedType:     OrderTypeTP1,
			expectedChain:    "SWI-15FEB-00042",
			expectedFallback: false,
		},
		{
			name:             "position stop loss order",
			input:            "POS-25DEC-99999-SL",
			expectedMode:     ModePosition,
			expectedDateStr:  "25DEC",
			expectedSeq:      99999,
			expectedType:     OrderTypeSL,
			expectedChain:    "POS-25DEC-99999",
			expectedFallback: false,
		},
		{
			name:             "ultra fast TP2 order",
			input:            "ULT-01MAR-00500-TP2",
			expectedMode:     ModeUltraFast,
			expectedDateStr:  "01MAR",
			expectedSeq:      500,
			expectedType:     OrderTypeTP2,
			expectedChain:    "ULT-01MAR-00500",
			expectedFallback: false,
		},
		{
			name:             "scalp TP3 order",
			input:            "SCA-12APR-12345-TP3",
			expectedMode:     ModeScalp,
			expectedDateStr:  "12APR",
			expectedSeq:      12345,
			expectedType:     OrderTypeTP3,
			expectedChain:    "SCA-12APR-12345",
			expectedFallback: false,
		},
		{
			name:             "rebuy order",
			input:            "SCA-08MAY-00010-RB",
			expectedMode:     ModeScalp,
			expectedDateStr:  "08MAY",
			expectedSeq:      10,
			expectedType:     OrderTypeRebuy,
			expectedChain:    "SCA-08MAY-00010",
			expectedFallback: false,
		},
		{
			name:             "DCA1 order",
			input:            "SWI-15JUN-00020-DCA1",
			expectedMode:     ModeSwing,
			expectedDateStr:  "15JUN",
			expectedSeq:      20,
			expectedType:     OrderTypeDCA1,
			expectedChain:    "SWI-15JUN-00020",
			expectedFallback: false,
		},
		{
			name:             "DCA2 order",
			input:            "POS-22JUL-00030-DCA2",
			expectedMode:     ModePosition,
			expectedDateStr:  "22JUL",
			expectedSeq:      30,
			expectedType:     OrderTypeDCA2,
			expectedChain:    "POS-22JUL-00030",
			expectedFallback: false,
		},
		{
			name:             "DCA3 order",
			input:            "ULT-05AUG-00040-DCA3",
			expectedMode:     ModeUltraFast,
			expectedDateStr:  "05AUG",
			expectedSeq:      40,
			expectedType:     OrderTypeDCA3,
			expectedChain:    "ULT-05AUG-00040",
			expectedFallback: false,
		},
		{
			name:             "hedge order",
			input:            "SCA-18SEP-00050-H",
			expectedMode:     ModeScalp,
			expectedDateStr:  "18SEP",
			expectedSeq:      50,
			expectedType:     OrderTypeHedge,
			expectedChain:    "SCA-18SEP-00050",
			expectedFallback: false,
		},
		{
			name:             "October date",
			input:            "SWI-30OCT-00001-E",
			expectedMode:     ModeSwing,
			expectedDateStr:  "30OCT",
			expectedSeq:      1,
			expectedType:     OrderTypeEntry,
			expectedChain:    "SWI-30OCT-00001",
			expectedFallback: false,
		},
		{
			name:             "November date",
			input:            "POS-15NOV-00001-E",
			expectedMode:     ModePosition,
			expectedDateStr:  "15NOV",
			expectedSeq:      1,
			expectedType:     OrderTypeEntry,
			expectedChain:    "POS-15NOV-00001",
			expectedFallback: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseClientOrderId(tt.input)
			if result == nil {
				t.Fatalf("ParseClientOrderId(%q) returned nil, expected valid result", tt.input)
			}

			if result.Mode != tt.expectedMode {
				t.Errorf("Mode = %v, want %v", result.Mode, tt.expectedMode)
			}
			if result.DateStr != tt.expectedDateStr {
				t.Errorf("DateStr = %v, want %v", result.DateStr, tt.expectedDateStr)
			}
			if result.Sequence != tt.expectedSeq {
				t.Errorf("Sequence = %v, want %v", result.Sequence, tt.expectedSeq)
			}
			if result.OrderType != tt.expectedType {
				t.Errorf("OrderType = %v, want %v", result.OrderType, tt.expectedType)
			}
			if result.ChainId != tt.expectedChain {
				t.Errorf("ChainId = %v, want %v", result.ChainId, tt.expectedChain)
			}
			if result.IsFallback != tt.expectedFallback {
				t.Errorf("IsFallback = %v, want %v", result.IsFallback, tt.expectedFallback)
			}
			if result.Raw != tt.input {
				t.Errorf("Raw = %v, want %v", result.Raw, tt.input)
			}
		})
	}
}

func TestParseClientOrderId_FallbackFormat(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedMode  TradingMode
		expectedType  OrderType
		expectedChain string
	}{
		{
			name:          "scalp fallback entry",
			input:         "SCA-FALLBACK-a3f7c2e9-E",
			expectedMode:  ModeScalp,
			expectedType:  OrderTypeEntry,
			expectedChain: "SCA-FALLBACK-a3f7c2e9",
		},
		{
			name:          "swing fallback TP1",
			input:         "SWI-FALLBACK-12345678-TP1",
			expectedMode:  ModeSwing,
			expectedType:  OrderTypeTP1,
			expectedChain: "SWI-FALLBACK-12345678",
		},
		{
			name:          "position fallback SL",
			input:         "POS-FALLBACK-abcdef01-SL",
			expectedMode:  ModePosition,
			expectedType:  OrderTypeSL,
			expectedChain: "POS-FALLBACK-abcdef01",
		},
		{
			name:          "ultra fast fallback",
			input:         "ULT-FALLBACK-00000000-E",
			expectedMode:  ModeUltraFast,
			expectedType:  OrderTypeEntry,
			expectedChain: "ULT-FALLBACK-00000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseClientOrderId(tt.input)
			if result == nil {
				t.Fatalf("ParseClientOrderId(%q) returned nil, expected valid result", tt.input)
			}

			if result.Mode != tt.expectedMode {
				t.Errorf("Mode = %v, want %v", result.Mode, tt.expectedMode)
			}
			if result.OrderType != tt.expectedType {
				t.Errorf("OrderType = %v, want %v", result.OrderType, tt.expectedType)
			}
			if result.ChainId != tt.expectedChain {
				t.Errorf("ChainId = %v, want %v", result.ChainId, tt.expectedChain)
			}
			if !result.IsFallback {
				t.Error("IsFallback = false, want true")
			}
			if result.Sequence != 0 {
				t.Errorf("Sequence = %v, want 0 for fallback", result.Sequence)
			}
			if result.DateStr != FallbackMarker {
				t.Errorf("DateStr = %v, want %v", result.DateStr, FallbackMarker)
			}
			if !result.Date.IsZero() {
				t.Errorf("Date = %v, want zero time for fallback", result.Date)
			}
		})
	}
}

func TestParseClientOrderId_InvalidFormats(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"random string", "hello-world"},
		{"legacy binance ID", "x-A1234567890"},
		{"too short", "SCA-E"},
		{"missing type", "SCA-06JAN-00001"},
		{"invalid mode", "XXX-06JAN-00001-E"},
		{"invalid sequence format", "SCA-06JAN-ABCDE-E"},
		{"sequence too short", "SCA-06JAN-001-E"},
		{"sequence too long", "SCA-06JAN-000001-E"},
		{"invalid date format", "SCA-6JAN-00001-E"},
		{"invalid month", "SCA-06XXX-00001-E"},
		{"invalid order type", "SCA-06JAN-00001-INVALID"},
		{"fallback wrong length", "SCA-FALLBACK-abc-E"},
		{"fallback invalid chars", "SCA-FALLBACK-XXXXXXXX-E"},
		{"special characters", "SCA-06JAN-00001-E!"},
		{"extra segments", "SCA-06JAN-00001-E-EXTRA"},
		{"uuid format", "550e8400-e29b-41d4-a716-446655440000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseClientOrderId(tt.input)
			if result != nil {
				t.Errorf("ParseClientOrderId(%q) = %+v, want nil", tt.input, result)
			}
		})
	}
}

func TestParseChainId(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal entry order",
			input:    "SCA-06JAN-00001-E",
			expected: "SCA-06JAN-00001",
		},
		{
			name:     "normal TP1 order",
			input:    "SCA-06JAN-00001-TP1",
			expected: "SCA-06JAN-00001",
		},
		{
			name:     "fallback order",
			input:    "SCA-FALLBACK-a3f7c2e9-E",
			expected: "SCA-FALLBACK-a3f7c2e9",
		},
		{
			name:     "invalid format",
			input:    "invalid-order-id",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseChainId(tt.input)
			if result != tt.expected {
				t.Errorf("ParseChainId(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsOurFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"normal format", "SCA-06JAN-00001-E", true},
		{"fallback format", "SCA-FALLBACK-a3f7c2e9-E", true},
		{"legacy format", "x-A1234567890", false},
		{"empty string", "", false},
		{"random string", "not-our-format", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsOurFormat(tt.input)
			if result != tt.expected {
				t.Errorf("IsOurFormat(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractOrderType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected OrderType
	}{
		{"entry order", "SCA-06JAN-00001-E", OrderTypeEntry},
		{"TP1 order", "SCA-06JAN-00001-TP1", OrderTypeTP1},
		{"TP2 order", "SCA-06JAN-00001-TP2", OrderTypeTP2},
		{"TP3 order", "SCA-06JAN-00001-TP3", OrderTypeTP3},
		{"SL order", "SCA-06JAN-00001-SL", OrderTypeSL},
		{"RB order", "SCA-06JAN-00001-RB", OrderTypeRebuy},
		{"DCA1 order", "SCA-06JAN-00001-DCA1", OrderTypeDCA1},
		{"DCA2 order", "SCA-06JAN-00001-DCA2", OrderTypeDCA2},
		{"DCA3 order", "SCA-06JAN-00001-DCA3", OrderTypeDCA3},
		{"hedge order", "SCA-06JAN-00001-H", OrderTypeHedge},
		{"fallback entry", "SCA-FALLBACK-a3f7c2e9-E", OrderTypeEntry},
		{"invalid format", "invalid", OrderType("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractOrderType(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractOrderType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractMode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TradingMode
	}{
		{"scalp mode", "SCA-06JAN-00001-E", ModeScalp},
		{"swing mode", "SWI-06JAN-00001-E", ModeSwing},
		{"position mode", "POS-06JAN-00001-E", ModePosition},
		{"ultra fast mode", "ULT-06JAN-00001-E", ModeUltraFast},
		{"fallback scalp", "SCA-FALLBACK-a3f7c2e9-E", ModeScalp},
		{"invalid format", "invalid", TradingMode("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractMode(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractMode(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBelongsToSameChain(t *testing.T) {
	tests := []struct {
		name     string
		id1      string
		id2      string
		expected bool
	}{
		{
			name:     "same chain entry and TP1",
			id1:      "SCA-06JAN-00001-E",
			id2:      "SCA-06JAN-00001-TP1",
			expected: true,
		},
		{
			name:     "same chain entry and SL",
			id1:      "SCA-06JAN-00001-E",
			id2:      "SCA-06JAN-00001-SL",
			expected: true,
		},
		{
			name:     "different chains same date",
			id1:      "SCA-06JAN-00001-E",
			id2:      "SCA-06JAN-00002-E",
			expected: false,
		},
		{
			name:     "different chains different date",
			id1:      "SCA-06JAN-00001-E",
			id2:      "SCA-07JAN-00001-E",
			expected: false,
		},
		{
			name:     "different modes",
			id1:      "SCA-06JAN-00001-E",
			id2:      "SWI-06JAN-00001-E",
			expected: false,
		},
		{
			name:     "same fallback chain",
			id1:      "SCA-FALLBACK-a3f7c2e9-E",
			id2:      "SCA-FALLBACK-a3f7c2e9-TP1",
			expected: true,
		},
		{
			name:     "different fallback chains",
			id1:      "SCA-FALLBACK-a3f7c2e9-E",
			id2:      "SCA-FALLBACK-b4g8d3f0-E",
			expected: false,
		},
		{
			name:     "normal and fallback",
			id1:      "SCA-06JAN-00001-E",
			id2:      "SCA-FALLBACK-a3f7c2e9-E",
			expected: false,
		},
		{
			name:     "invalid first ID",
			id1:      "invalid",
			id2:      "SCA-06JAN-00001-E",
			expected: false,
		},
		{
			name:     "invalid second ID",
			id1:      "SCA-06JAN-00001-E",
			id2:      "invalid",
			expected: false,
		},
		{
			name:     "both invalid",
			id1:      "invalid1",
			id2:      "invalid2",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BelongsToSameChain(tt.id1, tt.id2)
			if result != tt.expected {
				t.Errorf("BelongsToSameChain(%q, %q) = %v, want %v", tt.id1, tt.id2, result, tt.expected)
			}
		})
	}
}

func TestParseDateStr(t *testing.T) {
	// Test that dates are parsed correctly
	testCases := []struct {
		input         string
		expectedDay   int
		expectedMonth time.Month
	}{
		{"06JAN", 6, time.January},
		{"15FEB", 15, time.February},
		{"01MAR", 1, time.March},
		{"30APR", 30, time.April},
		{"15MAY", 15, time.May},
		{"10JUN", 10, time.June},
		{"04JUL", 4, time.July},
		{"20AUG", 20, time.August},
		{"25SEP", 25, time.September},
		{"31OCT", 31, time.October},
		{"11NOV", 11, time.November},
		{"25DEC", 25, time.December},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			parsed := ParseClientOrderId("SCA-" + tc.input + "-00001-E")
			if parsed == nil {
				t.Fatalf("Failed to parse order ID with date %s", tc.input)
			}

			if parsed.Date.Day() != tc.expectedDay {
				t.Errorf("Day = %d, want %d", parsed.Date.Day(), tc.expectedDay)
			}
			if parsed.Date.Month() != tc.expectedMonth {
				t.Errorf("Month = %v, want %v", parsed.Date.Month(), tc.expectedMonth)
			}
		})
	}
}

func TestParsedOrderId_DateIsZeroForFallback(t *testing.T) {
	parsed := ParseClientOrderId("SCA-FALLBACK-a3f7c2e9-E")
	if parsed == nil {
		t.Fatal("Failed to parse fallback order ID")
	}

	if !parsed.Date.IsZero() {
		t.Errorf("Date for fallback ID should be zero, got %v", parsed.Date)
	}
}

func TestCaseInsensitivity(t *testing.T) {
	// The parser normalizes input to uppercase internally
	// So lowercase input should be accepted and parsed correctly
	tests := []struct {
		name          string
		input         string
		shouldParse   bool
		expectedChain string
	}{
		{"all uppercase normal", "SCA-06JAN-00001-E", true, "SCA-06JAN-00001"},
		{"all uppercase fallback", "SCA-FALLBACK-a3f7c2e9-E", true, "SCA-FALLBACK-a3f7c2e9"},
		// Lowercase should be normalized and parse successfully
		{"lowercase mode", "sca-06JAN-00001-E", true, "SCA-06JAN-00001"},
		{"lowercase date", "SCA-06jan-00001-E", true, "SCA-06JAN-00001"},
		{"all lowercase", "sca-06jan-00001-e", true, "SCA-06JAN-00001"},
		{"lowercase fallback", "sca-fallback-a3f7c2e9-e", true, "SCA-FALLBACK-a3f7c2e9"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseClientOrderId(tt.input)
			if tt.shouldParse && result == nil {
				t.Errorf("ParseClientOrderId(%q) = nil, want non-nil", tt.input)
			}
			if !tt.shouldParse && result != nil {
				t.Errorf("ParseClientOrderId(%q) = %+v, want nil", tt.input, result)
			}
			if tt.shouldParse && result != nil && result.ChainId != tt.expectedChain {
				t.Errorf("ParseClientOrderId(%q).ChainId = %q, want %q", tt.input, result.ChainId, tt.expectedChain)
			}
		})
	}
}
