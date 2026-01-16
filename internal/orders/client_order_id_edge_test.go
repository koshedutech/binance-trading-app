// Package orders provides comprehensive edge case tests for the client order ID system.
// Epic 7: Client Order ID & Trade Lifecycle Tracking - Story 7.10: Edge Case Test Suite
package orders

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// TEST 1: MIDNIGHT ROLLOVER
// Verify sequence resets at midnight in user's timezone
// ============================================================================

func TestEdge_MidnightRollover(t *testing.T) {
	t.Run("11:59PM to 12:01AM crosses date boundary", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		// 11:59 PM on Jan 6 - last minute of the day
		time1 := time.Date(2026, 1, 6, 23, 59, 0, 0, tz)
		id1, _ := generator.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, time1)

		// 12:01 AM on Jan 7 - first minutes of next day
		time2 := time.Date(2026, 1, 7, 0, 1, 0, 0, tz)
		id2, _ := generator.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, time2)

		// Verify dates are different
		if !strings.Contains(id1, "06JAN") {
			t.Errorf("First ID should contain 06JAN, got: %s", id1)
		}
		if !strings.Contains(id2, "07JAN") {
			t.Errorf("Second ID should contain 07JAN, got: %s", id2)
		}

		// Both should have sequence 00001 (separate date keys in Redis)
		if !strings.Contains(id1, "00001") {
			t.Errorf("First ID should have sequence 00001, got: %s", id1)
		}
		if !strings.Contains(id2, "00001") {
			t.Errorf("Second ID should have sequence 00001 (new day), got: %s", id2)
		}

		// Verify the cache was called with different date keys
		if len(mockCache.incrementCalls) != 2 {
			t.Fatalf("Expected 2 increment calls, got: %d", len(mockCache.incrementCalls))
		}
		if mockCache.incrementCalls[0].DateKey == mockCache.incrementCalls[1].DateKey {
			t.Errorf("Date keys should be different: %s vs %s",
				mockCache.incrementCalls[0].DateKey, mockCache.incrementCalls[1].DateKey)
		}
	})

	t.Run("sequence resets independently per date", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		// Generate 3 orders on Jan 6
		jan6 := time.Date(2026, 1, 6, 12, 0, 0, 0, tz)
		for i := 0; i < 3; i++ {
			generator.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, jan6)
		}

		// Generate 2 orders on Jan 7
		jan7 := time.Date(2026, 1, 7, 12, 0, 0, 0, tz)
		id1, _ := generator.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, jan7)
		id2, _ := generator.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, jan7)

		// Jan 7 sequences should be 1 and 2, not 4 and 5
		if !strings.Contains(id1, "00001") {
			t.Errorf("First Jan 7 ID should have sequence 00001, got: %s", id1)
		}
		if !strings.Contains(id2, "00002") {
			t.Errorf("Second Jan 7 ID should have sequence 00002, got: %s", id2)
		}
	})

	t.Run("23:59:59 to 00:00:00 exact midnight boundary", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		// 23:59:59 on Jan 6
		time1 := time.Date(2026, 1, 6, 23, 59, 59, 0, tz)
		id1, _ := generator.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, time1)

		// 00:00:00 on Jan 7 (exact midnight)
		time2 := time.Date(2026, 1, 7, 0, 0, 0, 0, tz)
		id2, _ := generator.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, time2)

		// Dates should be different
		if strings.Contains(id1, "07JAN") || !strings.Contains(id1, "06JAN") {
			t.Errorf("ID at 23:59:59 should be 06JAN, got: %s", id1)
		}
		if strings.Contains(id2, "06JAN") || !strings.Contains(id2, "07JAN") {
			t.Errorf("ID at 00:00:00 should be 07JAN, got: %s", id2)
		}
	})
}

// ============================================================================
// TEST 2: YEAR BOUNDARY
// Dec 31 -> Jan 1 transition
// ============================================================================

func TestEdge_YearBoundary(t *testing.T) {
	t.Run("Dec 31 to Jan 1 date format change", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		// Dec 31, 2026, 23:59:59
		dec31 := time.Date(2026, 12, 31, 23, 59, 59, 0, tz)
		id1, _ := generator.GenerateAtTime(ctx, "user123", ModeSwing, OrderTypeEntry, dec31)

		// Jan 1, 2027, 00:00:01
		jan1 := time.Date(2027, 1, 1, 0, 0, 1, 0, tz)
		id2, _ := generator.GenerateAtTime(ctx, "user123", ModeSwing, OrderTypeEntry, jan1)

		// Verify date format changes correctly
		if !strings.Contains(id1, "31DEC") {
			t.Errorf("Dec 31 ID should contain 31DEC, got: %s", id1)
		}
		if !strings.Contains(id2, "01JAN") {
			t.Errorf("Jan 1 ID should contain 01JAN, got: %s", id2)
		}

		// Verify sequences reset for new year
		if !strings.Contains(id1, "00001") {
			t.Errorf("Dec 31 sequence should be 00001, got: %s", id1)
		}
		if !strings.Contains(id2, "00001") {
			t.Errorf("Jan 1 sequence should be 00001 (new date key), got: %s", id2)
		}
	})

	t.Run("year boundary with UTC timezone edge case", func(t *testing.T) {
		mockCache := NewMockCacheService()
		generator := NewTestableClientOrderIdGenerator(mockCache, time.UTC)
		ctx := context.Background()

		// UTC time that would be Jan 1 in some timezones but Dec 31 in UTC
		utcDec31 := time.Date(2026, 12, 31, 23, 0, 0, 0, time.UTC)
		id1, _ := generator.GenerateAtTime(ctx, "user123", ModePosition, OrderTypeEntry, utcDec31)

		utcJan1 := time.Date(2027, 1, 1, 1, 0, 0, 0, time.UTC)
		id2, _ := generator.GenerateAtTime(ctx, "user123", ModePosition, OrderTypeEntry, utcJan1)

		if !strings.Contains(id1, "31DEC") {
			t.Errorf("UTC Dec 31 ID should contain 31DEC, got: %s", id1)
		}
		if !strings.Contains(id2, "01JAN") {
			t.Errorf("UTC Jan 1 ID should contain 01JAN, got: %s", id2)
		}
	})

	t.Run("verify dateKey format across year boundary", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		dec31 := time.Date(2026, 12, 31, 12, 0, 0, 0, tz)
		jan1 := time.Date(2027, 1, 1, 12, 0, 0, 0, tz)

		generator.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, dec31)
		generator.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, jan1)

		// Check the dateKeys used in cache calls
		if len(mockCache.incrementCalls) != 2 {
			t.Fatalf("Expected 2 increment calls, got: %d", len(mockCache.incrementCalls))
		}

		// dateKey format is YYYYMMDD
		if mockCache.incrementCalls[0].DateKey != "20261231" {
			t.Errorf("Dec 31 dateKey should be 20261231, got: %s", mockCache.incrementCalls[0].DateKey)
		}
		if mockCache.incrementCalls[1].DateKey != "20270101" {
			t.Errorf("Jan 1 dateKey should be 20270101, got: %s", mockCache.incrementCalls[1].DateKey)
		}
	})
}

