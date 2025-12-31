package autopilot

import (
	"context"
	"log"
	"time"

	"binance-trading-bot/internal/database"
)

// TradeEventLogger handles logging of trade lifecycle events to the database
type TradeEventLogger struct {
	db     *database.DB
	logger *log.Logger
}

// NewTradeEventLogger creates a new event logger instance
func NewTradeEventLogger(db *database.DB) *TradeEventLogger {
	return &TradeEventLogger{
		db:     db,
		logger: log.New(log.Writer(), "[EVENT-LOGGER] ", log.LstdFlags),
	}
}

// LogPositionOpened logs when a new position is opened
func (el *TradeEventLogger) LogPositionOpened(
	ctx context.Context,
	futuresTradeID int64,
	symbol string,
	side string,
	mode string,
	entryPrice float64,
	quantity float64,
	leverage int,
	confidence float64,
	conditions map[string]interface{},
) error {
	if el.db == nil {
		return nil
	}

	modePtr := &mode
	event := &database.TradeLifecycleEvent{
		FuturesTradeID: &futuresTradeID,
		EventType:      database.EventTypePositionOpened,
		Timestamp:      time.Now(),
		TriggerPrice:   &entryPrice,
		Mode:           modePtr,
		Source:         database.EventSourceGinie,
		ConditionsMet:  conditions,
		Details: map[string]interface{}{
			"symbol":     symbol,
			"side":       side,
			"quantity":   quantity,
			"leverage":   leverage,
			"confidence": confidence,
		},
	}

	if err := el.db.CreateTradeLifecycleEvent(ctx, event); err != nil {
		el.logger.Printf("Failed to log position opened event: %v", err)
		return err
	}

	el.logger.Printf("Logged position opened: %s %s at %.8f", symbol, side, entryPrice)
	return nil
}

// LogSLTPPlaced logs when initial SL/TP orders are placed
func (el *TradeEventLogger) LogSLTPPlaced(
	ctx context.Context,
	futuresTradeID int64,
	symbol string,
	mode string,
	slPrice float64,
	tpLevels []float64,
) error {
	if el.db == nil {
		return nil
	}

	modePtr := &mode
	event := &database.TradeLifecycleEvent{
		FuturesTradeID: &futuresTradeID,
		EventType:      database.EventTypeSLTPPlaced,
		Timestamp:      time.Now(),
		NewValue:       &slPrice,
		Mode:           modePtr,
		Source:         database.EventSourceGinie,
		Details: map[string]interface{}{
			"symbol":    symbol,
			"sl_price":  slPrice,
			"tp_levels": tpLevels,
		},
	}

	if err := el.db.CreateTradeLifecycleEvent(ctx, event); err != nil {
		el.logger.Printf("Failed to log SLTP placed event: %v", err)
		return err
	}

	el.logger.Printf("Logged SLTP placed: %s SL=%.8f", symbol, slPrice)
	return nil
}

// LogSLRevised logs when SL is revised/updated
func (el *TradeEventLogger) LogSLRevised(
	ctx context.Context,
	futuresTradeID int64,
	symbol string,
	oldSL float64,
	newSL float64,
	reason string,
	revisionCount int,
) error {
	if el.db == nil {
		return nil
	}

	reasonPtr := &reason
	revCountPtr := &revisionCount
	event := &database.TradeLifecycleEvent{
		FuturesTradeID:  &futuresTradeID,
		EventType:       database.EventTypeSLRevised,
		Timestamp:       time.Now(),
		OldValue:        &oldSL,
		NewValue:        &newSL,
		Source:          database.EventSourceGinie,
		SLRevisionCount: revCountPtr,
		Reason:          reasonPtr,
		Details: map[string]interface{}{
			"symbol":          symbol,
			"improvement_pct": ((newSL - oldSL) / oldSL) * 100,
		},
	}

	if err := el.db.CreateTradeLifecycleEvent(ctx, event); err != nil {
		el.logger.Printf("Failed to log SL revised event: %v", err)
		return err
	}

	el.logger.Printf("Logged SL revised: %s %.8f -> %.8f (%s)", symbol, oldSL, newSL, reason)
	return nil
}

