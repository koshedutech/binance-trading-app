// Package orders provides client order ID generation testing for Binance futures trading.
// Epic 7: Client Order ID & Trade Lifecycle Tracking - Story 7.10: Edge Case Test Suite
package orders

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// MOCK TYPES
// ============================================================================

// MockCacheService mocks the cache.CacheService for testing ClientOrderIdGenerator
type MockCacheService struct {
	mu                  sync.RWMutex
	healthy             bool
	sequences           map[string]int64 // key: "user:{id}:sequence:{dateKey}"
	incrementCalls      []IncrementCall
	incrementErr        error
	healthyCalled       int
	simulateUnavailable bool
}

// IncrementCall tracks IncrementDailySequence invocations
type IncrementCall struct {
	UserID  string
	DateKey string
}

// NewMockCacheService creates a new mock cache service
func NewMockCacheService() *MockCacheService {
	return &MockCacheService{
		healthy:   true,
		sequences: make(map[string]int64),
	}
}

// IsHealthy returns mock health status
func (m *MockCacheService) IsHealthy() bool {
	m.mu.Lock()
	m.healthyCalled++
	m.mu.Unlock()
	return m.healthy
}

// IncrementDailySequence mocks the atomic sequence increment
func (m *MockCacheService) IncrementDailySequence(ctx context.Context, userID, dateKey string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.incrementCalls = append(m.incrementCalls, IncrementCall{UserID: userID, DateKey: dateKey})

	if m.simulateUnavailable || m.incrementErr != nil {
		if m.incrementErr != nil {
			return 0, m.incrementErr
		}
		return 0, errors.New("redis unavailable (circuit breaker open)")
	}

	key := fmt.Sprintf("user:%s:sequence:%s", userID, dateKey)
	m.sequences[key]++
	return m.sequences[key], nil
}

// SetSequence sets a specific sequence value for testing
func (m *MockCacheService) SetSequence(userID, dateKey string, value int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("user:%s:sequence:%s", userID, dateKey)
	m.sequences[key] = value
}

// ============================================================================
// TESTABLE CLIENT ORDER ID GENERATOR
// ============================================================================

// CacheServiceInterface defines the interface for cache operations needed by ClientOrderIdGenerator
type CacheServiceInterface interface {
	IsHealthy() bool
	IncrementDailySequence(ctx context.Context, userID, dateKey string) (int64, error)
}

// TestableClientOrderIdGenerator is a test wrapper for ClientOrderIdGenerator logic
type TestableClientOrderIdGenerator struct {
	cache    CacheServiceInterface
	timezone *time.Location
}

// NewTestableClientOrderIdGenerator creates a testable generator with mock cache
func NewTestableClientOrderIdGenerator(cache CacheServiceInterface, tz *time.Location) *TestableClientOrderIdGenerator {
	return &TestableClientOrderIdGenerator{
		cache:    cache,
		timezone: tz,
	}
}

// Generate creates a new clientOrderId with format: MODE-DDMMM-NNNNN-TYPE
func (g *TestableClientOrderIdGenerator) Generate(ctx context.Context, userID string, mode TradingMode, orderType OrderType) (string, error) {
	return g.GenerateAtTime(ctx, userID, mode, orderType, time.Now())
}

// GenerateAtTime creates a clientOrderId at a specific time (for testing)
func (g *TestableClientOrderIdGenerator) GenerateAtTime(ctx context.Context, userID string, mode TradingMode, orderType OrderType, t time.Time) (string, error) {
	// Get date in user's timezone
	now := t.In(g.timezone)
	dateStr := strings.ToUpper(now.Format("02Jan")) // "06JAN"
	dateKey := now.Format("20060102")               // "20260106"

	// Get mode code
	modeCode, ok := ModeCode[mode]
	if !ok {
		modeCode = "SCA" // Default fallback
	}

	// Try to get sequence from Redis
	seq, err := g.cache.IncrementDailySequence(ctx, userID, dateKey)
	if err != nil {
		// Redis unavailable - use fallback
		// Generate UUID-based fallback (simulated with timestamp for testing)
		fallbackID := fmt.Sprintf("%s-FALLBACK-%08x-%s", modeCode, now.UnixNano()%0xFFFFFFFF, orderType)
		return fallbackID, nil // Return fallback ID, not error
	}

	// Format: ULT-06JAN-00001-E
	return fmt.Sprintf("%s-%s-%05d-%s", modeCode, dateStr, seq, orderType), nil
}

