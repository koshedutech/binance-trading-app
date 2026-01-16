// Package orders provides client order ID generation for Binance futures trading.
// Epic 7: Client Order ID & Trade Lifecycle Tracking
package orders

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"
)

// fallbackCounter ensures unique fallback IDs even if crypto/rand fails
// and multiple goroutines call at the same nanosecond
var fallbackCounter uint64

const (
	// MaxClientOrderIDLength is the maximum length allowed by Binance
	MaxClientOrderIDLength = 36

	// FallbackMarker identifies fallback IDs generated when Redis is unavailable
	FallbackMarker = "FALLBACK"

	// Default timezone for date formatting
	DefaultTimezone = "Asia/Kolkata"
)

// Errors for client order ID operations
var (
	ErrClientOrderIDTooLong = errors.New("client order ID exceeds maximum length of 36 characters")
	ErrInvalidClientOrderID = errors.New("invalid client order ID format")
	ErrInvalidMode          = errors.New("invalid trading mode")
	ErrInvalidOrderType     = errors.New("invalid order type")
	ErrEmptyUserID          = errors.New("user ID cannot be empty")
)

// SequenceProvider provides atomic daily sequence numbers for clientOrderId generation.
// This interface breaks the import cycle: orders package doesn't need to import cache.
// cache.CacheService implements this interface.
type SequenceProvider interface {
	// IncrementDailySequence atomically increments and returns the daily sequence for a user.
	// dateKey is in YYYYMMDD format (e.g., "20260115")
	IncrementDailySequence(ctx context.Context, userID, dateKey string) (int64, error)
	// IsHealthy returns whether the sequence provider is available
	IsHealthy() bool
}

// ClientOrderIdGenerator generates structured client order IDs for Binance futures.
// Format: [MODE]-[DDMMM]-[NNNNN]-[TYPE] (e.g., "SCA-15JAN-00001-E")
// Fallback format: [MODE]-FALLBACK-[8CHAR]-[TYPE] (e.g., "SCA-FALLBACK-a3f7c2e9-E")
type ClientOrderIdGenerator struct {
	sequenceProvider SequenceProvider
	userID           string
	timezone         *time.Location
}

// NewClientOrderIdGenerator creates a new ClientOrderIdGenerator.
// sequenceProvider can be nil (will always use fallback IDs).
// If timezone is nil, defaults to Asia/Kolkata.
func NewClientOrderIdGenerator(sequenceProvider SequenceProvider, userID string, timezone *time.Location) (*ClientOrderIdGenerator, error) {
	if userID == "" {
		return nil, ErrEmptyUserID
	}

	if timezone == nil {
		var err error
		timezone, err = time.LoadLocation(DefaultTimezone)
		if err != nil {
			// Fallback to UTC if timezone loading fails
			log.Printf("[ClientOrderIdGenerator] Failed to load timezone %s, using UTC: %v", DefaultTimezone, err)
			timezone = time.UTC
		}
	}

	return &ClientOrderIdGenerator{
		sequenceProvider: sequenceProvider,
		userID:           userID,
		timezone:         timezone,
	}, nil
}