// LogMovedToBreakeven logs when SL is moved to breakeven
func (el *TradeEventLogger) LogMovedToBreakeven(
	ctx context.Context,
	futuresTradeID int64,
	symbol string,
	entryPrice float64,
	newSL float64,
	buffer float64,
	breakevenReason string, // "tp1_hit" or "proactive" or custom reason
) error {
	if el.db == nil {
		return nil
	}

	subtype := "moved_to_breakeven"
	// Use provided reason or default
	reason := breakevenReason
	if reason == "" {
		reason = "Moved SL to breakeven"
	}
	event := &database.TradeLifecycleEvent{
		FuturesTradeID: &futuresTradeID,
		EventType:      database.EventTypeMovedToBreakeven,
		EventSubtype:   &subtype,
		Timestamp:      time.Now(),
		OldValue:       &entryPrice, // Previous SL was likely below entry
		NewValue:       &newSL,
		Source:         database.EventSourceGinie,
		Reason:         &reason,
		Details: map[string]interface{}{
			"symbol":      symbol,
			"entry_price": entryPrice,
			"buffer":      buffer,
		},
	}

	if err := el.db.CreateTradeLifecycleEvent(ctx, event); err != nil {
		el.logger.Printf("Failed to log moved to breakeven event: %v", err)
		return err
	}

	el.logger.Printf("Logged moved to breakeven: %s entry=%.8f new_sl=%.8f", symbol, entryPrice, newSL)
	return nil
}

// LogTPHit logs when a take profit level is hit
func (el *TradeEventLogger) LogTPHit(
	ctx context.Context,
	futuresTradeID int64,
	symbol string,
	tpLevel int,
	triggerPrice float64,
	quantityClosed float64,
	pnl float64,
	pnlPercent float64,
) error {
	if el.db == nil {
		return nil
	}

	subtype := "tp" + string(rune('0'+tpLevel)) + "_hit"
	event := &database.TradeLifecycleEvent{
		FuturesTradeID: &futuresTradeID,
		EventType:      database.EventTypeTPHit,
		EventSubtype:   &subtype,
		Timestamp:      time.Now(),
		TriggerPrice:   &triggerPrice,
		Source:         database.EventSourceGinie,
		TPLevel:        &tpLevel,
		QuantityClosed: &quantityClosed,
		PnLRealized:    &pnl,
		PnLPercent:     &pnlPercent,
		Details: map[string]interface{}{
			"symbol": symbol,
		},
	}

	if err := el.db.CreateTradeLifecycleEvent(ctx, event); err != nil {
		el.logger.Printf("Failed to log TP hit event: %v", err)
		return err
	}

	el.logger.Printf("Logged TP%d hit: %s at %.8f, PnL=%.2f (%.2f%%)", tpLevel, symbol, triggerPrice, pnl, pnlPercent)
	return nil
}

// LogTrailingActivated logs when trailing stop is activated
func (el *TradeEventLogger) LogTrailingActivated(
	ctx context.Context,
	futuresTradeID int64,
	symbol string,
	mode string,
	activationReason string,
	activationPrice float64,
	profitPercent float64,
	tpLevel int,
) error {
	if el.db == nil {
		return nil
	}

	modePtr := &mode
	reasonPtr := &activationReason
	event := &database.TradeLifecycleEvent{
		FuturesTradeID: &futuresTradeID,
		EventType:      database.EventTypeTrailingActivated,
		Timestamp:      time.Now(),
		TriggerPrice:   &activationPrice,
		Mode:           modePtr,
		Source:         database.EventSourceTrailing,
		TPLevel:        &tpLevel,
		PnLPercent:     &profitPercent,
		Reason:         reasonPtr,
		Details: map[string]interface{}{
			"symbol": symbol,
		},
	}

	if err := el.db.CreateTradeLifecycleEvent(ctx, event); err != nil {
		el.logger.Printf("Failed to log trailing activated event: %v", err)
		return err
	}

	el.logger.Printf("Logged trailing activated: %s at %.8f (%.2f%% profit, reason: %s)", symbol, activationPrice, profitPercent, activationReason)
	return nil
}