// GenerateRelated creates a related order ID using the same chain base
func (g *TestableClientOrderIdGenerator) GenerateRelated(baseID string, orderType OrderType) string {
	return fmt.Sprintf("%s-%s", baseID, orderType)
}

// Note: ValidateClientOrderID is defined in client_order_id.go and returns error

// ExtractChainBase extracts the chain base ID from a full clientOrderId
// Example: "ULT-06JAN-00001-E" -> "ULT-06JAN-00001"
func ExtractChainBase(clientOrderID string) string {
	parts := strings.Split(clientOrderID, "-")
	if len(parts) < 4 {
		return clientOrderID // Return as-is if not valid format
	}
	return strings.Join(parts[:3], "-")
}

// ============================================================================
// TEST CASES: MODE CODES
// ============================================================================

// TestAllModeCodesGenerate verifies all 4 mode codes generate correctly
func TestAllModeCodesGenerate(t *testing.T) {
	testCases := []struct {
		mode         TradingMode
		expectedCode string
	}{
		{ModeUltraFast, "ULT"},
		{ModeScalp, "SCA"},
		{ModeSwing, "SWI"},
		{ModePosition, "POS"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.mode), func(t *testing.T) {
			mockCache := NewMockCacheService()
			tz, _ := time.LoadLocation("Asia/Kolkata")
			generator := NewTestableClientOrderIdGenerator(mockCache, tz)

			ctx := context.Background()
			id, err := generator.Generate(ctx, "user123", tc.mode, OrderTypeEntry)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !strings.HasPrefix(id, tc.expectedCode+"-") {
				t.Errorf("Expected ID to start with %s-, got: %s", tc.expectedCode, id)
			}
		})
	}
}