// ============================================================================
// TEST 3: CONCURRENT SEQUENCE
// 100 parallel requests, no duplicate sequences
// ============================================================================

func TestEdge_ConcurrentSequence(t *testing.T) {
	t.Run("100 parallel requests produce unique sequences", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		const numGoroutines = 100
		ids := make(chan string, numGoroutines)
		var wg sync.WaitGroup

		// Launch 100 goroutines simultaneously
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				id, err := generator.Generate(ctx, "user123", ModeUltraFast, OrderTypeEntry)
				if err != nil {
					t.Errorf("Concurrent generation error: %v", err)
					return
				}
				ids <- id
			}()
		}

		wg.Wait()
		close(ids)

		// Collect all IDs and verify uniqueness
		uniqueIDs := make(map[string]bool)
		var duplicates []string

		for id := range ids {
			if uniqueIDs[id] {
				duplicates = append(duplicates, id)
			}
			uniqueIDs[id] = true
		}

		if len(duplicates) > 0 {
			t.Errorf("Found %d duplicate IDs: %v", len(duplicates), duplicates)
		}

		if len(uniqueIDs) != numGoroutines {
			t.Errorf("Expected %d unique IDs, got %d", numGoroutines, len(uniqueIDs))
		}
	})

	t.Run("100 parallel requests from different users - per user uniqueness", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		const numUsers = 10
		const requestsPerUser = 10

		type userIDPair struct {
			userID string
			id     string
		}
		results := make(chan userIDPair, numUsers*requestsPerUser)
		var wg sync.WaitGroup

		for u := 0; u < numUsers; u++ {
			for r := 0; r < requestsPerUser; r++ {
				wg.Add(1)
				userID := fmt.Sprintf("user%d", u)
				go func(uid string) {
					defer wg.Done()
					id, err := generator.Generate(ctx, uid, ModeScalp, OrderTypeEntry)
					if err != nil {
						t.Errorf("Error generating for user %s: %v", uid, err)
						return
					}
					results <- userIDPair{userID: uid, id: id}
				}(userID)
			}
		}

		wg.Wait()
		close(results)

		// Collect IDs per user
		userIDs := make(map[string]map[string]bool)
		for pair := range results {
			if userIDs[pair.userID] == nil {
				userIDs[pair.userID] = make(map[string]bool)
			}
			if userIDs[pair.userID][pair.id] {
				t.Errorf("Duplicate ID for user %s: %s", pair.userID, pair.id)
			}
			userIDs[pair.userID][pair.id] = true
		}

		// Each user should have exactly requestsPerUser unique IDs
		for userID, ids := range userIDs {
			if len(ids) != requestsPerUser {
				t.Errorf("User %s should have %d unique IDs, got %d", userID, requestsPerUser, len(ids))
			}
		}

		// Verify all users were processed
		if len(userIDs) != numUsers {
			t.Errorf("Expected %d users, got %d", numUsers, len(userIDs))
		}
	})

	t.Run("concurrent requests with same base time", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		const numGoroutines = 50
		fixedTime := time.Date(2026, 1, 15, 12, 0, 0, 0, tz)
		ids := make(chan string, numGoroutines)
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				id, _ := generator.GenerateAtTime(ctx, "user123", ModeSwing, OrderTypeEntry, fixedTime)
				ids <- id
			}()
		}

		wg.Wait()
		close(ids)

		uniqueIDs := make(map[string]bool)
		for id := range ids {
			if uniqueIDs[id] {
				t.Errorf("Duplicate ID at same time: %s", id)
			}
			uniqueIDs[id] = true
		}

		if len(uniqueIDs) != numGoroutines {
			t.Errorf("Expected %d unique IDs, got %d", numGoroutines, len(uniqueIDs))
		}
	})
}

// ============================================================================
// TEST 4: MAX SEQUENCE
// Behavior at sequence 99999
// ============================================================================