// Generate creates a new client order ID with an atomic sequence number.
// Returns (fullID, baseID, error) where baseID is the ID without the order type suffix.
// If Redis is unavailable, automatically uses fallback ID generation.
//
// Example:
//   - fullID: "SCA-15JAN-00001-E"
//   - baseID: "SCA-15JAN-00001"
func (g *ClientOrderIdGenerator) Generate(ctx context.Context, mode TradingMode, orderType OrderType) (string, string, error) {
	// Validate inputs
	if err := validateMode(mode); err != nil {
		return "", "", err
	}
	if err := validateOrderType(orderType); err != nil {
		return "", "", err
	}

	// Get current time in user's timezone
	now := time.Now().In(g.timezone)
	dateStr := strings.ToUpper(now.Format("02Jan")) // "15JAN"

	// Try to get sequence from sequence provider (Redis)
	if g.sequenceProvider != nil && g.sequenceProvider.IsHealthy() {
		dateKey := now.Format("20060102") // "20260115" for Redis key
		seq, err := g.sequenceProvider.IncrementDailySequence(ctx, g.userID, dateKey)
		if err == nil {
			// Check for sequence overflow (max 99999 for 5-digit format)
			if seq > 99999 {
				log.Printf("[ClientOrderIdGenerator] Sequence overflow (%d > 99999), using fallback ID", seq)
				fullID, baseID := g.GenerateFallback(mode, orderType)
				return fullID, baseID, nil
			}

			// Successfully got sequence from Redis
			modeCode := getModeCode(mode)
			baseID := fmt.Sprintf("%s-%s-%05d", modeCode, dateStr, seq)
			fullID := fmt.Sprintf("%s-%s", baseID, orderType)

			// Validate length (should never trigger with valid seq, but safety check)
			if len(fullID) > MaxClientOrderIDLength {
				log.Printf("[ClientOrderIdGenerator] Generated ID too long, using fallback")
				fallbackFullID, fallbackBaseID := g.GenerateFallback(mode, orderType)
				return fallbackFullID, fallbackBaseID, nil
			}

			return fullID, baseID, nil
		}

		// Sequence provider failed, log and use fallback
		log.Printf("[ClientOrderIdGenerator] Sequence provider error, using fallback: %v", err)
	} else if g.sequenceProvider == nil {
		log.Printf("[ClientOrderIdGenerator] SequenceProvider is nil, using fallback ID generation")
	} else {
		log.Printf("[ClientOrderIdGenerator] SequenceProvider unhealthy, using fallback ID generation")
	}

	// Use fallback when Redis is unavailable
	fullID, baseID := g.GenerateFallback(mode, orderType)
	return fullID, baseID, nil
}

// GenerateRelated creates a related order ID using the same base ID.
// Use this for SL, TP, DCA orders that belong to the same order chain.
//
// Example:
//   - baseID: "SCA-15JAN-00001"
//   - orderType: OrderTypeTP1
//   - result: "SCA-15JAN-00001-TP1"
func (g *ClientOrderIdGenerator) GenerateRelated(baseID string, orderType OrderType) (string, error) {
	if baseID == "" {
		return "", errors.New("baseID cannot be empty")
	}
	if err := validateOrderType(orderType); err != nil {
		return "", err
	}

	fullID := fmt.Sprintf("%s-%s", baseID, orderType)

	// Validate length
	if len(fullID) > MaxClientOrderIDLength {
		return "", fmt.Errorf("%w: generated ID '%s' is %d characters", ErrClientOrderIDTooLong, fullID, len(fullID))
	}

	return fullID, nil
}

// GenerateFallback creates a fallback client order ID when Redis is unavailable.
// Uses a timestamp-based unique identifier to ensure uniqueness.
// Format: [MODE]-FALLBACK-[8CHAR]-[TYPE] (e.g., "SCA-FALLBACK-a3f7c2e9-E")
// Returns (fullID, baseID)
func (g *ClientOrderIdGenerator) GenerateFallback(mode TradingMode, orderType OrderType) (string, string) {
	modeCode := getModeCode(mode)
	uniqueID := generateShortUniqueID()
	baseID := fmt.Sprintf("%s-%s-%s", modeCode, FallbackMarker, uniqueID)
	fullID := fmt.Sprintf("%s-%s", baseID, orderType)

	return fullID, baseID
}