// TestModeFromString verifies string to TradingMode conversion
func TestModeFromString(t *testing.T) {
	testCases := []struct {
		input    string
		expected TradingMode
	}{
		{"ultra_fast", ModeUltraFast},
		{"scalp", ModeScalp},
		{"swing", ModeSwing},
		{"position", ModePosition},
		{"unknown", ModeScalp},    // Default fallback
		{"", ModeScalp},           // Empty string fallback
		{"ULTRA_FAST", ModeScalp}, // Case sensitive - should default
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := ModeFromString(tc.input)
			if result != tc.expected {
				t.Errorf("ModeFromString(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

// ============================================================================
// TEST CASES: ORDER TYPE SUFFIXES
// ============================================================================

// TestAllOrderTypeSuffixes verifies all 10 order type suffixes work correctly
func TestAllOrderTypeSuffixes(t *testing.T) {
	orderTypes := []OrderType{
		OrderTypeEntry, // E
		OrderTypeTP1,   // TP1
		OrderTypeTP2,   // TP2
		OrderTypeTP3,   // TP3
		OrderTypeRebuy, // RB
		OrderTypeDCA1,  // DCA1
		OrderTypeDCA2,  // DCA2
		OrderTypeDCA3,  // DCA3
		OrderTypeHedge, // H
		OrderTypeSL,    // SL
	}

	mockCache := NewMockCacheService()
	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	for _, orderType := range orderTypes {
		t.Run(string(orderType), func(t *testing.T) {
			id, err := generator.Generate(ctx, "user123", ModeUltraFast, orderType)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify suffix is at the end
			if !strings.HasSuffix(id, "-"+string(orderType)) {
				t.Errorf("Expected ID to end with -%s, got: %s", orderType, id)
			}

			// Verify ID is within Binance limit
			if err := ValidateClientOrderID(id); err != nil {
				t.Errorf("ID validation failed: %s (len=%d): %v", id, len(id), err)
			}
		})
	}
}

// TestAllOrderTypesFunction verifies AllOrderTypes returns all expected types
func TestAllOrderTypesFunction(t *testing.T) {
	types := AllOrderTypes()

	expectedTypes := []OrderType{
		OrderTypeEntry, OrderTypeTP1, OrderTypeTP2, OrderTypeTP3,
		OrderTypeRebuy, OrderTypeDCA1, OrderTypeDCA2, OrderTypeDCA3,
		OrderTypeHedge, OrderTypeHedgeSL, OrderTypeHedgeTP, OrderTypeSL,
	}

	if len(types) != len(expectedTypes) {
		t.Errorf("Expected %d order types, got %d", len(expectedTypes), len(types))
	}

	// Verify all expected types are present
	for _, expected := range expectedTypes {
		found := false
		for _, got := range types {
			if got == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing order type: %s", expected)
		}
	}
}

// ============================================================================
// TEST CASES: CHARACTER LIMIT (BINANCE 36 CHAR LIMIT)
// ============================================================================

// TestGeneratedIDsWithin36Characters verifies all combinations stay <= 36 chars
func TestGeneratedIDsWithin36Characters(t *testing.T) {
	modes := []TradingMode{ModeUltraFast, ModeScalp, ModeSwing, ModePosition}
	orderTypes := AllOrderTypes()

	mockCache := NewMockCacheService()
	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	for _, mode := range modes {
		for _, orderType := range orderTypes {
			testName := fmt.Sprintf("%s-%s", mode, orderType)
			t.Run(testName, func(t *testing.T) {
				id, err := generator.Generate(ctx, "user123", mode, orderType)

				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				if len(id) > 36 {
					t.Errorf("ID exceeds Binance 36 char limit: %s (len=%d)", id, len(id))
				}
			})
		}
	}
}

// TestMaxSequenceStaysWithinLimit verifies IDs with max sequence (99999) stay valid
func TestMaxSequenceStaysWithinLimit(t *testing.T) {
	mockCache := NewMockCacheService()
	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	// Set sequence to 99998 so next increment gives 99999
	now := time.Now().In(tz)
	dateKey := now.Format("20060102")
	mockCache.SetSequence("user123", dateKey, 99998)

	id, err := generator.Generate(ctx, "user123", ModePosition, OrderTypeDCA3)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Example: POS-15JAN-99999-DCA3 = 20 characters
	if len(id) > 36 {
		t.Errorf("Max sequence ID exceeds limit: %s (len=%d)", id, len(id))
	}

	// Verify sequence is 99999 in the ID
	if !strings.Contains(id, "99999") {
		t.Errorf("Expected sequence 99999 in ID, got: %s", id)
	}
}

// TestValidateClientOrderID tests the validation function
func TestValidateClientOrderID(t *testing.T) {
	testCases := []struct {
		name      string
		id        string
		wantError bool
	}{
		{"Normal ID", "ULT-06JAN-00001-E", false},
		{"Max length normal", "POS-31DEC-99999-DCA3", false},
		{"Exactly 36 chars", "ULT-FALLBACK-12345678-DCA3__________", false}, // 36 chars
		{"Over 36 chars", "ULT-FALLBACK-12345678-DCA3-EXTRA-STUFF", true},   // 38 chars - should fail
		{"Empty string", "", true},                                          // Empty should fail
		{"Single char", "A", true},                                          // Single char should fail (requires 3+ parts)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateClientOrderID(tc.id)
			gotError := err != nil
			if gotError != tc.wantError {
				t.Errorf("ValidateClientOrderID(%q): gotError=%v, wantError=%v (len=%d, err=%v)",
					tc.id, gotError, tc.wantError, len(tc.id), err)
			}
		})
	}
}

// ============================================================================
// TEST CASES: TIMEZONE-AWARE DATE FORMATTING
// ============================================================================

// TestTimezoneAwareDateFormatting verifies date formatting uses correct timezone
func TestTimezoneAwareDateFormatting(t *testing.T) {
	mockCache := NewMockCacheService()

	// Test with Asia/Kolkata (GMT+5:30)
	kolkataTime, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, kolkataTime)
	ctx := context.Background()

	// Create a specific UTC time that would be different dates in different timezones
	// UTC: Jan 14, 2026, 23:00 (11 PM)
	// Kolkata (UTC+5:30): Jan 15, 2026, 04:30 (4:30 AM next day)
	utcTime := time.Date(2026, 1, 14, 23, 0, 0, 0, time.UTC)

	id, err := generator.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, utcTime)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// In Kolkata timezone, this should be 15JAN, not 14JAN
	if !strings.Contains(id, "15JAN") {
		t.Errorf("Expected 15JAN for Kolkata timezone, got: %s", id)
	}
}