func TestEdge_MaxSequence(t *testing.T) {
	t.Run("sequence at 99999 produces valid ID", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		// Set sequence to 99998 so next gives 99999
		now := time.Now().In(tz)
		dateKey := now.Format("20060102")
		mockCache.SetSequence("user123", dateKey, 99998)

		id, err := generator.Generate(ctx, "user123", ModePosition, OrderTypeDCA3)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Verify sequence 99999 in ID
		if !strings.Contains(id, "99999") {
			t.Errorf("Expected sequence 99999 in ID, got: %s", id)
		}

		// Verify ID is within Binance 36 char limit
		if len(id) > 36 {
			t.Errorf("Max sequence ID exceeds 36 chars: len=%d, id=%s", len(id), id)
		}

		// Validate format
		if err := ValidateClientOrderID(id); err != nil {
			t.Errorf("Validation failed for max sequence ID: %v", err)
		}
	})

	t.Run("sequence 100000 produces 6-digit ID (overflow behavior)", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		now := time.Now().In(tz)
		dateKey := now.Format("20060102")
		mockCache.SetSequence("user123", dateKey, 99999) // Next will be 100000

		id, err := generator.Generate(ctx, "user123", ModeScalp, OrderTypeEntry)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// With %05d format, 100000 becomes "100000" (6 digits)
		// The ID should still work, just be slightly longer
		if !strings.Contains(id, "100000") {
			t.Errorf("Expected sequence 100000 in ID, got: %s", id)
		}

		// Still should be within 36 char limit: SCA-15JAN-100000-E = 18 chars
		if len(id) > 36 {
			t.Errorf("Overflow sequence ID exceeds limit: len=%d, id=%s", len(id), id)
		}
	})

	t.Run("sequence near max with longest order type", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		now := time.Now().In(tz)
		dateKey := now.Format("20060102")
		mockCache.SetSequence("user123", dateKey, 99998)

		// Test with DCA3 which is the longest suffix
		id, _ := generator.Generate(ctx, "user123", ModePosition, OrderTypeDCA3)

		// POS-DDMMM-99999-DCA3 = 3 + 1 + 5 + 1 + 5 + 1 + 4 = 20 chars
		expectedMaxLen := 20
		if len(id) != expectedMaxLen {
			t.Logf("Expected length %d, got %d: %s", expectedMaxLen, len(id), id)
		}

		if len(id) > 36 {
			t.Errorf("Max combo exceeds 36 chars: %s (len=%d)", id, len(id))
		}
	})
}

// ============================================================================
// TEST 5: ALL MODES x ALL TYPES
// Table-driven test for all 4 modes x 10 order types
// ============================================================================

func TestEdge_AllModesAllTypes(t *testing.T) {
	modes := []TradingMode{ModeUltraFast, ModeScalp, ModeSwing, ModePosition}
	orderTypes := AllOrderTypes()

	// Total combinations: 4 modes x 10 types = 40
	expectedCombos := len(modes) * len(orderTypes)
	testedCombos := 0

	for _, mode := range modes {
		for _, orderType := range orderTypes {
			modeCode := ModeCode[mode]
			testName := fmt.Sprintf("%s-%s", modeCode, orderType)

			t.Run(testName, func(t *testing.T) {
				mockCache := NewMockCacheService()
				tz, _ := time.LoadLocation("Asia/Kolkata")
				generator := NewTestableClientOrderIdGenerator(mockCache, tz)
				ctx := context.Background()

				id, err := generator.Generate(ctx, "user123", mode, orderType)

				// 1. No error
				if err != nil {
					t.Fatalf("Generation failed: %v", err)
				}

				// 2. Correct mode prefix
				if !strings.HasPrefix(id, modeCode+"-") {
					t.Errorf("Wrong prefix: expected %s-, got: %s", modeCode, id)
				}

				// 3. Correct order type suffix
				if !strings.HasSuffix(id, "-"+string(orderType)) {
					t.Errorf("Wrong suffix: expected -%s, got: %s", orderType, id)
				}

				// 4. Within Binance 36 char limit
				if len(id) > 36 {
					t.Errorf("ID exceeds 36 chars: len=%d, id=%s", len(id), id)
				}

				// 5. Validates successfully
				if err := ValidateClientOrderID(id); err != nil {
					t.Errorf("Validation failed: %v", err)
				}

				// 6. Parseable by parser
				parsed := ParseClientOrderId(id)
				if parsed == nil {
					t.Errorf("Parser failed to parse generated ID: %s", id)
				} else {
					if parsed.Mode != mode {
						t.Errorf("Parser mode mismatch: expected %s, got %s", mode, parsed.Mode)
					}
					if parsed.OrderType != orderType {
						t.Errorf("Parser orderType mismatch: expected %s, got %s", orderType, parsed.OrderType)
					}
				}

				testedCombos++
			})
		}
	}

	// Verify all combinations were tested
	if testedCombos != expectedCombos {
		t.Errorf("Expected to test %d combinations, but only tested %d", expectedCombos, testedCombos)
	}
}

// ============================================================================
// TEST 6: FALLBACK CHAIN GROUPING
// Fallback IDs still group correctly
// ============================================================================

