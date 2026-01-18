// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.19: Decision Output Interface - Tests
package decision

import (
	"fmt"
	"testing"
	"time"
)

// ========== PositionDecision Tests ==========

func TestNewPositionDecision(t *testing.T) {
	d := NewPositionDecision("BTCUSDT", 123)

	if d.ID == "" {
		t.Error("Expected non-empty ID")
	}
	if d.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %s", d.Symbol)
	}
	if d.UserID != 123 {
		t.Errorf("Expected userID 123, got %d", d.UserID)
	}
	if d.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
	if d.Indicators == nil {
		t.Error("Expected non-nil indicators map")
	}
	if d.BlockingReasons == nil {
		t.Error("Expected non-nil blocking reasons slice")
	}
	if d.DecisionChain != 1 {
		t.Errorf("Expected decision chain 1, got %d", d.DecisionChain)
	}
}

func TestDecisionBuilder_WithScore(t *testing.T) {
	tests := []struct {
		name           string
		score          int
		calibrated     float64
		wantScore      int
		wantConfidence string
	}{
		{"high score", 85, 0.75, 85, "HIGH"},
		{"medium score", 65, 0.55, 65, "MEDIUM"},
		{"low score", 40, 0.35, 40, "LOW"},
		{"clamped high", 150, 0.9, 100, "HIGH"},
		{"clamped low", -10, 0.1, 0, "LOW"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDecisionBuilder("ETHUSDT", 1).
				WithScore(tt.score, tt.calibrated).
				Build()

			if d.Score != tt.wantScore {
				t.Errorf("Expected score %d, got %d", tt.wantScore, d.Score)
			}
			if d.Confidence != tt.wantConfidence {
				t.Errorf("Expected confidence %s, got %s", tt.wantConfidence, d.Confidence)
			}
			if d.CalibratedConf != tt.calibrated {
				t.Errorf("Expected calibrated conf %f, got %f", tt.calibrated, d.CalibratedConf)
			}
		})
	}
}

func TestDecisionBuilder_WithStrategy(t *testing.T) {
	d := NewDecisionBuilder("BTCUSDT", 1).
		WithStrategy("trend_following", RegimeTrending, 0.85).
		Build()

	if d.Strategy != "trend_following" {
		t.Errorf("Expected strategy trend_following, got %s", d.Strategy)
	}
	if d.Regime != RegimeTrending {
		t.Errorf("Expected regime TRENDING, got %s", d.Regime)
	}
	if d.RegimeConf != 0.85 {
		t.Errorf("Expected regime conf 0.85, got %f", d.RegimeConf)
	}
}

func TestDecisionBuilder_WithBlocking(t *testing.T) {
	reasons := []BlockingReason{
		{Code: ReasonADXTooLow, Category: CategoryHardBlock, Description: "ADX too low"},
		{Code: ReasonHighRSI, Category: CategoryWarning, Description: "RSI overbought"},
	}

	d := NewDecisionBuilder("BTCUSDT", 1).
		WithBlocking(reasons).
		Build()

	if !d.IsBlocked {
		t.Error("Expected IsBlocked to be true")
	}
	if len(d.BlockingReasons) != 2 {
		t.Errorf("Expected 2 blocking reasons, got %d", len(d.BlockingReasons))
	}
	if d.Action != ActionBlocked {
		t.Errorf("Expected action BLOCKED, got %s", d.Action)
	}
}

func TestDecisionBuilder_WithBlockingNil(t *testing.T) {
	d := NewDecisionBuilder("BTCUSDT", 1).
		WithBlocking(nil).
		Build()

	if d.IsBlocked {
		t.Error("Expected IsBlocked to be false for nil reasons")
	}
	if d.BlockingReasons == nil {
		t.Error("Expected non-nil empty slice")
	}
}