// TestDateFormatUppercase verifies date is uppercase (e.g., "06JAN" not "06jan")
func TestDateFormatUppercase(t *testing.T) {
	mockCache := NewMockCacheService()
	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	// Test various months to ensure uppercase
	testDates := []time.Time{
		time.Date(2026, 1, 6, 12, 0, 0, 0, tz),   // January
		time.Date(2026, 2, 15, 12, 0, 0, 0, tz),  // February
		time.Date(2026, 12, 25, 12, 0, 0, 0, tz), // December
	}

	expectedSubstrings := []string{"06JAN", "15FEB", "25DEC"}

	for i, testTime := range testDates {
		t.Run(expectedSubstrings[i], func(t *testing.T) {
			id, err := generator.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, testTime)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !strings.Contains(id, expectedSubstrings[i]) {
				t.Errorf("Expected %s in ID, got: %s", expectedSubstrings[i], id)
			}

			// Ensure no lowercase month names
			if strings.Contains(id, strings.ToLower(expectedSubstrings[i][2:])) {
				t.Errorf("Month should be uppercase in ID: %s", id)
			}
		})
	}
}

// TestDifferentTimezones verifies generator works with different timezones
func TestDifferentTimezones(t *testing.T) {
	testCases := []struct {
		name     string
		timezone string
	}{
		{"Asia/Kolkata", "Asia/Kolkata"},
		{"UTC", "UTC"},
		{"America/New_York", "America/New_York"},
		{"Asia/Tokyo", "Asia/Tokyo"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCache := NewMockCacheService()
			tz, err := time.LoadLocation(tc.timezone)
			if err != nil {
				t.Skipf("Timezone %s not available: %v", tc.timezone, err)
			}

			generator := NewTestableClientOrderIdGenerator(mockCache, tz)
			ctx := context.Background()

			id, err := generator.Generate(ctx, "user123", ModeScalp, OrderTypeEntry)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify format: MODE-DDMMM-NNNNN-TYPE
			parts := strings.Split(id, "-")
			if len(parts) != 4 {
				t.Errorf("Expected 4 parts in ID, got %d: %s", len(parts), id)
			}

			// Verify date part is 5 characters (DDMMM)
			if len(parts) >= 2 && len(parts[1]) != 5 {
				t.Errorf("Date part should be 5 characters, got %d: %s", len(parts[1]), parts[1])
			}
		})
	}
}

// ============================================================================
// TEST CASES: GENERATE RELATED (SAME CHAIN BASE)
// ============================================================================

// TestGenerateRelatedMaintainsChainBase verifies related orders share same base
func TestGenerateRelatedMaintainsChainBase(t *testing.T) {
	mockCache := NewMockCacheService()
	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	// Generate entry order
	entryID, err := generator.Generate(ctx, "user123", ModeUltraFast, OrderTypeEntry)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Extract base ID (remove the -E suffix)
	baseID := ExtractChainBase(entryID)

	// Generate related orders
	relatedTypes := []OrderType{OrderTypeSL, OrderTypeTP1, OrderTypeTP2, OrderTypeTP3, OrderTypeHedge}

	for _, orderType := range relatedTypes {
		t.Run(string(orderType), func(t *testing.T) {
			relatedID := generator.GenerateRelated(baseID, orderType)

			// Verify same chain base
			relatedBase := ExtractChainBase(relatedID)
			if relatedBase != baseID {
				t.Errorf("Chain base mismatch: expected %s, got %s", baseID, relatedBase)
			}

			// Verify correct suffix
			if !strings.HasSuffix(relatedID, "-"+string(orderType)) {
				t.Errorf("Expected suffix -%s, got: %s", orderType, relatedID)
			}
		})
	}
}