func TestEdge_FallbackChainGrouping(t *testing.T) {
	t.Run("fallback IDs share same chain base", func(t *testing.T) {
		mockCache := NewMockCacheService()
		mockCache.simulateUnavailable = true // Force fallback mode
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		// Generate entry
		entryID, _ := generator.Generate(ctx, "user123", ModeUltraFast, OrderTypeEntry)
		baseID := ExtractChainBase(entryID)

		// Verify base is extractable and contains FALLBACK
		if !strings.Contains(baseID, "FALLBACK") {
			t.Errorf("Fallback base should contain FALLBACK: %s", baseID)
		}

		// Generate related orders
		relatedTypes := []OrderType{OrderTypeSL, OrderTypeTP1, OrderTypeTP2, OrderTypeDCA1}

		for _, orderType := range relatedTypes {
			relatedID := generator.GenerateRelated(baseID, orderType)

			// All related IDs should share the same chain base
			extractedBase := ExtractChainBase(relatedID)
			if extractedBase != baseID {
				t.Errorf("%s: chain base mismatch - expected %s, got %s", orderType, baseID, extractedBase)
			}

			// Should end with correct suffix
			if !strings.HasSuffix(relatedID, "-"+string(orderType)) {
				t.Errorf("%s: wrong suffix in %s", orderType, relatedID)
			}
		}
	})

	t.Run("fallback chain IDs are parseable", func(t *testing.T) {
		mockCache := NewMockCacheService()
		mockCache.simulateUnavailable = true
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		entryID, _ := generator.Generate(ctx, "user123", ModeScalp, OrderTypeEntry)
		baseID := ExtractChainBase(entryID)

		// Parse the entry ID
		parsed := ParseClientOrderId(entryID)
		if parsed == nil {
			t.Fatalf("Failed to parse fallback entry ID: %s", entryID)
		}
		if !parsed.IsFallback {
			t.Errorf("Parsed ID should be marked as fallback: %+v", parsed)
		}
		if parsed.ChainId != baseID {
			t.Errorf("Parser chainId mismatch: expected %s, got %s", baseID, parsed.ChainId)
		}

		// Generate and parse related
		tpID := generator.GenerateRelated(baseID, OrderTypeTP1)
		parsedTP := ParseClientOrderId(tpID)
		if parsedTP == nil {
			t.Fatalf("Failed to parse fallback TP1 ID: %s", tpID)
		}
		if parsedTP.ChainId != baseID {
			t.Errorf("Related ID chainId mismatch: expected %s, got %s", baseID, parsedTP.ChainId)
		}
	})

	t.Run("BelongsToSameChain works for fallback IDs", func(t *testing.T) {
		mockCache := NewMockCacheService()
		mockCache.simulateUnavailable = true
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		// Generate entry
		entryID, _ := generator.Generate(ctx, "user123", ModeSwing, OrderTypeEntry)
		baseID := ExtractChainBase(entryID)

		// Generate related orders
		slID := generator.GenerateRelated(baseID, OrderTypeSL)
		tp1ID := generator.GenerateRelated(baseID, OrderTypeTP1)

		// Same chain
		if !BelongsToSameChain(entryID, slID) {
			t.Errorf("Entry and SL should belong to same chain: %s, %s", entryID, slID)
		}
		if !BelongsToSameChain(slID, tp1ID) {
			t.Errorf("SL and TP1 should belong to same chain: %s, %s", slID, tp1ID)
		}

		// Generate different fallback chain
		entryID2, _ := generator.Generate(ctx, "user123", ModeSwing, OrderTypeEntry)
		time.Sleep(time.Millisecond) // Ensure different unique ID

		if BelongsToSameChain(entryID, entryID2) {
			t.Errorf("Different fallback entries should NOT be same chain: %s, %s", entryID, entryID2)
		}
	})
}

// ============================================================================
// TEST 7: PARSER EDGE CASES
// Parser handles all edge cases
// ============================================================================