func TestDecisionBuilder_WithIndicators(t *testing.T) {
	indicators := map[string]float64{
		"adx": 25.5,
		"rsi": 65.0,
	}

	d := NewDecisionBuilder("BTCUSDT", 1).
		WithIndicators(indicators).
		Build()

	if d.Indicators["adx"] != 25.5 {
		t.Errorf("Expected adx 25.5, got %f", d.Indicators["adx"])
	}

	// Verify map was copied (not shared reference)
	indicators["adx"] = 100
	if d.Indicators["adx"] != 25.5 {
		t.Error("Indicators map should be copied, not shared")
	}
}

func TestDecisionBuilder_WithLLMContext(t *testing.T) {
	ctx := &LLMContext{
		Symbol:       "BTCUSDT",
		MarketRegime: RegimeTrending,
		FinalScore:   75,
	}

	d := NewDecisionBuilder("BTCUSDT", 1).
		WithLLMContext(ctx).
		Build()

	if d.LLMContext == nil {
		t.Error("Expected non-nil LLMContext")
	}
	if d.LLMContext.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT in LLMContext, got %s", d.LLMContext.Symbol)
	}
}

func TestDecisionBuilder_WithPreviousDecision(t *testing.T) {
	d := NewDecisionBuilder("BTCUSDT", 1).
		WithPreviousDecision("prev-123", 5).
		Build()

	if d.PreviousDecisionID != "prev-123" {
		t.Errorf("Expected previous decision ID prev-123, got %s", d.PreviousDecisionID)
	}
	if d.DecisionChain != 5 {
		t.Errorf("Expected decision chain 5, got %d", d.DecisionChain)
	}
}

func TestDecisionBuilder_WithPreviousDecisionMinChain(t *testing.T) {
	d := NewDecisionBuilder("BTCUSDT", 1).
		WithPreviousDecision("prev-123", 0).
		Build()

	if d.DecisionChain != 1 {
		t.Errorf("Expected minimum chain count 1, got %d", d.DecisionChain)
	}
}