// ValidateClientOrderID validates that a client order ID meets Binance requirements.
// Returns nil if valid, error otherwise.
func ValidateClientOrderID(id string) error {
	if id == "" {
		return ErrInvalidClientOrderID
	}

	if len(id) > MaxClientOrderIDLength {
		return fmt.Errorf("%w: ID '%s' is %d characters (max %d)", ErrClientOrderIDTooLong, id, len(id), MaxClientOrderIDLength)
	}

	// Basic format validation: should have at least MODE-xxx-TYPE structure
	parts := strings.Split(id, "-")
	if len(parts) < 3 {
		return fmt.Errorf("%w: expected at least 3 parts separated by '-'", ErrInvalidClientOrderID)
	}

	// Validate mode code (first part should be 3 chars)
	modeCode := parts[0]
	if len(modeCode) != 3 {
		return fmt.Errorf("%w: mode code '%s' should be 3 characters", ErrInvalidClientOrderID, modeCode)
	}

	// Validate it's a known mode code
	validModeCodes := map[string]bool{
		"ULT": true, // ultra_fast
		"SCA": true, // scalp
		"SWI": true, // swing
		"POS": true, // position
	}
	if !validModeCodes[modeCode] {
		return fmt.Errorf("%w: unknown mode code '%s'", ErrInvalidClientOrderID, modeCode)
	}

	return nil
}

// IsFallbackID checks if the client order ID is a fallback ID (generated when Redis was unavailable)
func IsFallbackID(id string) bool {
	return strings.Contains(id, "-"+FallbackMarker+"-")
}

// ExtractBaseID extracts the base ID from a full client order ID.
// For "SCA-15JAN-00001-TP1" returns "SCA-15JAN-00001"
// For "SCA-FALLBACK-a3f7c2e9-E" returns "SCA-FALLBACK-a3f7c2e9"
func ExtractBaseID(fullID string) (string, error) {
	if fullID == "" {
		return "", ErrInvalidClientOrderID
	}

	parts := strings.Split(fullID, "-")
	if len(parts) < 3 {
		return "", fmt.Errorf("%w: cannot extract base ID from '%s'", ErrInvalidClientOrderID, fullID)
	}

	// For fallback IDs: MODE-FALLBACK-UUID-TYPE -> MODE-FALLBACK-UUID
	if len(parts) >= 4 && parts[1] == FallbackMarker {
		return strings.Join(parts[:3], "-"), nil
	}

	// For normal IDs: MODE-DDMMM-NNNNN-TYPE -> MODE-DDMMM-NNNNN
	if len(parts) >= 4 {
		return strings.Join(parts[:3], "-"), nil
	}

	// Edge case: ID might already be a base ID
	return fullID, nil
}

// getModeCode returns the 3-character code for a TradingMode
func getModeCode(mode TradingMode) string {
	if code, exists := ModeCode[mode]; exists {
		return code
	}
	// Default fallback
	return "SCA"
}

// validateMode checks if the trading mode is valid
func validateMode(mode TradingMode) error {
	switch mode {
	case ModeUltraFast, ModeScalp, ModeSwing, ModePosition:
		return nil
	default:
		return fmt.Errorf("%w: '%s'", ErrInvalidMode, mode)
	}
}

// validateOrderType checks if the order type is valid
func validateOrderType(orderType OrderType) error {
	switch orderType {
	case OrderTypeEntry, OrderTypeTP1, OrderTypeTP2, OrderTypeTP3,
		OrderTypeRebuy, OrderTypeDCA1, OrderTypeDCA2, OrderTypeDCA3,
		OrderTypeHedge, OrderTypeHedgeSL, OrderTypeHedgeTP, OrderTypeSL:
		return nil
	default:
		return fmt.Errorf("%w: '%s'", ErrInvalidOrderType, orderType)
	}
}

// generateShortUniqueID generates an 8-character hex unique identifier
// Uses crypto/rand for better uniqueness guarantees
func generateShortUniqueID() string {
	b := make([]byte, 4) // 4 bytes = 8 hex characters
	_, err := rand.Read(b)
	if err != nil {
		// Fallback to timestamp + atomic counter if crypto/rand fails
		// This ensures uniqueness even if called at the same nanosecond
		counter := atomic.AddUint64(&fallbackCounter, 1)
		combined := (uint64(time.Now().UnixNano()) << 16) | (counter & 0xFFFF)
		return fmt.Sprintf("%08x", combined&0xFFFFFFFF)
	}
	return hex.EncodeToString(b)
}