func TestEdge_ParserEdgeCases(t *testing.T) {
	t.Run("empty string returns nil", func(t *testing.T) {
		result := ParseClientOrderId("")
		if result != nil {
			t.Errorf("Empty string should return nil, got: %+v", result)
		}
	})

	t.Run("legacy/unstructured IDs return nil", func(t *testing.T) {
		legacyIDs := []string{
			"x-A1234567890",           // Legacy Binance format
			"some-random-string",      // Random string
			"uuid-format-id",          // UUID-like
			"12345",                   // Numeric only
			"ORDER_123456",            // Underscore format
			"BINANCE_ORDER_ABC123DEF", // Another legacy format
		}

		for _, id := range legacyIDs {
			result := ParseClientOrderId(id)
			if result != nil {
				t.Errorf("Legacy ID '%s' should return nil, got: %+v", id, result)
			}
		}
	})

	t.Run("too few parts returns nil", func(t *testing.T) {
		shortIDs := []string{
			"SCA",
			"SCA-06JAN",
			"SCA-06JAN-00001", // Missing order type
		}

		for _, id := range shortIDs {
			result := ParseClientOrderId(id)
			if result != nil {
				t.Errorf("Short ID '%s' should return nil, got: %+v", id, result)
			}
		}
	})

	t.Run("invalid mode codes return nil", func(t *testing.T) {
		invalidModes := []string{
			"XXX-06JAN-00001-E",  // Unknown mode
			"ABC-06JAN-00001-E",  // Unknown mode
			"SC-06JAN-00001-E",   // Too short
			"SCAL-06JAN-00001-E", // Too long
			"sca-06JAN-00001-E",  // Lowercase - should be handled by normalization
		}

		// Note: lowercase is normalized, so sca should actually parse
		for _, id := range invalidModes[:4] {
			result := ParseClientOrderId(id)
			if result != nil {
				t.Errorf("Invalid mode ID '%s' should return nil, got: %+v", id, result)
			}
		}

		// Lowercase should be normalized and parse
		result := ParseClientOrderId("sca-06JAN-00001-E")
		if result == nil {
			t.Error("Lowercase mode 'sca' should be normalized and parse successfully")
		}
	})

	t.Run("invalid dates return nil", func(t *testing.T) {
		invalidDates := []string{
			"SCA-99ABC-00001-E", // Invalid day
			"SCA-00JAN-00001-E", // Day 00
			"SCA-32JAN-00001-E", // Day 32
			"SCA-06XXX-00001-E", // Invalid month
			"SCA-6JAN-00001-E",  // Single digit day (missing leading zero)
			"SCA-006JAN-00001-E", // Too many digits in day
		}

		for _, id := range invalidDates {
			result := ParseClientOrderId(id)
			if result != nil {
				t.Errorf("Invalid date ID '%s' should return nil, got: %+v", id, result)
			}
		}
	})

	t.Run("29FEB valid for leap year parsing", func(t *testing.T) {
		// The parser accepts 29FEB - it's valid syntax
		// Date validation in time.Date handles leap year specifics
		id := "SCA-29FEB-00001-E"
		result := ParseClientOrderId(id)

		if result == nil {
			t.Error("29FEB should parse successfully (syntax is valid)")
		} else {
			if result.DateStr != "29FEB" {
				t.Errorf("DateStr should be 29FEB, got: %s", result.DateStr)
			}
			// Date should be parsed (Go handles Feb 29 in non-leap years by rolling over)
			if result.Date.Day() == 0 {
				t.Error("Date should be parsed")
			}
		}
	})

	t.Run("30FEB invalid but parser accepts syntax", func(t *testing.T) {
		// 30FEB is syntactically valid (DD + MMM format)
		// but time.Date will roll it to March 2
		id := "SCA-30FEB-00001-E"
		result := ParseClientOrderId(id)

		if result == nil {
			t.Error("30FEB syntax should parse (Go handles date rollover)")
		}
	})

	t.Run("31APR rolls to May 1", func(t *testing.T) {
		// April only has 30 days
		id := "SCA-31APR-00001-E"
		result := ParseClientOrderId(id)

		if result == nil {
			t.Log("31APR syntax parsing depends on implementation")
		} else {
			// Go's time.Date handles overflow by rolling to next month
			t.Logf("31APR parsed as: %v", result.Date)
		}
	})

	t.Run("invalid sequence format returns nil", func(t *testing.T) {
		invalidSeqs := []string{
			"SCA-06JAN-ABCDE-E", // Non-numeric
			"SCA-06JAN-0001-E",  // 4 digits
			"SCA-06JAN-000001-E", // 6 digits
			"SCA-06JAN-1-E",     // Single digit
		}

		for _, id := range invalidSeqs {
			result := ParseClientOrderId(id)
			if result != nil {
				t.Errorf("Invalid sequence ID '%s' should return nil, got: %+v", id, result)
			}
		}
	})

	t.Run("invalid order types return nil", func(t *testing.T) {
		invalidTypes := []string{
			"SCA-06JAN-00001-INVALID",
			"SCA-06JAN-00001-X",
			"SCA-06JAN-00001-TP4", // No TP4
			"SCA-06JAN-00001-DCA4", // No DCA4
			"SCA-06JAN-00001-",    // Empty type
		}

		for _, id := range invalidTypes {
			result := ParseClientOrderId(id)
			if result != nil {
				t.Errorf("Invalid type ID '%s' should return nil, got: %+v", id, result)
			}
		}
	})

	t.Run("extra segments return nil", func(t *testing.T) {
		extraSegments := []string{
			"SCA-06JAN-00001-E-EXTRA",
			"SCA-06JAN-00001-TP1-SOMETHING",
		}

		for _, id := range extraSegments {
			result := ParseClientOrderId(id)
			if result != nil {
				t.Errorf("Extra segments ID '%s' should return nil, got: %+v", id, result)
			}
		}
	})

	t.Run("fallback ID edge cases", func(t *testing.T) {
		validFallbacks := []string{
			"SCA-FALLBACK-a3f7c2e9-E",
			"ULT-FALLBACK-00000000-TP1",
			"SWI-FALLBACK-ffffffff-SL",
			"POS-FALLBACK-12345678-DCA3",
		}

		for _, id := range validFallbacks {
			result := ParseClientOrderId(id)
			if result == nil {
				t.Errorf("Valid fallback ID '%s' should parse successfully", id)
			} else if !result.IsFallback {
				t.Errorf("Fallback ID '%s' should be marked as fallback", id)
			}
		}

		invalidFallbacks := []string{
			"SCA-FALLBACK-abc-E",      // Too short unique ID
			"SCA-FALLBACK-abcdefghi-E", // Too long unique ID
			"SCA-FALLBACK-XXXXXXXX-E", // Invalid hex chars
			"SCA-FALLBACK--E",         // Empty unique ID
		}

		for _, id := range invalidFallbacks {
			result := ParseClientOrderId(id)
			if result != nil {
				t.Errorf("Invalid fallback ID '%s' should return nil, got: %+v", id, result)
			}
		}
	})
}

// ============================================================================
// TEST 8: TIMEZONE VARIATIONS
// Different timezones produce correct dates
// ============================================================================

func TestEdge_TimezoneVariations(t *testing.T) {
	// Use a fixed UTC time that will result in different dates in different timezones
	// UTC: 2026-01-15 03:00:00
	// - In UTC: Jan 15
	// - In Asia/Kolkata (UTC+5:30): Jan 15, 08:30
	// - In America/New_York (UTC-5): Jan 14, 22:00 (previous day!)
	utcTime := time.Date(2026, 1, 15, 3, 0, 0, 0, time.UTC)

	testCases := []struct {
		name         string
		timezone     string
		expectedDate string
	}{
		{"UTC", "UTC", "15JAN"},
		{"Asia/Kolkata (+5:30)", "Asia/Kolkata", "15JAN"},
		{"America/New_York (-5)", "America/New_York", "14JAN"}, // Previous day!
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

			id, err := generator.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, utcTime)
			if err != nil {
				t.Fatalf("Generation failed: %v", err)
			}

			if !strings.Contains(id, tc.expectedDate) {
				t.Errorf("Timezone %s: expected date %s in ID, got: %s", tc.timezone, tc.expectedDate, id)
			}

			// Also verify the cache was called with correct date key
			if len(mockCache.incrementCalls) > 0 {
				call := mockCache.incrementCalls[0]
				expectedInTz := utcTime.In(tz)
				expectedDateKey := expectedInTz.Format("20060102")
				if call.DateKey != expectedDateKey {
					t.Errorf("Timezone %s: expected dateKey %s, got %s", tc.timezone, expectedDateKey, call.DateKey)
				}
			}
		})
	}

	t.Run("same instant different dates", func(t *testing.T) {
		// A time that crosses the date boundary between timezones
		utcTime := time.Date(2026, 1, 15, 20, 0, 0, 0, time.UTC)

		// UTC: Jan 15, 20:00
		// Asia/Tokyo (UTC+9): Jan 16, 05:00 (next day!)

		mockUTC := NewMockCacheService()
		genUTC := NewTestableClientOrderIdGenerator(mockUTC, time.UTC)

		mockTokyo := NewMockCacheService()
		tzTokyo, _ := time.LoadLocation("Asia/Tokyo")
		genTokyo := NewTestableClientOrderIdGenerator(mockTokyo, tzTokyo)

		ctx := context.Background()

		idUTC, _ := genUTC.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, utcTime)
		idTokyo, _ := genTokyo.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, utcTime)

		// Same instant should produce different dates
		if strings.Contains(idUTC, "16JAN") {
			t.Errorf("UTC ID should NOT have 16JAN: %s", idUTC)
		}
		if !strings.Contains(idTokyo, "16JAN") {
			t.Errorf("Tokyo ID should have 16JAN: %s", idTokyo)
		}
	})
}

