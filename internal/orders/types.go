// Package orders provides client order ID generation for Binance futures trading.
// Epic 7: Client Order ID & Trade Lifecycle Tracking
package orders

// TradingMode represents the 4 trading strategy modes
type TradingMode string

const (
	ModeUltraFast TradingMode = "ultra_fast"
	ModeScalp     TradingMode = "scalp"
	ModeSwing     TradingMode = "swing"
	ModePosition  TradingMode = "position"
)

// ModeCode maps TradingMode to 3-character codes for clientOrderId
var ModeCode = map[TradingMode]string{
	ModeUltraFast: "ULT",
	ModeScalp:     "SCA",
	ModeSwing:     "SWI",
	ModePosition:  "POS",
}

// ModeFromString converts a string mode name to TradingMode
func ModeFromString(mode string) TradingMode {
	switch mode {
	case "ultra_fast":
		return ModeUltraFast
	case "scalp":
		return ModeScalp
	case "swing":
		return ModeSwing
	case "position":
		return ModePosition
	default:
		return ModeScalp // Default fallback
	}
}

// OrderType represents the order purpose in a position lifecycle
type OrderType string

const (
	// Entry orders
	OrderTypeEntry OrderType = "E" // Initial entry order

	// Take Profit orders (from position optimization)
	OrderTypeTP1 OrderType = "TP1" // Take Profit 1 (30% at 0.4%)
	OrderTypeTP2 OrderType = "TP2" // Take Profit 2 (50% at 0.7%)
	OrderTypeTP3 OrderType = "TP3" // Take Profit 3 (80% at 1.0%)

	// Rebuy order (after TP hits)
	OrderTypeRebuy OrderType = "RB" // Rebuy/Re-entry order

	// DCA orders (from NEG-TP levels on loss)
	OrderTypeDCA1 OrderType = "DCA1" // DCA at NEG-TP1 (-0.4%)
	OrderTypeDCA2 OrderType = "DCA2" // DCA at NEG-TP2 (-0.7%)
	OrderTypeDCA3 OrderType = "DCA3" // DCA at NEG-TP3 (-1.0%)

	// Hedge order (optional feature)
	OrderTypeHedge   OrderType = "H"    // Hedge entry (opposite side)
	OrderTypeHedgeSL OrderType = "HSL"  // Hedge Stop Loss order
	OrderTypeHedgeTP OrderType = "HTP"  // Hedge Take Profit order

	// Stop Loss order
	OrderTypeSL OrderType = "SL" // Stop Loss order
)

// AllOrderTypes returns all valid order types
func AllOrderTypes() []OrderType {
	return []OrderType{
		OrderTypeEntry,
		OrderTypeTP1, OrderTypeTP2, OrderTypeTP3,
		OrderTypeRebuy,
		OrderTypeDCA1, OrderTypeDCA2, OrderTypeDCA3,
		OrderTypeHedge, OrderTypeHedgeSL, OrderTypeHedgeTP,
		OrderTypeSL,
	}
}