// LogTrailingUpdated logs when trailing SL is updated (moved up/down)
func (el *TradeEventLogger) LogTrailingUpdated(
	ctx context.Context,
	futuresTradeID int64,
	symbol string,
	side string,
	oldSL float64,
	newSL float64,
	highWaterMark float64,
	improvementPct float64,
) error {
	if el.db == nil {
		return nil
	}

	reason := "Trailing SL moved " + side
	event := &database.TradeLifecycleEvent{
		FuturesTradeID: &futuresTradeID,
		EventType:      database.EventTypeTrailingUpdated,
		Timestamp:      time.Now(),
		OldValue:       &oldSL,
		NewValue:       &newSL,
		Source:         database.EventSourceTrailing,
		Reason:         &reason,
		Details: map[string]interface{}{
			"symbol":          symbol,
			"side":            side,
			"high_water_mark": highWaterMark,
			"improvement_pct": improvementPct,
		},
	}

	if err := el.db.CreateTradeLifecycleEvent(ctx, event); err != nil {
		el.logger.Printf("Failed to log trailing updated event: %v", err)
		return err
	}

	el.logger.Printf("Logged trailing updated: %s SL %.8f -> %.8f (%.2f%% improvement)", symbol, oldSL, newSL, improvementPct)
	return nil
}

// LogPositionClosed logs when a position is fully closed
func (el *TradeEventLogger) LogPositionClosed(
	ctx context.Context,
	futuresTradeID int64,
	symbol string,
	closePrice float64,
	quantity float64,
	totalPnL float64,
	pnlPercent float64,
	reason string,
	source string,
) error {
	if el.db == nil {
		return nil
	}

	reasonPtr := &reason
	event := &database.TradeLifecycleEvent{
		FuturesTradeID: &futuresTradeID,
		EventType:      database.EventTypePositionClosed,
		Timestamp:      time.Now(),
		TriggerPrice:   &closePrice,
		Source:         source,
		QuantityClosed: &quantity,
		PnLRealized:    &totalPnL,
		PnLPercent:     &pnlPercent,
		Reason:         reasonPtr,
		Details: map[string]interface{}{
			"symbol": symbol,
		},
	}

	if err := el.db.CreateTradeLifecycleEvent(ctx, event); err != nil {
		el.logger.Printf("Failed to log position closed event: %v", err)
		return err
	}

	el.logger.Printf("Logged position closed: %s at %.8f, PnL=%.2f (%.2f%%), reason: %s, source: %s",
		symbol, closePrice, totalPnL, pnlPercent, reason, source)
	return nil
}

// LogExternalClose logs when a position is closed externally (not by Ginie)
func (el *TradeEventLogger) LogExternalClose(
	ctx context.Context,
	futuresTradeID int64,
	symbol string,
	closePrice float64,
	quantity float64,
	pnl float64,
	pnlPercent float64,
) error {
	if el.db == nil {
		return nil
	}

	reason := "Position closed externally (manual or Binance)"
	event := &database.TradeLifecycleEvent{
		FuturesTradeID: &futuresTradeID,
		EventType:      database.EventTypeExternalClose,
		Timestamp:      time.Now(),
		TriggerPrice:   &closePrice,
		Source:         database.EventSourceExternal,
		QuantityClosed: &quantity,
		PnLRealized:    &pnl,
		PnLPercent:     &pnlPercent,
		Reason:         &reason,
		Details: map[string]interface{}{
			"symbol": symbol,
		},
	}

	if err := el.db.CreateTradeLifecycleEvent(ctx, event); err != nil {
		el.logger.Printf("Failed to log external close event: %v", err)
		return err
	}

	el.logger.Printf("Logged external close: %s at %.8f, PnL=%.2f (%.2f%%)", symbol, closePrice, pnl, pnlPercent)
	return nil
}

// LogSLHit logs when a stop loss is hit
func (el *TradeEventLogger) LogSLHit(
	ctx context.Context,
	futuresTradeID int64,
	symbol string,
	slPrice float64,
	quantity float64,
	pnl float64,
	pnlPercent float64,
) error {
	if el.db == nil {
		return nil
	}

	reason := "Stop loss triggered"
	event := &database.TradeLifecycleEvent{
		FuturesTradeID: &futuresTradeID,
		EventType:      database.EventTypeSLHit,
		Timestamp:      time.Now(),
		TriggerPrice:   &slPrice,
		Source:         database.EventSourceGinie,
		QuantityClosed: &quantity,
		PnLRealized:    &pnl,
		PnLPercent:     &pnlPercent,
		Reason:         &reason,
		Details: map[string]interface{}{
			"symbol": symbol,
		},
	}

	if err := el.db.CreateTradeLifecycleEvent(ctx, event); err != nil {
		el.logger.Printf("Failed to log SL hit event: %v", err)
		return err
	}

	el.logger.Printf("Logged SL hit: %s at %.8f, PnL=%.2f (%.2f%%)", symbol, slPrice, pnl, pnlPercent)
	return nil
}