func TestDetermineAction(t *testing.T) {
	tests := []struct {
		name      string
		score     int
		isBlocked bool
		regime    MarketRegime
		expected  Action
	}{
		{"blocked takes precedence", 90, true, RegimeTrending, ActionBlocked},
		{"high score trending", 80, false, RegimeTrending, ActionEnterLong},
		{"high score volatile", 80, false, RegimeVolatile, ActionEnterLong},
		{"high score ranging", 80, false, RegimeRanging, ActionHold},
		{"medium score", 60, false, RegimeTrending, ActionHold},
		{"low score", 30, false, RegimeTrending, ActionHold},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineAction(tt.score, tt.isBlocked, tt.regime)
			if result != tt.expected {
				t.Errorf("Expected action %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestDetermineActionWithDirection(t *testing.T) {
	tests := []struct {
		name      string
		score     int
		isBlocked bool
		isLong    bool
		threshold int
		expected  Action
	}{
		{"blocked", 90, true, true, 75, ActionBlocked},
		{"long entry", 80, false, true, 75, ActionEnterLong},
		{"short entry", 80, false, false, 75, ActionEnterShort},
		{"below threshold", 70, false, true, 75, ActionHold},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineActionWithDirection(tt.score, tt.isBlocked, tt.isLong, tt.threshold)
			if result != tt.expected {
				t.Errorf("Expected action %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetConfidenceLevel(t *testing.T) {
	tests := []struct {
		score    int
		expected string
	}{
		{100, "HIGH"},
		{75, "HIGH"},
		{74, "MEDIUM"},
		{55, "MEDIUM"},
		{54, "LOW"},
		{0, "LOW"},
	}

	for _, tt := range tests {
		result := GetConfidenceLevel(tt.score)
		if result != tt.expected {
			t.Errorf("GetConfidenceLevel(%d) = %s, want %s", tt.score, result, tt.expected)
		}
	}
}

func TestPositionDecision_MarkExecuted(t *testing.T) {
	d := NewPositionDecision("BTCUSDT", 1)
	d.MarkExecuted("order-123")

	if d.OrderID != "order-123" {
		t.Errorf("Expected order ID order-123, got %s", d.OrderID)
	}
	if d.ExecutedAt == nil {
		t.Error("Expected ExecutedAt to be set")
	}
	if !d.IsExecuted() {
		t.Error("Expected IsExecuted() to return true")
	}
}

func TestPositionDecision_MarkError(t *testing.T) {
	d := NewPositionDecision("BTCUSDT", 1)
	d.MarkError("connection timeout")

	if d.ExecutionError != "connection timeout" {
		t.Errorf("Expected error message 'connection timeout', got %s", d.ExecutionError)
	}
	if !d.HasError() {
		t.Error("Expected HasError() to return true")
	}
}

func TestPositionDecision_MarkRolledBack(t *testing.T) {
	d := NewPositionDecision("BTCUSDT", 1)
	d.MarkRolledBack("market conditions changed")

	if !d.RolledBack {
		t.Error("Expected RolledBack to be true")
	}
	if d.RollbackReason != "market conditions changed" {
		t.Errorf("Expected rollback reason 'market conditions changed', got %s", d.RollbackReason)
	}
	if d.RollbackAt == nil {
		t.Error("Expected RollbackAt to be set")
	}
}

func TestPositionDecision_Copy(t *testing.T) {
	original := NewDecisionBuilder("BTCUSDT", 1).
		WithScore(75, 0.65).
		WithStrategy("trend_following", RegimeTrending, 0.8).
		WithIndicators(map[string]float64{"adx": 30}).
		Build()
	original.MarkExecuted("order-123")

	copy := original.Copy()

	// Verify copy has same values
	if copy.ID != original.ID {
		t.Error("Copy should have same ID")
	}
	if copy.Score != original.Score {
		t.Error("Copy should have same score")
	}
	if copy.OrderID != original.OrderID {
		t.Error("Copy should have same order ID")
	}

	// Verify maps are not shared
	copy.Indicators["adx"] = 100
	if original.Indicators["adx"] != 30 {
		t.Error("Modifying copy should not affect original")
	}
}

func TestPositionDecision_CopyNil(t *testing.T) {
	var d *PositionDecision = nil
	copy := d.Copy()
	if copy != nil {
		t.Error("Copy of nil should be nil")
	}
}

func TestPositionDecision_CopyWithLLMContext(t *testing.T) {
	// Create decision with LLMContext containing BlockingReasons slice
	ctx := &LLMContext{
		Symbol:          "BTCUSDT",
		MarketRegime:    RegimeTrending,
		FinalScore:      75,
		BlockingReasons: []string{"reason1", "reason2"},
	}

	original := NewDecisionBuilder("BTCUSDT", 1).
		WithScore(75, 0.65).
		WithLLMContext(ctx).
		Build()

	copy := original.Copy()

	// Verify LLMContext is copied
	if copy.LLMContext == nil {
		t.Fatal("Copy should have LLMContext")
	}
	if copy.LLMContext == original.LLMContext {
		t.Error("Copy should have different LLMContext pointer")
	}
	if len(copy.LLMContext.BlockingReasons) != 2 {
		t.Errorf("Expected 2 blocking reasons in copy, got %d", len(copy.LLMContext.BlockingReasons))
	}

	// Verify BlockingReasons slice is not shared
	copy.LLMContext.BlockingReasons[0] = "modified"
	if original.LLMContext.BlockingReasons[0] != "reason1" {
		t.Error("Modifying copy's LLMContext.BlockingReasons should not affect original")
	}
}

// ========== DecisionResponse Tests ==========

func TestPositionDecision_ToResponse_Recommended(t *testing.T) {
	d := NewDecisionBuilder("BTCUSDT", 1).
		WithScore(80, 0.7).
		WithStrategy("trend_following", RegimeTrending, 0.8).
		WithAction(ActionEnterLong).
		Build()

	resp := d.ToResponse(75)

	if !resp.Recommended {
		t.Error("Expected recommended to be true for score >= threshold")
	}
	if resp.NextAction != "execute" {
		t.Errorf("Expected next action 'execute', got %s", resp.NextAction)
	}
	if resp.Message == "" {
		t.Error("Expected non-empty message")
	}
}

func TestPositionDecision_ToResponse_Blocked(t *testing.T) {
	reasons := []BlockingReason{
		{Code: ReasonADXTooLow, Description: "ADX below threshold"},
	}

	d := NewDecisionBuilder("BTCUSDT", 1).
		WithScore(80, 0.7).
		WithBlocking(reasons).
		Build()

	resp := d.ToResponse(75)

	if resp.Recommended {
		t.Error("Expected recommended to be false when blocked")
	}
	if resp.NextAction != "skip" {
		t.Errorf("Expected next action 'skip', got %s", resp.NextAction)
	}
}

func TestPositionDecision_ToResponse_Review(t *testing.T) {
	d := NewDecisionBuilder("BTCUSDT", 1).
		WithScore(60, 0.5).
		Build()

	resp := d.ToResponse(75)

	if resp.Recommended {
		t.Error("Expected recommended to be false for score < threshold")
	}
	if resp.NextAction != "review" {
		t.Errorf("Expected next action 'review', got %s", resp.NextAction)
	}
}

func TestPositionDecision_ToSummary(t *testing.T) {
	d := NewDecisionBuilder("BTCUSDT", 1).
		WithScore(80, 0.7).
		WithStrategy("trend_following", RegimeTrending, 0.8).
		WithAction(ActionEnterLong).
		Build()

	summary := d.ToSummary()

	if summary.ID != d.ID {
		t.Error("Summary should have same ID")
	}
	if summary.Symbol != d.Symbol {
		t.Error("Summary should have same symbol")
	}
	if summary.Action != d.Action {
		t.Error("Summary should have same action")
	}
	if summary.Score != d.Score {
		t.Error("Summary should have same score")
	}
}

// ========== Integration Helper Tests ==========

func TestBuildFromCoinState(t *testing.T) {
	state := &CoinState{
		Symbol:         "BTCUSDT",
		Price:          50000,
		Regime:         RegimeTrending,
		ActiveStrategy: "trend_following",
		ADX:            30,
		RSI:            55,
		EMA9:           49500,
		EMA21:          49000,
	}

	d := BuildFromCoinState(state, 1, 75, 0.65)

	if d == nil {
		t.Fatal("Expected non-nil decision")
	}
	if d.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %s", d.Symbol)
	}
	if d.Score != 75 {
		t.Errorf("Expected score 75, got %d", d.Score)
	}
	if d.Strategy != "trend_following" {
		t.Errorf("Expected strategy trend_following, got %s", d.Strategy)
	}
	if d.Indicators["adx"] != 30 {
		t.Errorf("Expected ADX indicator 30, got %f", d.Indicators["adx"])
	}
}

func TestBuildFromCoinState_Blocked(t *testing.T) {
	state := &CoinState{
		Symbol:          "BTCUSDT",
		BlockingReasons: []string{"ADX too low", "RSI overbought"},
	}

	d := BuildFromCoinState(state, 1, 75, 0.65)

	if !d.IsBlocked {
		t.Error("Expected decision to be blocked")
	}
	if len(d.BlockingReasons) != 2 {
		t.Errorf("Expected 2 blocking reasons, got %d", len(d.BlockingReasons))
	}
}

func TestBuildFromCoinStateNil(t *testing.T) {
	d := BuildFromCoinState(nil, 1, 75, 0.65)
	if d != nil {
		t.Error("Expected nil result for nil state")
	}
}

func TestBuildFromLLMContext(t *testing.T) {
	ctx := &LLMContext{
		Symbol:           "BTCUSDT",
		MarketRegime:     RegimeTrending,
		RegimeConfidence: 0.85,
		ActiveStrategy:   "trend_following",
		ADXValue:         30,
		RSI:              55,
		FinalScore:       75,
	}

	d := BuildFromLLMContext(ctx, 1)

	if d == nil {
		t.Fatal("Expected non-nil decision")
	}
	if d.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %s", d.Symbol)
	}
	if d.Score != 75 {
		t.Errorf("Expected score 75, got %d", d.Score)
	}
	if d.LLMContext == nil {
		t.Error("Expected LLM context to be attached")
	}
}

func TestBuildFromLLMContextNil(t *testing.T) {
	d := BuildFromLLMContext(nil, 1)
	if d != nil {
		t.Error("Expected nil result for nil context")
	}
}

// ========== DecisionAuditService Tests ==========

func TestNewDecisionAuditService(t *testing.T) {
	svc := NewDecisionAuditService()
	if svc == nil {
		t.Fatal("Expected non-nil service")
	}
	if svc.maxHistoryPerSymbol != DefaultAuditMaxHistory {
		t.Errorf("Expected max history %d, got %d", DefaultAuditMaxHistory, svc.maxHistoryPerSymbol)
	}
}

func TestDecisionAuditService_RecordDecision(t *testing.T) {
	svc := NewDecisionAuditService()
	d := NewPositionDecision("BTCUSDT", 1)

	err := svc.RecordDecision(d)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify we can retrieve it
	retrieved, err := svc.GetDecision(d.ID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if retrieved.Symbol != d.Symbol {
		t.Errorf("Expected symbol %s, got %s", d.Symbol, retrieved.Symbol)
	}
}

func TestDecisionAuditService_RecordDecisionNil(t *testing.T) {
	svc := NewDecisionAuditService()
	err := svc.RecordDecision(nil)
	if err == nil {
		t.Error("Expected error for nil decision")
	}
}

func TestDecisionAuditService_RecordDecisionEmptyID(t *testing.T) {
	svc := NewDecisionAuditService()
	d := &PositionDecision{Symbol: "BTCUSDT", UserID: 1}
	err := svc.RecordDecision(d)
	if err == nil {
		t.Error("Expected error for empty ID")
	}
}

func TestDecisionAuditService_GetDecisionHistory(t *testing.T) {
	svc := NewDecisionAuditService()

	// Record multiple decisions for same symbol
	for i := 0; i < 5; i++ {
		d := NewPositionDecision("BTCUSDT", 1)
		d.Score = i * 10
		if err := svc.RecordDecision(d); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	// Get history
	history, err := svc.GetDecisionHistory("BTCUSDT", 10)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(history) != 5 {
		t.Errorf("Expected 5 decisions, got %d", len(history))
	}

	// Verify order (most recent first)
	for i := 0; i < len(history)-1; i++ {
		if history[i].Timestamp.Before(history[i+1].Timestamp) {
			t.Error("Expected decisions in reverse chronological order")
			break
		}
	}
}

func TestDecisionAuditService_GetUserDecisions(t *testing.T) {
	svc := NewDecisionAuditService()

	// Record decisions for different users
	for i := 0; i < 3; i++ {
		d := NewPositionDecision("BTCUSDT", 1)
		svc.RecordDecision(d)
	}
	for i := 0; i < 2; i++ {
		d := NewPositionDecision("ETHUSDT", 2)
		svc.RecordDecision(d)
	}

	// Get user 1 decisions
	user1Decisions, _ := svc.GetUserDecisions(1, 10)
	if len(user1Decisions) != 3 {
		t.Errorf("Expected 3 decisions for user 1, got %d", len(user1Decisions))
	}

	// Get user 2 decisions
	user2Decisions, _ := svc.GetUserDecisions(2, 10)
	if len(user2Decisions) != 2 {
		t.Errorf("Expected 2 decisions for user 2, got %d", len(user2Decisions))
	}
}

func TestDecisionAuditService_MarkExecuted(t *testing.T) {
	svc := NewDecisionAuditService()
	d := NewPositionDecision("BTCUSDT", 1)
	svc.RecordDecision(d)

	execTime := time.Now()
	err := svc.MarkExecuted(d.ID, "order-123", execTime)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify update
	retrieved, _ := svc.GetDecision(d.ID)
	if retrieved.OrderID != "order-123" {
		t.Errorf("Expected order ID order-123, got %s", retrieved.OrderID)
	}
	if retrieved.ExecutedAt == nil {
		t.Error("Expected ExecutedAt to be set")
	}
}

func TestDecisionAuditService_MarkError(t *testing.T) {
	svc := NewDecisionAuditService()
	d := NewPositionDecision("BTCUSDT", 1)
	svc.RecordDecision(d)

	err := svc.MarkError(d.ID, fmt.Errorf("test error"))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify update
	retrieved, _ := svc.GetDecision(d.ID)
	if retrieved.ExecutionError != "test error" {
		t.Errorf("Expected error 'test error', got %s", retrieved.ExecutionError)
	}
}

func TestDecisionAuditService_RollbackDecision(t *testing.T) {
	svc := NewDecisionAuditService()
	d := NewPositionDecision("BTCUSDT", 1)
	svc.RecordDecision(d)

	err := svc.RollbackDecision(d.ID, "market changed")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify update
	retrieved, _ := svc.GetDecision(d.ID)
	if !retrieved.RolledBack {
		t.Error("Expected RolledBack to be true")
	}
	if retrieved.RollbackReason != "market changed" {
		t.Errorf("Expected reason 'market changed', got %s", retrieved.RollbackReason)
	}
}

func TestDecisionAuditService_GetRecentDecisions(t *testing.T) {
	svc := NewDecisionAuditService()

	// Record decisions
	for i := 0; i < 10; i++ {
		d := NewPositionDecision("BTCUSDT", 1)
		svc.RecordDecision(d)
		time.Sleep(time.Millisecond) // Ensure different timestamps
	}

	// Get recent
	recent, _ := svc.GetRecentDecisions(5)
	if len(recent) != 5 {
		t.Errorf("Expected 5 recent decisions, got %d", len(recent))
	}
}

func TestDecisionAuditService_GetLastDecisionForSymbol(t *testing.T) {
	svc := NewDecisionAuditService()

	// Record multiple decisions
	var lastID string
	for i := 0; i < 3; i++ {
		d := NewPositionDecision("BTCUSDT", 1)
		d.Score = i * 10
		svc.RecordDecision(d)
		lastID = d.ID
		time.Sleep(time.Millisecond)
	}

	// Get last
	last, _ := svc.GetLastDecisionForSymbol("BTCUSDT")
	if last == nil {
		t.Fatal("Expected non-nil last decision")
	}
	if last.ID != lastID {
		t.Error("Expected most recent decision")
	}
}

func TestDecisionAuditService_GetDecisionChainCount(t *testing.T) {
	svc := NewDecisionAuditService()

	// Initially 0
	count := svc.GetDecisionChainCount("BTCUSDT")
	if count != 0 {
		t.Errorf("Expected chain count 0, got %d", count)
	}

	// After adding decisions
	for i := 0; i < 5; i++ {
		d := NewPositionDecision("BTCUSDT", 1)
		svc.RecordDecision(d)
	}

	count = svc.GetDecisionChainCount("BTCUSDT")
	if count != 5 {
		t.Errorf("Expected chain count 5, got %d", count)
	}
}

func TestDecisionAuditService_GetExecutedDecisions(t *testing.T) {
	svc := NewDecisionAuditService()

	// Record some executed and some not
	for i := 0; i < 5; i++ {
		d := NewPositionDecision("BTCUSDT", 1)
		svc.RecordDecision(d)
		if i%2 == 0 {
			svc.MarkExecuted(d.ID, "order-"+string(rune('0'+i)), time.Now())
		}
	}

	executed, _ := svc.GetExecutedDecisions(1, 10)
	if len(executed) != 3 { // 0, 2, 4
		t.Errorf("Expected 3 executed decisions, got %d", len(executed))
	}
}

func TestDecisionAuditService_GetBlockedDecisions(t *testing.T) {
	svc := NewDecisionAuditService()

	// Record some blocked and some not
	for i := 0; i < 5; i++ {
		var d *PositionDecision
		if i%2 == 0 {
			d = NewDecisionBuilder("BTCUSDT", 1).
				WithBlocking([]BlockingReason{{Description: "test"}}).
				Build()
		} else {
			d = NewDecisionBuilder("BTCUSDT", 1).
				WithScore(80, 0.7).
				Build()
		}
		svc.RecordDecision(d)
	}

	blocked, _ := svc.GetBlockedDecisions(1, 10)
	if len(blocked) != 3 { // 0, 2, 4
		t.Errorf("Expected 3 blocked decisions, got %d", len(blocked))
	}
}

func TestDecisionAuditService_ClearSymbolHistory(t *testing.T) {
	svc := NewDecisionAuditService()

	// Record for multiple symbols
	for i := 0; i < 3; i++ {
		d := NewPositionDecision("BTCUSDT", 1)
		svc.RecordDecision(d)
	}
	d := NewPositionDecision("ETHUSDT", 1)
	svc.RecordDecision(d)

	// Clear BTCUSDT history
	svc.ClearSymbolHistory("BTCUSDT")

	btcHistory, _ := svc.GetDecisionHistory("BTCUSDT", 10)
	if len(btcHistory) != 0 {
		t.Errorf("Expected 0 BTC decisions after clear, got %d", len(btcHistory))
	}

	ethHistory, _ := svc.GetDecisionHistory("ETHUSDT", 10)
	if len(ethHistory) != 1 {
		t.Errorf("Expected 1 ETH decision (unchanged), got %d", len(ethHistory))
	}
}

func TestDecisionAuditService_GetStats(t *testing.T) {
	svc := NewDecisionAuditService()

	// Record various decisions
	d1 := NewPositionDecision("BTCUSDT", 1)
	svc.RecordDecision(d1)
	svc.MarkExecuted(d1.ID, "order-1", time.Now())

	d2 := NewDecisionBuilder("ETHUSDT", 1).
		WithBlocking([]BlockingReason{{Description: "test"}}).
		Build()
	svc.RecordDecision(d2)

	d3 := NewPositionDecision("BTCUSDT", 2)
	svc.RecordDecision(d3)
	svc.MarkError(d3.ID, fmt.Errorf("test"))

	stats := svc.GetStats()
	if stats.TotalDecisions != 3 {
		t.Errorf("Expected 3 total decisions, got %d", stats.TotalDecisions)
	}
	if stats.UniqueSymbols != 2 {
		t.Errorf("Expected 2 unique symbols, got %d", stats.UniqueSymbols)
	}
	if stats.UniqueUsers != 2 {
		t.Errorf("Expected 2 unique users, got %d", stats.UniqueUsers)
	}
	if stats.ExecutedDecisions != 1 {
		t.Errorf("Expected 1 executed decision, got %d", stats.ExecutedDecisions)
	}
	if stats.BlockedDecisions != 1 {
		t.Errorf("Expected 1 blocked decision, got %d", stats.BlockedDecisions)
	}
	if stats.ErrorDecisions != 1 {
		t.Errorf("Expected 1 error decision, got %d", stats.ErrorDecisions)
	}
}

func TestDecisionAuditService_ClearAll(t *testing.T) {
	svc := NewDecisionAuditService()

	// Record decisions
	for i := 0; i < 5; i++ {
		d := NewPositionDecision("BTCUSDT", 1)
		svc.RecordDecision(d)
	}

	svc.ClearAll()

	stats := svc.GetStats()
	if stats.TotalDecisions != 0 {
		t.Errorf("Expected 0 decisions after ClearAll, got %d", stats.TotalDecisions)
	}
}