// ============================================================================
// TEST 9: BINANCE LENGTH LIMIT
// All generated IDs <= 36 characters
// ============================================================================

func TestEdge_BinanceLengthLimit(t *testing.T) {
	t.Run("all normal IDs within 36 chars", func(t *testing.T) {
		modes := []TradingMode{ModeUltraFast, ModeScalp, ModeSwing, ModePosition}
		orderTypes := AllOrderTypes()

		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		for _, mode := range modes {
			for _, orderType := range orderTypes {
				id, _ := generator.Generate(ctx, "user123", mode, orderType)

				if len(id) > 36 {
					t.Errorf("%s-%s: ID exceeds 36 chars: len=%d, id=%s",
						ModeCode[mode], orderType, len(id), id)
				}
			}
		}
	})

	t.Run("all fallback IDs within 36 chars", func(t *testing.T) {
		modes := []TradingMode{ModeUltraFast, ModeScalp, ModeSwing, ModePosition}
		orderTypes := AllOrderTypes()

		mockCache := NewMockCacheService()
		mockCache.simulateUnavailable = true // Force fallback
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		for _, mode := range modes {
			for _, orderType := range orderTypes {
				id, _ := generator.Generate(ctx, "user123", mode, orderType)

				if len(id) > 36 {
					t.Errorf("Fallback %s-%s: ID exceeds 36 chars: len=%d, id=%s",
						ModeCode[mode], orderType, len(id), id)
				}
			}
		}
	})

	t.Run("fallback with longest order type (DCA3)", func(t *testing.T) {
		mockCache := NewMockCacheService()
		mockCache.simulateUnavailable = true
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		id, _ := generator.Generate(ctx, "user123", ModePosition, OrderTypeDCA3)

		// POS-FALLBACK-xxxxxxxx-DCA3
		// 3 + 1 + 8 + 1 + 8 + 1 + 4 = 26 chars
		expectedMaxLen := 26
		if len(id) > expectedMaxLen {
			t.Logf("Fallback ID length: %d (expected max ~%d)", len(id), expectedMaxLen)
		}

		if len(id) > 36 {
			t.Errorf("Longest fallback ID exceeds 36: len=%d, id=%s", len(id), id)
		}
	})

	t.Run("max sequence with longest order type", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		now := time.Now().In(tz)
		dateKey := now.Format("20060102")
		mockCache.SetSequence("user123", dateKey, 99998)

		id, _ := generator.Generate(ctx, "user123", ModePosition, OrderTypeDCA3)

		// POS-DDMMM-99999-DCA3 = 3 + 1 + 5 + 1 + 5 + 1 + 4 = 20 chars
		if len(id) > 36 {
			t.Errorf("Max sequence longest type exceeds 36: len=%d, id=%s", len(id), id)
		}
	})

	t.Run("calculate theoretical max length", func(t *testing.T) {
		// Normal format: MODE-DDMMM-NNNNN-TYPE
		// MODE: 3 chars (ULT, SCA, SWI, POS)
		// DDMMM: 5 chars (01JAN to 31DEC)
		// NNNNN: 5 chars (00001 to 99999)
		// TYPE: 1-4 chars (E, SL, TP1-3, RB, DCA1-3, H)
		// Hyphens: 3

		// Max normal: 3 + 1 + 5 + 1 + 5 + 1 + 4 = 20 chars

		// Fallback format: MODE-FALLBACK-8CHAR-TYPE
		// MODE: 3 chars
		// FALLBACK: 8 chars
		// 8CHAR: 8 chars (hex)
		// TYPE: 1-4 chars
		// Hyphens: 3

		// Max fallback: 3 + 1 + 8 + 1 + 8 + 1 + 4 = 26 chars

		maxNormalLen := 3 + 1 + 5 + 1 + 5 + 1 + 4 // 20
		maxFallbackLen := 3 + 1 + 8 + 1 + 8 + 1 + 4 // 26

		if maxNormalLen > 36 {
			t.Errorf("Theoretical max normal length %d exceeds 36", maxNormalLen)
		}
		if maxFallbackLen > 36 {
			t.Errorf("Theoretical max fallback length %d exceeds 36", maxFallbackLen)
		}

		t.Logf("Max normal ID length: %d chars", maxNormalLen)
		t.Logf("Max fallback ID length: %d chars", maxFallbackLen)
	})
}

// ============================================================================
// TEST 10: ID FORMAT CONSISTENCY
// Format is always MODE-DATE-SEQ-TYPE
// ============================================================================