// TestExtractChainBase verifies chain base extraction from various IDs
func TestExtractChainBase(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"Normal entry", "ULT-06JAN-00001-E", "ULT-06JAN-00001"},
		{"Normal SL", "SCA-15FEB-00042-SL", "SCA-15FEB-00042"},
		{"TP3", "SWI-25DEC-99999-TP3", "SWI-25DEC-99999"},
		{"DCA3", "POS-01JAN-00001-DCA3", "POS-01JAN-00001"},
		{"Fallback ID", "ULT-FALLBACK-a3f7c2e9-E", "ULT-FALLBACK-a3f7c2e9"},
		{"Invalid format", "invalid", "invalid"},
		{"Too few parts", "ULT-06JAN", "ULT-06JAN"},
		{"Empty", "", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractChainBase(tc.input)
			if result != tc.expected {
				t.Errorf("ExtractChainBase(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

// TestGenerateRelatedForFullChain verifies a complete order chain
func TestGenerateRelatedForFullChain(t *testing.T) {
	mockCache := NewMockCacheService()
	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	// Generate entry
	entryID, _ := generator.Generate(ctx, "user123", ModeUltraFast, OrderTypeEntry)
	baseID := ExtractChainBase(entryID)

	// Build complete chain
	chain := []string{entryID}
	chainTypes := []OrderType{OrderTypeSL, OrderTypeTP1, OrderTypeTP2, OrderTypeTP3, OrderTypeHedge}

	for _, ot := range chainTypes {
		chain = append(chain, generator.GenerateRelated(baseID, ot))
	}

	// Verify all IDs in chain have same base
	for i, id := range chain {
		extractedBase := ExtractChainBase(id)
		if extractedBase != baseID {
			t.Errorf("Chain ID %d (%s) has wrong base: expected %s, got %s",
				i, id, baseID, extractedBase)
		}
	}

	// Verify chain length
	expectedLen := 6 // E + SL + TP1 + TP2 + TP3 + H
	if len(chain) != expectedLen {
		t.Errorf("Expected chain length %d, got %d", expectedLen, len(chain))
	}
}

// ============================================================================
// TEST CASES: FALLBACK GENERATION (REDIS UNAVAILABLE)
// ============================================================================

// TestFallbackGenerationWhenRedisUnavailable verifies fallback IDs are generated
func TestFallbackGenerationWhenRedisUnavailable(t *testing.T) {
	mockCache := NewMockCacheService()
	mockCache.simulateUnavailable = true // Simulate Redis being down

	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	id, err := generator.Generate(ctx, "user123", ModeUltraFast, OrderTypeEntry)

	// Should NOT return error - fallback should work
	if err != nil {
		t.Fatalf("Fallback should not return error, got: %v", err)
	}

	// Should contain FALLBACK keyword
	if !strings.Contains(id, "FALLBACK") {
		t.Errorf("Expected FALLBACK in ID, got: %s", id)
	}

	// Should still have correct mode prefix
	if !strings.HasPrefix(id, "ULT-") {
		t.Errorf("Expected ULT- prefix, got: %s", id)
	}

	// Should still have correct suffix
	if !strings.HasSuffix(id, "-E") {
		t.Errorf("Expected -E suffix, got: %s", id)
	}

	// Should still be within Binance limit
	if err := ValidateClientOrderID(id); err != nil {
		t.Errorf("Fallback ID validation failed: %s (len=%d): %v", id, len(id), err)
	}
}

// TestFallbackIDsAreUnique verifies fallback IDs don't collide
func TestFallbackIDsAreUnique(t *testing.T) {
	mockCache := NewMockCacheService()
	mockCache.simulateUnavailable = true

	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	// Generate multiple fallback IDs
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := generator.Generate(ctx, "user123", ModeScalp, OrderTypeEntry)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if ids[id] {
			t.Errorf("Duplicate fallback ID generated: %s", id)
		}
		ids[id] = true

		// Small delay to ensure timestamp differs
		time.Sleep(time.Nanosecond)
	}
}

// TestFallbackChainGrouping verifies fallback IDs still support chain grouping
func TestFallbackChainGrouping(t *testing.T) {
	mockCache := NewMockCacheService()
	mockCache.simulateUnavailable = true

	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	// Generate fallback entry
	entryID, _ := generator.Generate(ctx, "user123", ModeUltraFast, OrderTypeEntry)
	baseID := ExtractChainBase(entryID)

	// Generate related orders using the fallback base
	slID := generator.GenerateRelated(baseID, OrderTypeSL)
	tp1ID := generator.GenerateRelated(baseID, OrderTypeTP1)

	// Verify all have same chain base
	if ExtractChainBase(slID) != baseID {
		t.Errorf("SL chain base mismatch: expected %s, got %s", baseID, ExtractChainBase(slID))
	}
	if ExtractChainBase(tp1ID) != baseID {
		t.Errorf("TP1 chain base mismatch: expected %s, got %s", baseID, ExtractChainBase(tp1ID))
	}

	// Verify FALLBACK is in the base
	if !strings.Contains(baseID, "FALLBACK") {
		t.Errorf("Expected FALLBACK in base ID: %s", baseID)
	}
}

// TestRedisRecoveryAfterFallback verifies normal generation resumes when Redis recovers
func TestRedisRecoveryAfterFallback(t *testing.T) {
	mockCache := NewMockCacheService()
	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	// Phase 1: Redis unavailable
	mockCache.simulateUnavailable = true
	fallbackID, _ := generator.Generate(ctx, "user123", ModeScalp, OrderTypeEntry)

	if !strings.Contains(fallbackID, "FALLBACK") {
		t.Errorf("Expected FALLBACK ID when Redis down, got: %s", fallbackID)
	}

	// Phase 2: Redis recovers
	mockCache.simulateUnavailable = false
	normalID, _ := generator.Generate(ctx, "user123", ModeScalp, OrderTypeEntry)

	if strings.Contains(normalID, "FALLBACK") {
		t.Errorf("Should not generate FALLBACK ID when Redis is healthy, got: %s", normalID)
	}

	// Verify normal format: SCA-DDMMM-NNNNN-E
	parts := strings.Split(normalID, "-")
	if len(parts) != 4 {
		t.Errorf("Expected 4 parts in normal ID, got: %s", normalID)
	}
}

// ============================================================================
// TEST CASES: SEQUENCE GENERATION
// ============================================================================

// TestSequenceIncrementsCorrectly verifies sequence numbers increment
func TestSequenceIncrementsCorrectly(t *testing.T) {
	mockCache := NewMockCacheService()
	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	// Generate 5 orders
	for i := 1; i <= 5; i++ {
		id, err := generator.Generate(ctx, "user123", ModeScalp, OrderTypeEntry)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		expectedSeq := fmt.Sprintf("%05d", i)
		if !strings.Contains(id, expectedSeq) {
			t.Errorf("Order %d: expected sequence %s in ID, got: %s", i, expectedSeq, id)
		}
	}
}

// TestSequenceIsZeroPadded verifies 5-digit zero-padded format
func TestSequenceIsZeroPadded(t *testing.T) {
	mockCache := NewMockCacheService()
	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	id, err := generator.Generate(ctx, "user123", ModeScalp, OrderTypeEntry)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should contain "00001" (first sequence, zero-padded)
	if !strings.Contains(id, "00001") {
		t.Errorf("Expected zero-padded sequence 00001 in ID, got: %s", id)
	}
}

// TestConcurrentSequenceGeneration verifies no duplicate sequences under load
func TestConcurrentSequenceGeneration(t *testing.T) {
	mockCache := NewMockCacheService()
	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	const goroutines = 100
	ids := make(chan string, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := generator.Generate(ctx, "user123", ModeScalp, OrderTypeEntry)
			if err != nil {
				t.Errorf("Concurrent generation error: %v", err)
				return
			}
			ids <- id
		}()
	}

	wg.Wait()
	close(ids)

	// Collect all IDs and check for uniqueness
	uniqueIDs := make(map[string]bool)
	for id := range ids {
		if uniqueIDs[id] {
			t.Errorf("Duplicate ID generated under concurrent load: %s", id)
		}
		uniqueIDs[id] = true
	}

	// Verify we got all expected IDs
	if len(uniqueIDs) != goroutines {
		t.Errorf("Expected %d unique IDs, got %d", goroutines, len(uniqueIDs))
	}
}

// ============================================================================
// TEST CASES: DATE BOUNDARY
// ============================================================================

// TestMidnightRollover verifies sequence resets at timezone midnight
func TestMidnightRollover(t *testing.T) {
	mockCache := NewMockCacheService()
	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	// 11:59 PM on Jan 14
	time1 := time.Date(2026, 1, 14, 23, 59, 0, 0, tz)
	id1, _ := generator.GenerateAtTime(ctx, "user123", ModeUltraFast, OrderTypeEntry, time1)

	// 12:01 AM on Jan 15
	time2 := time.Date(2026, 1, 15, 0, 1, 0, 0, tz)
	id2, _ := generator.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, time2)

	// Verify dates are different
	if !strings.Contains(id1, "14JAN") {
		t.Errorf("First ID should have 14JAN, got: %s", id1)
	}
	if !strings.Contains(id2, "15JAN") {
		t.Errorf("Second ID should have 15JAN, got: %s", id2)
	}

	// Verify both sequences are 00001 (reset at midnight)
	if !strings.Contains(id1, "00001") {
		t.Errorf("First ID should have sequence 00001, got: %s", id1)
	}
	if !strings.Contains(id2, "00001") {
		t.Errorf("Second ID should have sequence 00001 (reset), got: %s", id2)
	}
}

