// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.20: Actor Tracking System Tests
package decision

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// ========== Actor Type Tests ==========

func TestActorString(t *testing.T) {
	tests := []struct {
		actor    Actor
		expected string
	}{
		{ActorUser, "USER"},
		{ActorGinieAuto, "GINIE_AUTO"},
		{ActorGinieSuggested, "GINIE_SUGGESTED"},
		{ActorSystem, "SYSTEM"},
		{ActorUnknown, "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(string(tt.actor), func(t *testing.T) {
			if got := tt.actor.String(); got != tt.expected {
				t.Errorf("Actor.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestActorDisplayName(t *testing.T) {
	tests := []struct {
		actor    Actor
		expected string
	}{
		{ActorUser, "Manual"},
		{ActorGinieAuto, "Auto"},
		{ActorGinieSuggested, "Suggested"},
		{ActorSystem, "System"},
		{ActorUnknown, "Unknown"},
		{Actor("INVALID"), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.actor), func(t *testing.T) {
			if got := tt.actor.DisplayName(); got != tt.expected {
				t.Errorf("Actor.DisplayName() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestActorBadge(t *testing.T) {
	tests := []struct {
		actor    Actor
		expected string
	}{
		{ActorUser, "U"},
		{ActorGinieAuto, "A"},
		{ActorGinieSuggested, "S"},
		{ActorSystem, "X"},
		{ActorUnknown, "?"},
		{Actor("INVALID"), "?"},
	}

	for _, tt := range tests {
		t.Run(string(tt.actor), func(t *testing.T) {
			if got := tt.actor.Badge(); got != tt.expected {
				t.Errorf("Actor.Badge() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseActor(t *testing.T) {
	tests := []struct {
		input    string
		expected Actor
	}{
		{"USER", ActorUser},
		{"GINIE_AUTO", ActorGinieAuto},
		{"GINIE_SUGGESTED", ActorGinieSuggested},
		{"SYSTEM", ActorSystem},
		{"UNKNOWN", ActorUnknown},
		{"INVALID", ActorUnknown},
		{"", ActorUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseActor(tt.input); got != tt.expected {
				t.Errorf("ParseActor(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsValidActor(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"USER", true},
		{"GINIE_AUTO", true},
		{"GINIE_SUGGESTED", true},
		{"SYSTEM", true},
		{"UNKNOWN", true},
		{"INVALID", false},
		{"", false},
		{"user", false}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsValidActor(tt.input); got != tt.expected {
				t.Errorf("IsValidActor(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestAllActors(t *testing.T) {
	actors := AllActors()
	if len(actors) != 5 {
		t.Errorf("AllActors() returned %d actors, want 5", len(actors))
	}

	expected := map[Actor]bool{
		ActorUser:           true,
		ActorGinieAuto:      true,
		ActorGinieSuggested: true,
		ActorSystem:         true,
		ActorUnknown:        true,
	}

	for _, a := range actors {
		if !expected[a] {
			t.Errorf("Unexpected actor in AllActors(): %v", a)
		}
	}
}

// ========== ActorInfo Tests ==========

func TestNewActorInfo(t *testing.T) {
	info := NewActorInfo(ActorUser, "Test description")

	if info.Actor != ActorUser {
		t.Errorf("NewActorInfo actor = %v, want %v", info.Actor, ActorUser)
	}
	if info.Description != "Test description" {
		t.Errorf("NewActorInfo description = %v, want %v", info.Description, "Test description")
	}
	if info.InitiatedAt.IsZero() {
		t.Error("NewActorInfo InitiatedAt should not be zero")
	}
}

func TestNewUserActorInfo(t *testing.T) {
	info := NewUserActorInfo("192.168.1.1")

	if info.Actor != ActorUser {
		t.Errorf("NewUserActorInfo actor = %v, want %v", info.Actor, ActorUser)
	}
	if info.SourceIP != "192.168.1.1" {
		t.Errorf("NewUserActorInfo SourceIP = %v, want %v", info.SourceIP, "192.168.1.1")
	}
}

func TestNewAutoActorInfo(t *testing.T) {
	info := NewAutoActorInfo("trend_following")

	if info.Actor != ActorGinieAuto {
		t.Errorf("NewAutoActorInfo actor = %v, want %v", info.Actor, ActorGinieAuto)
	}
	if info.AutoRule != "trend_following" {
		t.Errorf("NewAutoActorInfo AutoRule = %v, want %v", info.AutoRule, "trend_following")
	}
}

func TestNewSuggestedActorInfo(t *testing.T) {
	info := NewSuggestedActorInfo("sugg-123")

	if info.Actor != ActorGinieSuggested {
		t.Errorf("NewSuggestedActorInfo actor = %v, want %v", info.Actor, ActorGinieSuggested)
	}
	if info.SuggestionID != "sugg-123" {
		t.Errorf("NewSuggestedActorInfo SuggestionID = %v, want %v", info.SuggestionID, "sugg-123")
	}
}

func TestNewSystemActorInfo(t *testing.T) {
	info := NewSystemActorInfo("Stop loss triggered")

	if info.Actor != ActorSystem {
		t.Errorf("NewSystemActorInfo actor = %v, want %v", info.Actor, ActorSystem)
	}
	if info.Description != "Stop loss triggered" {
		t.Errorf("NewSystemActorInfo Description = %v, want %v", info.Description, "Stop loss triggered")
	}
}

func TestActorInfoCopy(t *testing.T) {
	original := &ActorInfo{
		Actor:        ActorUser,
		Description:  "Test",
		InitiatedAt:  time.Now(),
		SourceIP:     "192.168.1.1",
		AutoRule:     "rule1",
		SuggestionID: "sugg-1",
	}

	copied := original.Copy()

	if copied == original {
		t.Error("Copy should return a different pointer")
	}
	if copied.Actor != original.Actor {
		t.Errorf("Copy Actor = %v, want %v", copied.Actor, original.Actor)
	}
	if copied.Description != original.Description {
		t.Errorf("Copy Description = %v, want %v", copied.Description, original.Description)
	}
	if copied.SourceIP != original.SourceIP {
		t.Errorf("Copy SourceIP = %v, want %v", copied.SourceIP, original.SourceIP)
	}

	// Test nil copy
	var nilInfo *ActorInfo
	if nilInfo.Copy() != nil {
		t.Error("Copy of nil should return nil")
	}
}

// ========== ActorMetrics Tests ==========

func TestNewActorMetrics(t *testing.T) {
	metrics := NewActorMetrics(ActorUser)

	if metrics.Actor != ActorUser {
		t.Errorf("NewActorMetrics actor = %v, want %v", metrics.Actor, ActorUser)
	}
	if metrics.TotalTrades != 0 {
		t.Errorf("NewActorMetrics TotalTrades = %v, want 0", metrics.TotalTrades)
	}
}

func TestActorMetricsUpdate(t *testing.T) {
	metrics := NewActorMetrics(ActorUser)
	now := time.Now()

	// First trade - win
	metrics.Update(100.0, true, now)
	if metrics.TotalTrades != 1 {
		t.Errorf("TotalTrades = %v, want 1", metrics.TotalTrades)
	}
	if metrics.WinningTrades != 1 {
		t.Errorf("WinningTrades = %v, want 1", metrics.WinningTrades)
	}
	if metrics.WinRate != 100.0 {
		t.Errorf("WinRate = %v, want 100.0", metrics.WinRate)
	}
	if metrics.TotalPnL != 100.0 {
		t.Errorf("TotalPnL = %v, want 100.0", metrics.TotalPnL)
	}
	if metrics.BestTrade != 100.0 {
		t.Errorf("BestTrade = %v, want 100.0", metrics.BestTrade)
	}

	// Second trade - loss
	metrics.Update(-50.0, false, now.Add(time.Minute))
	if metrics.TotalTrades != 2 {
		t.Errorf("TotalTrades = %v, want 2", metrics.TotalTrades)
	}
	if metrics.LosingTrades != 1 {
		t.Errorf("LosingTrades = %v, want 1", metrics.LosingTrades)
	}
	if metrics.WinRate != 50.0 {
		t.Errorf("WinRate = %v, want 50.0", metrics.WinRate)
	}
	if metrics.TotalPnL != 50.0 {
		t.Errorf("TotalPnL = %v, want 50.0", metrics.TotalPnL)
	}
	if metrics.WorstTrade != -50.0 {
		t.Errorf("WorstTrade = %v, want -50.0", metrics.WorstTrade)
	}
	if metrics.AvgPnL != 25.0 {
		t.Errorf("AvgPnL = %v, want 25.0", metrics.AvgPnL)
	}
}

func TestActorMetricsCopy(t *testing.T) {
	original := &ActorMetrics{
		Actor:         ActorUser,
		TotalTrades:   10,
		WinningTrades: 6,
		LosingTrades:  4,
		WinRate:       60.0,
		TotalPnL:      500.0,
		AvgPnL:        50.0,
		BestTrade:     200.0,
		WorstTrade:    -100.0,
		LastTradeAt:   time.Now(),
	}

	copied := original.Copy()

	if copied == original {
		t.Error("Copy should return a different pointer")
	}
	if copied.TotalTrades != original.TotalTrades {
		t.Errorf("Copy TotalTrades = %v, want %v", copied.TotalTrades, original.TotalTrades)
	}
	if copied.WinRate != original.WinRate {
		t.Errorf("Copy WinRate = %v, want %v", copied.WinRate, original.WinRate)
	}

	// Test nil copy
	var nilMetrics *ActorMetrics
	if nilMetrics.Copy() != nil {
		t.Error("Copy of nil should return nil")
	}
}

// ========== TradeRecord Tests ==========

func TestTradeRecordCopy(t *testing.T) {
	original := &TradeRecord{
		TradeID:    "trade-1",
		Symbol:     "BTCUSDT",
		Actor:      ActorUser,
		ActorInfo:  NewUserActorInfo("192.168.1.1"),
		Side:       "LONG",
		EntryPrice: 50000.0,
		ExitPrice:  52000.0,
		Quantity:   0.1,
		PnL:        200.0,
		PnLPercent: 4.0,
		Won:        true,
		EnteredAt:  time.Now().Add(-time.Hour),
		ExitedAt:   time.Now(),
		DecisionID: "dec-1",
	}

	copied := original.Copy()

	if copied == original {
		t.Error("Copy should return a different pointer")
	}
	if copied.TradeID != original.TradeID {
		t.Errorf("Copy TradeID = %v, want %v", copied.TradeID, original.TradeID)
	}
	if copied.ActorInfo == original.ActorInfo {
		t.Error("ActorInfo should be deep copied")
	}

	// Test nil copy
	var nilRecord *TradeRecord
	if nilRecord.Copy() != nil {
		t.Error("Copy of nil should return nil")
	}
}

// ========== ActorTrackingService Tests ==========

func TestNewActorTrackingService(t *testing.T) {
	service := NewActorTrackingService()

	if service == nil {
		t.Fatal("NewActorTrackingService returned nil")
	}
	if service.maxTradesPerUser != DefaultMaxTradesPerUser {
		t.Errorf("maxTradesPerUser = %v, want %v", service.maxTradesPerUser, DefaultMaxTradesPerUser)
	}
}

func TestNewActorTrackingServiceWithMaxTrades(t *testing.T) {
	service := NewActorTrackingServiceWithMaxTrades(500)
	if service.maxTradesPerUser != 500 {
		t.Errorf("maxTradesPerUser = %v, want 500", service.maxTradesPerUser)
	}

	// Test with invalid value
	service = NewActorTrackingServiceWithMaxTrades(0)
	if service.maxTradesPerUser != DefaultMaxTradesPerUser {
		t.Errorf("maxTradesPerUser = %v, want %v", service.maxTradesPerUser, DefaultMaxTradesPerUser)
	}
}

func TestRecordTrade(t *testing.T) {
	service := NewActorTrackingService()
	userID := 1

	record := TradeRecord{
		TradeID:    "trade-1",
		Symbol:     "BTCUSDT",
		Actor:      ActorUser,
		ActorInfo:  NewUserActorInfo("192.168.1.1"),
		Side:       "LONG",
		EntryPrice: 50000.0,
		ExitPrice:  52000.0,
		PnL:        200.0,
		Won:        true,
		ExitedAt:   time.Now(),
	}

	err := service.RecordTrade(userID, record)
	if err != nil {
		t.Fatalf("RecordTrade failed: %v", err)
	}

	// Verify metrics updated
	metrics, err := service.GetMetricsByActor(userID, ActorUser)
	if err != nil {
		t.Fatalf("GetMetricsByActor failed: %v", err)
	}
	if metrics.TotalTrades != 1 {
		t.Errorf("TotalTrades = %v, want 1", metrics.TotalTrades)
	}
	if metrics.WinningTrades != 1 {
		t.Errorf("WinningTrades = %v, want 1", metrics.WinningTrades)
	}
}

func TestRecordTradeValidation(t *testing.T) {
	service := NewActorTrackingService()

	tests := []struct {
		name    string
		userID  int
		record  TradeRecord
		wantErr bool
	}{
		{
			name:   "valid record",
			userID: 1,
			record: TradeRecord{
				TradeID: "trade-1",
				Symbol:  "BTCUSDT",
				Actor:   ActorUser,
			},
			wantErr: false,
		},
		{
			name:   "invalid user ID",
			userID: 0,
			record: TradeRecord{
				TradeID: "trade-1",
				Symbol:  "BTCUSDT",
				Actor:   ActorUser,
			},
			wantErr: true,
		},
		{
			name:   "empty trade ID",
			userID: 1,
			record: TradeRecord{
				TradeID: "",
				Symbol:  "BTCUSDT",
				Actor:   ActorUser,
			},
			wantErr: true,
		},
		{
			name:   "empty symbol",
			userID: 1,
			record: TradeRecord{
				TradeID: "trade-1",
				Symbol:  "",
				Actor:   ActorUser,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.RecordTrade(tt.userID, tt.record)
			if (err != nil) != tt.wantErr {
				t.Errorf("RecordTrade() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRecordTradeDuplicateID(t *testing.T) {
	service := NewActorTrackingService()
	userID := 1

	record := TradeRecord{
		TradeID:  "trade-1",
		Symbol:   "BTCUSDT",
		Actor:    ActorUser,
		PnL:      100,
		Won:      true,
		ExitedAt: time.Now(),
	}

	// First record should succeed
	err := service.RecordTrade(userID, record)
	if err != nil {
		t.Fatalf("First RecordTrade failed: %v", err)
	}

	// Duplicate should fail
	err = service.RecordTrade(userID, record)
	if err == nil {
		t.Error("Expected error for duplicate trade ID, got nil")
	}

	// Verify metrics only counted once
	metrics, _ := service.GetMetricsByActor(userID, ActorUser)
	if metrics.TotalTrades != 1 {
		t.Errorf("TotalTrades = %v, want 1 (duplicate should not count)", metrics.TotalTrades)
	}
}

func TestRecordTradeDuplicateIDDifferentUser(t *testing.T) {
	service := NewActorTrackingService()

	record := TradeRecord{
		TradeID:  "trade-1",
		Symbol:   "BTCUSDT",
		Actor:    ActorUser,
		PnL:      100,
		Won:      true,
		ExitedAt: time.Now(),
	}

	// Record for user 1
	err := service.RecordTrade(1, record)
	if err != nil {
		t.Fatalf("RecordTrade for user 1 failed: %v", err)
	}

	// Same trade ID for different user should succeed
	err = service.RecordTrade(2, record)
	if err != nil {
		t.Errorf("RecordTrade for user 2 should succeed, got error: %v", err)
	}
}

func TestGetMetricsByActor(t *testing.T) {
	service := NewActorTrackingService()
	userID := 1

	// Record some trades
	service.RecordTrade(userID, TradeRecord{
		TradeID: "trade-1", Symbol: "BTCUSDT", Actor: ActorUser,
		PnL: 100, Won: true, ExitedAt: time.Now(),
	})
	service.RecordTrade(userID, TradeRecord{
		TradeID: "trade-2", Symbol: "ETHUSDT", Actor: ActorUser,
		PnL: -50, Won: false, ExitedAt: time.Now(),
	})
	service.RecordTrade(userID, TradeRecord{
		TradeID: "trade-3", Symbol: "BTCUSDT", Actor: ActorGinieAuto,
		PnL: 150, Won: true, ExitedAt: time.Now(),
	})

	// Get USER metrics
	userMetrics, err := service.GetMetricsByActor(userID, ActorUser)
	if err != nil {
		t.Fatalf("GetMetricsByActor failed: %v", err)
	}
	if userMetrics.TotalTrades != 2 {
		t.Errorf("User TotalTrades = %v, want 2", userMetrics.TotalTrades)
	}
	if userMetrics.WinRate != 50.0 {
		t.Errorf("User WinRate = %v, want 50.0", userMetrics.WinRate)
	}

	// Get AUTO metrics
	autoMetrics, err := service.GetMetricsByActor(userID, ActorGinieAuto)
	if err != nil {
		t.Fatalf("GetMetricsByActor failed: %v", err)
	}
	if autoMetrics.TotalTrades != 1 {
		t.Errorf("Auto TotalTrades = %v, want 1", autoMetrics.TotalTrades)
	}
	if autoMetrics.WinRate != 100.0 {
		t.Errorf("Auto WinRate = %v, want 100.0", autoMetrics.WinRate)
	}

	// Get metrics for actor with no trades
	systemMetrics, err := service.GetMetricsByActor(userID, ActorSystem)
	if err != nil {
		t.Fatalf("GetMetricsByActor failed: %v", err)
	}
	if systemMetrics.TotalTrades != 0 {
		t.Errorf("System TotalTrades = %v, want 0", systemMetrics.TotalTrades)
	}
}

func TestGetAllMetrics(t *testing.T) {
	service := NewActorTrackingService()
	userID := 1

	// Record trades for different actors
	service.RecordTrade(userID, TradeRecord{
		TradeID: "trade-1", Symbol: "BTCUSDT", Actor: ActorUser,
		PnL: 100, Won: true, ExitedAt: time.Now(),
	})
	service.RecordTrade(userID, TradeRecord{
		TradeID: "trade-2", Symbol: "BTCUSDT", Actor: ActorGinieAuto,
		PnL: 150, Won: true, ExitedAt: time.Now(),
	})

	allMetrics, err := service.GetAllMetrics(userID)
	if err != nil {
		t.Fatalf("GetAllMetrics failed: %v", err)
	}

	if len(allMetrics) != 2 {
		t.Errorf("GetAllMetrics returned %d actors, want 2", len(allMetrics))
	}

	if allMetrics[ActorUser] == nil {
		t.Error("Expected USER metrics to exist")
	}
	if allMetrics[ActorGinieAuto] == nil {
		t.Error("Expected GINIE_AUTO metrics to exist")
	}
}

func TestGetTradesByActor(t *testing.T) {
	service := NewActorTrackingService()
	userID := 1

	// Record trades
	service.RecordTrade(userID, TradeRecord{
		TradeID: "trade-1", Symbol: "BTCUSDT", Actor: ActorUser,
		ExitedAt: time.Now().Add(-2 * time.Hour),
	})
	service.RecordTrade(userID, TradeRecord{
		TradeID: "trade-2", Symbol: "ETHUSDT", Actor: ActorGinieAuto,
		ExitedAt: time.Now().Add(-1 * time.Hour),
	})
	service.RecordTrade(userID, TradeRecord{
		TradeID: "trade-3", Symbol: "BTCUSDT", Actor: ActorUser,
		ExitedAt: time.Now(),
	})

	// Get USER trades
	trades, err := service.GetTradesByActor(userID, ActorUser, 10)
	if err != nil {
		t.Fatalf("GetTradesByActor failed: %v", err)
	}
	if len(trades) != 2 {
		t.Errorf("GetTradesByActor returned %d trades, want 2", len(trades))
	}

	// Verify most recent first
	if trades[0].TradeID != "trade-3" {
		t.Errorf("First trade should be trade-3, got %v", trades[0].TradeID)
	}

	// Get AUTO trades
	autoTrades, err := service.GetTradesByActor(userID, ActorGinieAuto, 10)
	if err != nil {
		t.Fatalf("GetTradesByActor failed: %v", err)
	}
	if len(autoTrades) != 1 {
		t.Errorf("GetTradesByActor returned %d trades, want 1", len(autoTrades))
	}
}

func TestGetTradeHistory(t *testing.T) {
	service := NewActorTrackingService()
	userID := 1

	// Record trades
	for i := 1; i <= 5; i++ {
		service.RecordTrade(userID, TradeRecord{
			TradeID:  "trade-" + string(rune('0'+i)),
			Symbol:   "BTCUSDT",
			Actor:    ActorUser,
			ExitedAt: time.Now().Add(time.Duration(i) * time.Minute),
		})
	}

	// Get all trades
	trades, err := service.GetTradeHistory(userID, 100)
	if err != nil {
		t.Fatalf("GetTradeHistory failed: %v", err)
	}
	if len(trades) != 5 {
		t.Errorf("GetTradeHistory returned %d trades, want 5", len(trades))
	}

	// Get limited trades
	limitedTrades, err := service.GetTradeHistory(userID, 2)
	if err != nil {
		t.Fatalf("GetTradeHistory failed: %v", err)
	}
	if len(limitedTrades) != 2 {
		t.Errorf("GetTradeHistory with limit returned %d trades, want 2", len(limitedTrades))
	}
}

func TestGetActorComparison(t *testing.T) {
	service := NewActorTrackingService()
	userID := 1

	// Record trades to meet threshold (5 trades per actor)
	for i := 0; i < 6; i++ {
		won := i%2 == 0
		pnl := 100.0
		if !won {
			pnl = -50.0
		}
		service.RecordTrade(userID, TradeRecord{
			TradeID:  "user-" + string(rune('0'+i)),
			Symbol:   "BTCUSDT",
			Actor:    ActorUser,
			PnL:      pnl,
			Won:      won,
			ExitedAt: time.Now(),
		})
	}

	for i := 0; i < 6; i++ {
		won := i%3 != 0 // 66% win rate
		pnl := 100.0
		if !won {
			pnl = -50.0
		}
		service.RecordTrade(userID, TradeRecord{
			TradeID:  "auto-" + string(rune('0'+i)),
			Symbol:   "BTCUSDT",
			Actor:    ActorGinieAuto,
			PnL:      pnl,
			Won:      won,
			ExitedAt: time.Now(),
		})
	}

	comparison, err := service.GetActorComparison(userID)
	if err != nil {
		t.Fatalf("GetActorComparison failed: %v", err)
	}

	if comparison.TotalTrades != 12 {
		t.Errorf("TotalTrades = %v, want 12", comparison.TotalTrades)
	}
	if len(comparison.Actors) != 2 {
		t.Errorf("Actors count = %v, want 2", len(comparison.Actors))
	}
	if comparison.Analysis == "" {
		t.Error("Analysis should not be empty")
	}
}

func TestGetActorComparisonForPeriod(t *testing.T) {
	service := NewActorTrackingService()
	userID := 1

	// Record trades at different times
	now := time.Now()

	// Recent trade (within last day)
	service.RecordTrade(userID, TradeRecord{
		TradeID:  "recent",
		Symbol:   "BTCUSDT",
		Actor:    ActorUser,
		PnL:      100,
		Won:      true,
		ExitedAt: now.Add(-1 * time.Hour),
	})

	// Old trade (1 week ago)
	service.RecordTrade(userID, TradeRecord{
		TradeID:  "old",
		Symbol:   "BTCUSDT",
		Actor:    ActorUser,
		PnL:      -50,
		Won:      false,
		ExitedAt: now.Add(-8 * 24 * time.Hour),
	})

	// Get day comparison
	dayComparison, err := service.GetActorComparisonForPeriod(userID, "day")
	if err != nil {
		t.Fatalf("GetActorComparisonForPeriod failed: %v", err)
	}
	if dayComparison.TotalTrades != 1 {
		t.Errorf("Day TotalTrades = %v, want 1", dayComparison.TotalTrades)
	}

	// Get all-time comparison
	allComparison, err := service.GetActorComparisonForPeriod(userID, "all")
	if err != nil {
		t.Fatalf("GetActorComparisonForPeriod failed: %v", err)
	}
	if allComparison.TotalTrades != 2 {
		t.Errorf("All TotalTrades = %v, want 2", allComparison.TotalTrades)
	}
}

func TestGetActorComparisonForPeriodInvalidPeriod(t *testing.T) {
	service := NewActorTrackingService()
	userID := 1

	// Test invalid period strings
	invalidPeriods := []string{"daily", "weekly", "monthly", "yearly", "invalid", ""}

	for _, period := range invalidPeriods {
		_, err := service.GetActorComparisonForPeriod(userID, period)
		if err == nil {
			t.Errorf("Expected error for invalid period '%s', got nil", period)
		}
	}
}

func TestGetTradesByActorForPeriodInvalidPeriod(t *testing.T) {
	service := NewActorTrackingService()
	userID := 1

	// Test invalid period strings
	invalidPeriods := []string{"daily", "weekly", "monthly", "yearly", "invalid", ""}

	for _, period := range invalidPeriods {
		_, err := service.GetTradesByActorForPeriod(userID, ActorUser, period, 10)
		if err == nil {
			t.Errorf("Expected error for invalid period '%s', got nil", period)
		}
	}
}

func TestIsValidPeriod(t *testing.T) {
	tests := []struct {
		period   string
		expected bool
	}{
		{"day", true},
		{"week", true},
		{"month", true},
		{"all", true},
		{"daily", false},
		{"weekly", false},
		{"monthly", false},
		{"", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			if got := IsValidPeriod(tt.period); got != tt.expected {
				t.Errorf("IsValidPeriod(%v) = %v, want %v", tt.period, got, tt.expected)
			}
		})
	}
}

func TestGetLeaderboard(t *testing.T) {
	service := NewActorTrackingService()
	userID := 1

	// Record trades with different win rates
	for i := 0; i < 10; i++ {
		service.RecordTrade(userID, TradeRecord{
			TradeID:  "user-" + string(rune('0'+i)),
			Symbol:   "BTCUSDT",
			Actor:    ActorUser,
			PnL:      100,
			Won:      i < 5, // 50% win rate
			ExitedAt: time.Now(),
		})
	}

	for i := 0; i < 5; i++ {
		service.RecordTrade(userID, TradeRecord{
			TradeID:  "auto-" + string(rune('0'+i)),
			Symbol:   "BTCUSDT",
			Actor:    ActorGinieAuto,
			PnL:      100,
			Won:      i < 4, // 80% win rate
			ExitedAt: time.Now(),
		})
	}

	leaderboard, err := service.GetLeaderboard(userID)
	if err != nil {
		t.Fatalf("GetLeaderboard failed: %v", err)
	}

	if len(leaderboard) != 2 {
		t.Errorf("Leaderboard length = %v, want 2", len(leaderboard))
	}

	// AUTO should be first (higher win rate)
	if leaderboard[0].Actor != ActorGinieAuto {
		t.Errorf("First in leaderboard = %v, want GINIE_AUTO", leaderboard[0].Actor)
	}
}

func TestClearUserData(t *testing.T) {
	service := NewActorTrackingService()
	userID := 1

	// Record some trades
	service.RecordTrade(userID, TradeRecord{
		TradeID: "trade-1", Symbol: "BTCUSDT", Actor: ActorUser,
		ExitedAt: time.Now(),
	})

	// Verify data exists
	trades, _ := service.GetTradeHistory(userID, 100)
	if len(trades) != 1 {
		t.Errorf("Expected 1 trade before clear, got %d", len(trades))
	}

	// Clear data
	service.ClearUserData(userID)

	// Verify data cleared
	trades, _ = service.GetTradeHistory(userID, 100)
	if len(trades) != 0 {
		t.Errorf("Expected 0 trades after clear, got %d", len(trades))
	}

	metrics, _ := service.GetAllMetrics(userID)
	if len(metrics) != 0 {
		t.Errorf("Expected 0 metrics after clear, got %d", len(metrics))
	}
}

func TestActorTrackingServiceGetStats(t *testing.T) {
	service := NewActorTrackingService()

	// Record trades for multiple users
	service.RecordTrade(1, TradeRecord{
		TradeID: "trade-1", Symbol: "BTCUSDT", Actor: ActorUser,
		ExitedAt: time.Now(),
	})
	service.RecordTrade(2, TradeRecord{
		TradeID: "trade-2", Symbol: "BTCUSDT", Actor: ActorGinieAuto,
		ExitedAt: time.Now(),
	})

	stats := service.GetStats()

	if stats.TotalUsers != 2 {
		t.Errorf("TotalUsers = %v, want 2", stats.TotalUsers)
	}
	if stats.TotalTrades != 2 {
		t.Errorf("TotalTrades = %v, want 2", stats.TotalTrades)
	}
	if stats.TradesByActor[ActorUser] != 1 {
		t.Errorf("TradesByActor[USER] = %v, want 1", stats.TradesByActor[ActorUser])
	}
	if stats.TradesByActor[ActorGinieAuto] != 1 {
		t.Errorf("TradesByActor[GINIE_AUTO] = %v, want 1", stats.TradesByActor[ActorGinieAuto])
	}
}

// ========== ActorResponse Tests ==========

func TestActorToResponse(t *testing.T) {
	resp := ActorUser.ToResponse()

	if resp.Actor != "USER" {
		t.Errorf("Actor = %v, want USER", resp.Actor)
	}
	if resp.DisplayName != "Manual" {
		t.Errorf("DisplayName = %v, want Manual", resp.DisplayName)
	}
	if resp.Badge != "U" {
		t.Errorf("Badge = %v, want U", resp.Badge)
	}
}

func TestActorToResponseWithDescription(t *testing.T) {
	resp := ActorUser.ToResponseWithDescription("Manual trade from web UI")

	if resp.Description != "Manual trade from web UI" {
		t.Errorf("Description = %v, want 'Manual trade from web UI'", resp.Description)
	}
}

func TestActorInfoToResponse(t *testing.T) {
	info := &ActorInfo{
		Actor:       ActorUser,
		Description: "Test trade",
		InitiatedAt: time.Now(),
		SourceIP:    "192.168.1.1",
	}

	resp := info.ToResponse()

	if resp == nil {
		t.Fatal("ToResponse returned nil")
	}
	if resp.Actor.Actor != "USER" {
		t.Errorf("Actor = %v, want USER", resp.Actor.Actor)
	}
	if resp.SourceIP != "192.168.1.1" {
		t.Errorf("SourceIP = %v, want 192.168.1.1", resp.SourceIP)
	}

	// Test nil
	var nilInfo *ActorInfo
	if nilInfo.ToResponse() != nil {
		t.Error("ToResponse of nil should return nil")
	}
}

// ========== Integration with PositionDecision Tests ==========

func TestDecisionBuilderWithActor(t *testing.T) {
	actorInfo := NewAutoActorInfo("trend_following")

	decision := NewDecisionBuilder("BTCUSDT", 1).
		WithActor(ActorGinieAuto, actorInfo).
		WithScore(80, 0.75).
		Build()

	if decision.Actor != ActorGinieAuto {
		t.Errorf("Actor = %v, want GINIE_AUTO", decision.Actor)
	}
	if decision.ActorInfo == nil {
		t.Fatal("ActorInfo should not be nil")
	}
	if decision.ActorInfo.AutoRule != "trend_following" {
		t.Errorf("AutoRule = %v, want trend_following", decision.ActorInfo.AutoRule)
	}
}

func TestDecisionBuilderWithActorNilInfo(t *testing.T) {
	decision := NewDecisionBuilder("BTCUSDT", 1).
		WithActor(ActorUser, nil).
		WithScore(80, 0.75).
		Build()

	if decision.Actor != ActorUser {
		t.Errorf("Actor = %v, want USER", decision.Actor)
	}
	if decision.ActorInfo != nil {
		t.Error("ActorInfo should be nil when nil is passed")
	}
}

func TestPositionDecisionDefaultActor(t *testing.T) {
	decision := NewPositionDecision("BTCUSDT", 1)

	if decision.Actor != ActorUnknown {
		t.Errorf("Default Actor = %v, want UNKNOWN", decision.Actor)
	}
}

func TestPositionDecisionCopyWithActor(t *testing.T) {
	original := NewDecisionBuilder("BTCUSDT", 1).
		WithActor(ActorGinieAuto, NewAutoActorInfo("breakout")).
		WithScore(75, 0.7).
		Build()

	copied := original.Copy()

	if copied.Actor != original.Actor {
		t.Errorf("Copy Actor = %v, want %v", copied.Actor, original.Actor)
	}
	if copied.ActorInfo == original.ActorInfo {
		t.Error("ActorInfo should be deep copied")
	}
	if copied.ActorInfo.AutoRule != original.ActorInfo.AutoRule {
		t.Errorf("Copy AutoRule = %v, want %v", copied.ActorInfo.AutoRule, original.ActorInfo.AutoRule)
	}
}

func TestDecisionResponseIncludesActor(t *testing.T) {
	decision := NewDecisionBuilder("BTCUSDT", 1).
		WithActor(ActorUser, NewUserActorInfo("192.168.1.1")).
		WithScore(80, 0.75).
		Build()

	response := decision.ToResponse(70)

	if response.Actor.Actor != "USER" {
		t.Errorf("Response Actor = %v, want USER", response.Actor.Actor)
	}
	if response.Actor.DisplayName != "Manual" {
		t.Errorf("Response DisplayName = %v, want Manual", response.Actor.DisplayName)
	}
}

func TestDecisionSummaryIncludesActor(t *testing.T) {
	decision := NewDecisionBuilder("BTCUSDT", 1).
		WithActor(ActorGinieAuto, nil).
		WithScore(80, 0.75).
		Build()

	summary := decision.ToSummary()

	if summary.Actor.Actor != "GINIE_AUTO" {
		t.Errorf("Summary Actor = %v, want GINIE_AUTO", summary.Actor.Actor)
	}
}

// ========== Concurrent Access Tests ==========

func TestActorTrackingServiceConcurrentAccess(t *testing.T) {
	service := NewActorTrackingService()
	userID := 1

	// Run concurrent operations
	var wg sync.WaitGroup
	numGoroutines := 10
	tradesPerGoroutine := 50

	// Concurrent writes
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < tradesPerGoroutine; i++ {
				tradeID := fmt.Sprintf("trade-%d-%d", goroutineID, i)
				record := TradeRecord{
					TradeID:  tradeID,
					Symbol:   "BTCUSDT",
					Actor:    ActorUser,
					PnL:      float64(i * 10),
					Won:      i%2 == 0,
					ExitedAt: time.Now(),
				}
				_ = service.RecordTrade(userID, record)
			}
		}(g)
	}

	// Concurrent reads while writing
	for g := 0; g < numGoroutines/2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < tradesPerGoroutine; i++ {
				_, _ = service.GetMetricsByActor(userID, ActorUser)
				_, _ = service.GetTradeHistory(userID, 10)
				_, _ = service.GetActorComparison(userID)
			}
		}()
	}

	wg.Wait()

	// Verify final state
	metrics, err := service.GetMetricsByActor(userID, ActorUser)
	if err != nil {
		t.Fatalf("GetMetricsByActor failed: %v", err)
	}

	expectedTrades := int64(numGoroutines * tradesPerGoroutine)
	if metrics.TotalTrades != expectedTrades {
		t.Errorf("TotalTrades = %v, want %v", metrics.TotalTrades, expectedTrades)
	}
}

func TestActorTrackingServiceConcurrentClearData(t *testing.T) {
	service := NewActorTrackingService()

	// Add some initial data
	for i := 0; i < 100; i++ {
		service.RecordTrade(1, TradeRecord{
			TradeID:  fmt.Sprintf("trade-%d", i),
			Symbol:   "BTCUSDT",
			Actor:    ActorUser,
			ExitedAt: time.Now(),
		})
	}

	// Concurrent operations including clear
	var wg sync.WaitGroup

	// Reader goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_, _ = service.GetMetricsByActor(1, ActorUser)
		}
	}()

	// Clear goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		service.ClearUserData(1)
	}()

	// Writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 100; i < 150; i++ {
			service.RecordTrade(1, TradeRecord{
				TradeID:  fmt.Sprintf("trade-%d", i),
				Symbol:   "BTCUSDT",
				Actor:    ActorUser,
				ExitedAt: time.Now(),
			})
		}
	}()

	wg.Wait()

	// Test should complete without panics - state may vary due to race timing
}