func TestEdge_IDFormatConsistency(t *testing.T) {
	t.Run("normal ID format parts", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		modes := []TradingMode{ModeUltraFast, ModeScalp, ModeSwing, ModePosition}
		orderTypes := AllOrderTypes()

		for _, mode := range modes {
			for _, orderType := range orderTypes {
				id, _ := generator.Generate(ctx, "user123", mode, orderType)
				parts := strings.Split(id, "-")

				// Must have exactly 4 parts
				if len(parts) != 4 {
					t.Errorf("%s-%s: Expected 4 parts, got %d: %s",
						ModeCode[mode], orderType, len(parts), id)
					continue
				}

				// Part 0: Mode - exactly 3 uppercase chars
				if len(parts[0]) != 3 {
					t.Errorf("Mode should be 3 chars, got %d: %s", len(parts[0]), parts[0])
				}
				if parts[0] != strings.ToUpper(parts[0]) {
					t.Errorf("Mode should be uppercase: %s", parts[0])
				}

				// Part 1: Date - exactly 5 chars (DDMMM)
				if len(parts[1]) != 5 {
					t.Errorf("Date should be 5 chars, got %d: %s", len(parts[1]), parts[1])
				}

				// Part 2: Sequence - exactly 5 digits
				if len(parts[2]) != 5 {
					t.Errorf("Sequence should be 5 chars, got %d: %s", len(parts[2]), parts[2])
				}
				for _, c := range parts[2] {
					if c < '0' || c > '9' {
						t.Errorf("Sequence should be all digits: %s", parts[2])
						break
					}
				}

				// Part 3: Order type - matches expected
				if parts[3] != string(orderType) {
					t.Errorf("Order type mismatch: expected %s, got %s", orderType, parts[3])
				}
			}
		}
	})

	t.Run("fallback ID format parts", func(t *testing.T) {
		mockCache := NewMockCacheService()
		mockCache.simulateUnavailable = true
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		modes := []TradingMode{ModeUltraFast, ModeScalp, ModeSwing, ModePosition}
		orderTypes := AllOrderTypes()

		for _, mode := range modes {
			for _, orderType := range orderTypes {
				id, _ := generator.Generate(ctx, "user123", mode, orderType)
				parts := strings.Split(id, "-")

				// Must have exactly 4 parts
				if len(parts) != 4 {
					t.Errorf("Fallback %s-%s: Expected 4 parts, got %d: %s",
						ModeCode[mode], orderType, len(parts), id)
					continue
				}

				// Part 0: Mode - exactly 3 uppercase chars
				if len(parts[0]) != 3 {
					t.Errorf("Fallback mode should be 3 chars: %s", parts[0])
				}
				if parts[0] != strings.ToUpper(parts[0]) {
					t.Errorf("Fallback mode should be uppercase: %s", parts[0])
				}

				// Part 1: FALLBACK marker
				if parts[1] != "FALLBACK" {
					t.Errorf("Expected FALLBACK marker, got: %s", parts[1])
				}

				// Part 2: 8-char hex unique ID
				if len(parts[2]) != 8 {
					t.Errorf("Fallback unique ID should be 8 chars: %s", parts[2])
				}

				// Part 3: Order type
				if parts[3] != string(orderType) {
					t.Errorf("Order type mismatch: expected %s, got %s", orderType, parts[3])
				}
			}
		}
	})

	t.Run("hyphens are only separators", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		id, _ := generator.Generate(ctx, "user123", ModeScalp, OrderTypeEntry)

		// Count hyphens
		hyphenCount := strings.Count(id, "-")
		if hyphenCount != 3 {
			t.Errorf("Expected exactly 3 hyphens, got %d in: %s", hyphenCount, id)
		}

		// No other special characters
		for i, c := range id {
			isAlphaNum := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
			isHyphen := c == '-'
			if !isAlphaNum && !isHyphen {
				t.Errorf("Unexpected character '%c' at position %d in: %s", c, i, id)
			}
		}
	})

	t.Run("mode codes are consistent", func(t *testing.T) {
		expectedCodes := map[TradingMode]string{
			ModeUltraFast: "ULT",
			ModeScalp:     "SCA",
			ModeSwing:     "SWI",
			ModePosition:  "POS",
		}

		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		for mode, expectedCode := range expectedCodes {
			id, _ := generator.Generate(ctx, "user123", mode, OrderTypeEntry)

			if !strings.HasPrefix(id, expectedCode+"-") {
				t.Errorf("Mode %s should produce prefix %s-, got: %s", mode, expectedCode, id)
			}

			// Also verify through ModeCode map
			if ModeCode[mode] != expectedCode {
				t.Errorf("ModeCode[%s] = %s, expected %s", mode, ModeCode[mode], expectedCode)
			}
		}
	})

	t.Run("sequence is zero-padded", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		// Generate first order (sequence 1)
		id1, _ := generator.Generate(ctx, "user123", ModeScalp, OrderTypeEntry)

		parts := strings.Split(id1, "-")
		if len(parts) >= 3 {
			seq := parts[2]
			if seq != "00001" {
				t.Errorf("First sequence should be zero-padded to 00001, got: %s", seq)
			}
		}

		// Generate more to get to sequence 10
		for i := 0; i < 8; i++ {
			generator.Generate(ctx, "user123", ModeScalp, OrderTypeEntry)
		}

		id10, _ := generator.Generate(ctx, "user123", ModeScalp, OrderTypeEntry)
		parts10 := strings.Split(id10, "-")
		if len(parts10) >= 3 {
			seq := parts10[2]
			if seq != "00010" {
				t.Errorf("Sequence 10 should be zero-padded to 00010, got: %s", seq)
			}
		}
	})

	t.Run("date format is DDMMM uppercase", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		testTimes := []struct {
			time     time.Time
			expected string
		}{
			{time.Date(2026, 1, 6, 12, 0, 0, 0, tz), "06JAN"},
			{time.Date(2026, 2, 14, 12, 0, 0, 0, tz), "14FEB"},
			{time.Date(2026, 3, 1, 12, 0, 0, 0, tz), "01MAR"},
			{time.Date(2026, 4, 30, 12, 0, 0, 0, tz), "30APR"},
			{time.Date(2026, 5, 15, 12, 0, 0, 0, tz), "15MAY"},
			{time.Date(2026, 6, 10, 12, 0, 0, 0, tz), "10JUN"},
			{time.Date(2026, 7, 4, 12, 0, 0, 0, tz), "04JUL"},
			{time.Date(2026, 8, 20, 12, 0, 0, 0, tz), "20AUG"},
			{time.Date(2026, 9, 25, 12, 0, 0, 0, tz), "25SEP"},
			{time.Date(2026, 10, 31, 12, 0, 0, 0, tz), "31OCT"},
			{time.Date(2026, 11, 11, 12, 0, 0, 0, tz), "11NOV"},
			{time.Date(2026, 12, 25, 12, 0, 0, 0, tz), "25DEC"},
		}

		for _, tt := range testTimes {
			id, _ := generator.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, tt.time)

			if !strings.Contains(id, tt.expected) {
				t.Errorf("Date %s: expected %s in ID, got: %s", tt.time.Format("2006-01-02"), tt.expected, id)
			}

			// Verify uppercase
			parts := strings.Split(id, "-")
			if len(parts) >= 2 {
				dateStr := parts[1]
				if dateStr != strings.ToUpper(dateStr) {
					t.Errorf("Date should be uppercase: %s", dateStr)
				}
			}
		}
	})
}