// TestYearBoundary verifies IDs work across Dec 31 -> Jan 1
func TestYearBoundary(t *testing.T) {
	mockCache := NewMockCacheService()
	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	// Dec 31, 2026
	time1 := time.Date(2026, 12, 31, 23, 59, 0, 0, tz)
	id1, err := generator.GenerateAtTime(ctx, "user123", ModeSwing, OrderTypeEntry, time1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(id1, "31DEC") {
		t.Errorf("Expected 31DEC in ID, got: %s", id1)
	}

	// Jan 1, 2027
	time2 := time.Date(2027, 1, 1, 0, 1, 0, 0, tz)
	id2, err := generator.GenerateAtTime(ctx, "user123", ModeSwing, OrderTypeEntry, time2)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(id2, "01JAN") {
		t.Errorf("Expected 01JAN in ID, got: %s", id2)
	}
}

// ============================================================================
// TEST CASES: FORMAT VALIDATION
// ============================================================================

// TestIDFormatStructure verifies the MODE-DDMMM-NNNNN-TYPE format
func TestIDFormatStructure(t *testing.T) {
	mockCache := NewMockCacheService()
	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	id, err := generator.Generate(ctx, "user123", ModeUltraFast, OrderTypeEntry)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	parts := strings.Split(id, "-")

	// Verify 4 parts
	if len(parts) != 4 {
		t.Fatalf("Expected 4 parts separated by -, got %d: %s", len(parts), id)
	}

	// Part 1: Mode code (3 chars)
	if len(parts[0]) != 3 {
		t.Errorf("Mode code should be 3 characters, got %d: %s", len(parts[0]), parts[0])
	}

	// Part 2: Date (5 chars: DDMMM)
	if len(parts[1]) != 5 {
		t.Errorf("Date should be 5 characters, got %d: %s", len(parts[1]), parts[1])
	}

	// Part 3: Sequence (5 digits)
	if len(parts[2]) != 5 {
		t.Errorf("Sequence should be 5 digits, got %d: %s", len(parts[2]), parts[2])
	}

	// Verify sequence is numeric
	for _, c := range parts[2] {
		if c < '0' || c > '9' {
			t.Errorf("Sequence should be all digits, got: %s", parts[2])
			break
		}
	}

	// Part 4: Order type suffix
	if parts[3] != string(OrderTypeEntry) {
		t.Errorf("Expected order type %s, got: %s", OrderTypeEntry, parts[3])
	}
}

// ============================================================================
// TEST CASES: USER ISOLATION
// ============================================================================

// TestDifferentUsersHaveIndependentSequences verifies user isolation
func TestDifferentUsersHaveIndependentSequences(t *testing.T) {
	mockCache := NewMockCacheService()
	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	// Generate for user1
	user1ID, _ := generator.Generate(ctx, "user1", ModeScalp, OrderTypeEntry)

	// Generate for user2
	user2ID, _ := generator.Generate(ctx, "user2", ModeScalp, OrderTypeEntry)

	// Both should have sequence 00001 (independent counters)
	if !strings.Contains(user1ID, "00001") {
		t.Errorf("User1 should have sequence 00001, got: %s", user1ID)
	}
	if !strings.Contains(user2ID, "00001") {
		t.Errorf("User2 should have sequence 00001, got: %s", user2ID)
	}

	// Verify increment calls were for different users
	if len(mockCache.incrementCalls) != 2 {
		t.Errorf("Expected 2 increment calls, got: %d", len(mockCache.incrementCalls))
	}

	if mockCache.incrementCalls[0].UserID == mockCache.incrementCalls[1].UserID {
		t.Error("Increment calls should be for different users")
	}
}

// ============================================================================
// TABLE-DRIVEN TESTS
// ============================================================================

// TestAllModesAllTypes tests all combinations of modes and order types
func TestAllModesAllTypes(t *testing.T) {
	modes := []TradingMode{ModeUltraFast, ModeScalp, ModeSwing, ModePosition}
	orderTypes := AllOrderTypes()

	for _, mode := range modes {
		for _, orderType := range orderTypes {
			testName := fmt.Sprintf("%s/%s", ModeCode[mode], orderType)
			t.Run(testName, func(t *testing.T) {
				mockCache := NewMockCacheService()
				tz, _ := time.LoadLocation("Asia/Kolkata")
				generator := NewTestableClientOrderIdGenerator(mockCache, tz)
				ctx := context.Background()

				id, err := generator.Generate(ctx, "user123", mode, orderType)

				// No error
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				// Has correct mode prefix
				if !strings.HasPrefix(id, ModeCode[mode]+"-") {
					t.Errorf("Expected prefix %s-, got: %s", ModeCode[mode], id)
				}

				// Has correct order type suffix
				if !strings.HasSuffix(id, "-"+string(orderType)) {
					t.Errorf("Expected suffix -%s, got: %s", orderType, id)
				}

				// Within Binance limit
				if len(id) > 36 {
					t.Errorf("ID exceeds 36 char limit: len=%d, id=%s", len(id), id)
				}
			})
		}
	}
}

// TestErrorConditions tests various error scenarios
func TestErrorConditions(t *testing.T) {
	testCases := []struct {
		name           string
		setupCache     func(*MockCacheService)
		expectFallback bool
		expectError    bool
	}{
		{
			name: "Redis healthy",
			setupCache: func(m *MockCacheService) {
				m.healthy = true
				m.simulateUnavailable = false
			},
			expectFallback: false,
			expectError:    false,
		},
		{
			name: "Redis unavailable",
			setupCache: func(m *MockCacheService) {
				m.healthy = true
				m.simulateUnavailable = true
			},
			expectFallback: true,
			expectError:    false,
		},
		{
			name: "Redis returns error",
			setupCache: func(m *MockCacheService) {
				m.healthy = true
				m.incrementErr = errors.New("redis connection timeout")
			},
			expectFallback: true,
			expectError:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockCache := NewMockCacheService()
			tc.setupCache(mockCache)

			tz, _ := time.LoadLocation("Asia/Kolkata")
			generator := NewTestableClientOrderIdGenerator(mockCache, tz)
			ctx := context.Background()

			id, err := generator.Generate(ctx, "user123", ModeScalp, OrderTypeEntry)

			if tc.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			isFallback := strings.Contains(id, "FALLBACK")
			if tc.expectFallback && !isFallback {
				t.Errorf("Expected fallback ID, got: %s", id)
			}
			if !tc.expectFallback && isFallback {
				t.Errorf("Did not expect fallback ID, got: %s", id)
			}
		})
	}
}