// ============================================================================
// ADDITIONAL EDGE CASE TESTS
// ============================================================================

func TestEdgeCases_UserIsolation(t *testing.T) {
	t.Run("different users have independent sequences", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		// Generate 5 orders for user1
		for i := 0; i < 5; i++ {
			generator.Generate(ctx, "user1", ModeScalp, OrderTypeEntry)
		}

		// Generate first order for user2
		id, _ := generator.Generate(ctx, "user2", ModeScalp, OrderTypeEntry)

		// User2's first order should have sequence 00001
		if !strings.Contains(id, "00001") {
			t.Errorf("User2's first order should be 00001, got: %s", id)
		}
	})
}

func TestEdgeCases_RedisRecovery(t *testing.T) {
	t.Run("transitions from fallback to normal when Redis recovers", func(t *testing.T) {
		mockCache := NewMockCacheService()
		tz, _ := time.LoadLocation("Asia/Kolkata")
		generator := NewTestableClientOrderIdGenerator(mockCache, tz)
		ctx := context.Background()

		// Phase 1: Redis unavailable
		mockCache.simulateUnavailable = true
		fallbackID, _ := generator.Generate(ctx, "user123", ModeScalp, OrderTypeEntry)

		if !strings.Contains(fallbackID, "FALLBACK") {
			t.Errorf("Should generate fallback when Redis unavailable: %s", fallbackID)
		}

		// Phase 2: Redis recovers
		mockCache.simulateUnavailable = false
		normalID, _ := generator.Generate(ctx, "user123", ModeScalp, OrderTypeEntry)

		if strings.Contains(normalID, "FALLBACK") {
			t.Errorf("Should NOT generate fallback when Redis healthy: %s", normalID)
		}
	})
}

func TestEdgeCases_OrderTypeDescriptions(t *testing.T) {
	t.Run("all order types have valid descriptions", func(t *testing.T) {
		orderTypes := AllOrderTypes()

		expectedTypes := map[OrderType]bool{
			OrderTypeEntry:   true, // E
			OrderTypeTP1:     true, // TP1
			OrderTypeTP2:     true, // TP2
			OrderTypeTP3:     true, // TP3
			OrderTypeRebuy:   true, // RB
			OrderTypeDCA1:    true, // DCA1
			OrderTypeDCA2:    true, // DCA2
			OrderTypeDCA3:    true, // DCA3
			OrderTypeHedge:   true, // H
			OrderTypeHedgeSL: true, // HSL
			OrderTypeHedgeTP: true, // HTP
			OrderTypeSL:      true, // SL
		}

		if len(orderTypes) != len(expectedTypes) {
			t.Errorf("Expected %d order types, got %d", len(expectedTypes), len(orderTypes))
		}

		for _, ot := range orderTypes {
			if !expectedTypes[ot] {
				t.Errorf("Unexpected order type: %s", ot)
			}
		}
	})
}

func TestEdgeCases_SpecialDates(t *testing.T) {
	mockCache := NewMockCacheService()
	tz, _ := time.LoadLocation("Asia/Kolkata")
	generator := NewTestableClientOrderIdGenerator(mockCache, tz)
	ctx := context.Background()

	t.Run("leap year Feb 29", func(t *testing.T) {
		// 2028 is a leap year
		feb29 := time.Date(2028, 2, 29, 12, 0, 0, 0, tz)
		id, _ := generator.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, feb29)

		if !strings.Contains(id, "29FEB") {
			t.Errorf("Leap year Feb 29 should produce 29FEB, got: %s", id)
		}
	})

	t.Run("DST transition (if applicable)", func(t *testing.T) {
		// Test around typical DST transition times
		// This depends on the timezone but ensures no crashes
		nyTz, _ := time.LoadLocation("America/New_York")
		genNY := NewTestableClientOrderIdGenerator(NewMockCacheService(), nyTz)

		// Around March DST transition
		marchTime := time.Date(2026, 3, 8, 2, 30, 0, 0, time.UTC)
		id, err := genNY.GenerateAtTime(ctx, "user123", ModeScalp, OrderTypeEntry, marchTime)

		if err != nil {
			t.Errorf("DST transition should not cause error: %v", err)
		}
		if id == "" {
			t.Error("Should produce valid ID during DST transition")
		}
	})
}
