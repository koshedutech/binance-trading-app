// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.6: Strategy Registry - Tests
package decision

import (
	"strings"
	"testing"
	"time"
)

// MockStrategy implements the Strategy interface for testing.
type MockStrategy struct {
	name              string
	supportedRegimes  []MarketRegime
	requiredIndicators []string
	entryConditions   []Condition
	exitConditions    []Condition
	scoreValue        float64
}

func NewMockStrategy(name string, regimes []MarketRegime) *MockStrategy {
	return &MockStrategy{
		name:              name,
		supportedRegimes:  regimes,
		requiredIndicators: []string{"adx", "rsi", "ema_21"},
		entryConditions: []Condition{
			*NewCondition("adx_min", "ADX must be above minimum", "adx").WithThreshold(OperatorGreaterOrEqual, 25.0),
			*NewCondition("rsi_range", "RSI must be in valid range", "rsi").WithRange(30.0, 70.0),
		},
		exitConditions: []Condition{
			*NewCondition("stop_loss", "Stop loss triggered", "price").WithThreshold(OperatorLessThan, 0.0),
		},
		scoreValue: 75.0,
	}
}

func (m *MockStrategy) Name() string {
	return m.name
}

func (m *MockStrategy) SupportedRegimes() []MarketRegime {
	return m.supportedRegimes
}

func (m *MockStrategy) RequiredIndicators() []string {
	return m.requiredIndicators
}

func (m *MockStrategy) CalculateScore(state *CoinState) StrategyScore {
	score := NewStrategyScore(m.name)
	score.TechnicalScore = m.scoreValue * 0.4
	score.ContextScore = m.scoreValue * 0.3
	score.LLMScore = m.scoreValue * 0.2
	score.HistoryScore = m.scoreValue * 0.1
	score.CalculateFinal()
	score.Confidence = m.scoreValue
	score.Recommendation = RecommendEnterLong
	score.EntryConditionsMet = 2
	score.EntryConditionsTotal = 2
	return *score
}

func (m *MockStrategy) GetEntryConditions() []Condition {
	return m.entryConditions
}

func (m *MockStrategy) GetExitConditions() []Condition {
	return m.exitConditions
}

func (m *MockStrategy) SetScore(score float64) {
	m.scoreValue = score
}

// TestConditionEvaluate tests the Condition.Evaluate method
func TestConditionEvaluate(t *testing.T) {
	tests := []struct {
		name     string
		operator ConditionOperator
		threshold float64
		minVal   float64
		maxVal   float64
		value    float64
		expected bool
	}{
		{"GreaterThan_Pass", OperatorGreaterThan, 25.0, 0, 0, 30.0, true},
		{"GreaterThan_Fail", OperatorGreaterThan, 25.0, 0, 0, 20.0, false},
		{"GreaterThan_Equal", OperatorGreaterThan, 25.0, 0, 0, 25.0, false},
		{"GreaterOrEqual_Pass", OperatorGreaterOrEqual, 25.0, 0, 0, 30.0, true},
		{"GreaterOrEqual_Equal", OperatorGreaterOrEqual, 25.0, 0, 0, 25.0, true},
		{"GreaterOrEqual_Fail", OperatorGreaterOrEqual, 25.0, 0, 0, 20.0, false},
		{"LessThan_Pass", OperatorLessThan, 25.0, 0, 0, 20.0, true},
		{"LessThan_Fail", OperatorLessThan, 25.0, 0, 0, 30.0, false},
		{"LessOrEqual_Pass", OperatorLessOrEqual, 25.0, 0, 0, 20.0, true},
		{"LessOrEqual_Equal", OperatorLessOrEqual, 25.0, 0, 0, 25.0, true},
		{"LessOrEqual_Fail", OperatorLessOrEqual, 25.0, 0, 0, 30.0, false},
		{"Equal_Pass", OperatorEqual, 25.0, 0, 0, 25.0, true},
		{"Equal_Fail", OperatorEqual, 25.0, 0, 0, 30.0, false},
		{"Equal_FloatPrecision", OperatorEqual, 0.1 + 0.2, 0, 0, 0.3, true}, // Float precision test
		{"NotEqual_Pass", OperatorNotEqual, 25.0, 0, 0, 30.0, true},
		{"NotEqual_Fail", OperatorNotEqual, 25.0, 0, 0, 25.0, false},
		{"NotEqual_FloatPrecision", OperatorNotEqual, 0.1 + 0.2, 0, 0, 0.3, false}, // Float precision test
		{"InRange_Pass", OperatorInRange, 0, 30.0, 70.0, 50.0, true},
		{"InRange_LowerBound", OperatorInRange, 0, 30.0, 70.0, 30.0, true},
		{"InRange_UpperBound", OperatorInRange, 0, 30.0, 70.0, 70.0, true},
		{"InRange_BelowMin", OperatorInRange, 0, 30.0, 70.0, 20.0, false},
		{"InRange_AboveMax", OperatorInRange, 0, 30.0, 70.0, 80.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			condition := &Condition{
				Operator:  tt.operator,
				Threshold: tt.threshold,
				MinValue:  tt.minVal,
				MaxValue:  tt.maxVal,
			}
			result := condition.Evaluate(tt.value)
			if result != tt.expected {
				t.Errorf("Evaluate(%f) = %v, expected %v", tt.value, result, tt.expected)
			}
		})
	}
}

// TestNewCondition tests condition creation and builder methods
func TestNewCondition(t *testing.T) {
	cond := NewCondition("test_condition", "Test description", "adx")

	if cond.Name != "test_condition" {
		t.Errorf("Name = %s, expected test_condition", cond.Name)
	}
	if cond.Description != "Test description" {
		t.Errorf("Description = %s, expected Test description", cond.Description)
	}
	if cond.Indicator != "adx" {
		t.Errorf("Indicator = %s, expected adx", cond.Indicator)
	}
	if !cond.Required {
		t.Error("Required should default to true")
	}
	if cond.Weight != 1.0 {
		t.Errorf("Weight = %f, expected 1.0", cond.Weight)
	}

	// Test builder methods
	cond.WithThreshold(OperatorGreaterThan, 25.0)
	if cond.Operator != OperatorGreaterThan {
		t.Errorf("Operator = %s, expected >", cond.Operator)
	}
	if cond.Threshold != 25.0 {
		t.Errorf("Threshold = %f, expected 25.0", cond.Threshold)
	}

	cond.WithRange(30.0, 70.0)
	if cond.Operator != OperatorInRange {
		t.Errorf("Operator = %s, expected IN_RANGE", cond.Operator)
	}
	if cond.MinValue != 30.0 || cond.MaxValue != 70.0 {
		t.Errorf("Range = [%f, %f], expected [30.0, 70.0]", cond.MinValue, cond.MaxValue)
	}

	cond.WithWeight(0.5)
	if cond.Weight != 0.5 {
		t.Errorf("Weight = %f, expected 0.5", cond.Weight)
	}

	cond.WithRequired(false)
	if cond.Required {
		t.Error("Required should be false after WithRequired(false)")
	}

	cond.WithBlockCategory(CategoryHardBlock)
	if cond.BlockCategory != CategoryHardBlock {
		t.Errorf("BlockCategory = %s, expected HARD_BLOCK", cond.BlockCategory)
	}
}

// TestNewConditionWeightClamping tests that weight is clamped to valid range
func TestNewConditionWeightClamping(t *testing.T) {
	cond := NewCondition("test", "desc", "ind")

	cond.WithWeight(-0.5)
	if cond.Weight != 0 {
		t.Errorf("Negative weight should clamp to 0, got %f", cond.Weight)
	}

	cond.WithWeight(1.5)
	if cond.Weight != 1 {
		t.Errorf("Weight > 1 should clamp to 1, got %f", cond.Weight)
	}
}

// TestStrategyScore tests the StrategyScore methods
func TestStrategyScore(t *testing.T) {
	score := NewStrategyScore("test_strategy")

	if score.StrategyName != "test_strategy" {
		t.Errorf("StrategyName = %s, expected test_strategy", score.StrategyName)
	}
	if score.Recommendation != RecommendHold {
		t.Errorf("Recommendation = %s, expected HOLD", score.Recommendation)
	}
	if score.Timestamp == 0 {
		t.Error("Timestamp should be set")
	}

	// Test score calculation
	score.TechnicalScore = 30.0
	score.ContextScore = 22.0
	score.LLMScore = 15.0
	score.HistoryScore = 8.0
	score.CalculateFinal()

	expectedFinal := 75.0
	if score.FinalScore != expectedFinal {
		t.Errorf("FinalScore = %f, expected %f", score.FinalScore, expectedFinal)
	}

	// Test threshold checking
	if !score.PassesThreshold(70.0) {
		t.Error("Score 75 should pass threshold 70")
	}
	if score.PassesThreshold(80.0) {
		t.Error("Score 75 should not pass threshold 80")
	}

	// Test blocking
	if score.IsBlocked() {
		t.Error("Score should not be blocked initially")
	}

	score.AddBlockingReason(BlockingReason{Category: CategorySoftBlock})
	if score.IsBlocked() {
		t.Error("Soft block should not mark as blocked")
	}

	score.AddBlockingReason(BlockingReason{Category: CategoryHardBlock})
	if !score.IsBlocked() {
		t.Error("Hard block should mark as blocked")
	}

	if score.PassesThreshold(70.0) {
		t.Error("Blocked score should not pass threshold")
	}

	// Test failed conditions
	score.AddFailedCondition("adx_min")
	if len(score.FailedConditions) != 1 || score.FailedConditions[0] != "adx_min" {
		t.Error("Failed condition not recorded correctly")
	}

	// Test component details
	score.SetComponentDetail("trend_alignment", 12.5)
	if val, ok := score.ComponentDetails["trend_alignment"]; !ok || val != 12.5 {
		t.Error("Component detail not set correctly")
	}
}

// TestStrategyScoreClamping tests that final score is clamped to 0-100
func TestStrategyScoreClamping(t *testing.T) {
	score := NewStrategyScore("test")

	// Test overflow clamping
	score.TechnicalScore = 50.0
	score.ContextScore = 40.0
	score.LLMScore = 30.0
	score.HistoryScore = 20.0
	score.CalculateFinal()

	if score.FinalScore > 100.0 {
		t.Errorf("FinalScore %f should be clamped to 100", score.FinalScore)
	}

	// Test negative clamping
	score2 := NewStrategyScore("test2")
	score2.TechnicalScore = -10.0
	score2.CalculateFinal()

	// Since other components are 0, we may get negative, but it should clamp
	if score2.FinalScore < 0 {
		t.Errorf("FinalScore %f should be clamped to 0", score2.FinalScore)
	}
}

// TestStrategyScoreString tests the String method
func TestStrategyScoreString(t *testing.T) {
	score := NewStrategyScore("trend_following")
	score.TechnicalScore = 30.0
	score.ContextScore = 20.0
	score.LLMScore = 15.0
	score.HistoryScore = 10.0
	score.CalculateFinal()
	score.Confidence = 75.0
	score.Recommendation = RecommendEnterLong
	score.EntryConditionsMet = 3
	score.EntryConditionsTotal = 4

	str := score.String()
	if str == "" {
		t.Error("String() should return non-empty string")
	}

	// Check that key components are in the string
	expected := []string{"trend_following", "75.0", "ENTER_LONG", "3/4"}
	for _, exp := range expected {
		if !testContainsSubstring(str, exp) {
			t.Errorf("String() should contain %s, got: %s", exp, str)
		}
	}
}

// testContainsSubstring is a test helper to check if s contains substr
func testContainsSubstring(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestStrategyInfo tests the StrategyInfo creation
func TestStrategyInfo(t *testing.T) {
	strategy := NewMockStrategy("test_strategy", []MarketRegime{RegimeTrending, RegimeVolatile})
	info := NewStrategyInfo(strategy)

	if info.Name != "test_strategy" {
		t.Errorf("Name = %s, expected test_strategy", info.Name)
	}
	if len(info.SupportedRegimes) != 2 {
		t.Errorf("SupportedRegimes length = %d, expected 2", len(info.SupportedRegimes))
	}
	if len(info.RequiredIndicators) != 3 {
		t.Errorf("RequiredIndicators length = %d, expected 3", len(info.RequiredIndicators))
	}
	if info.EntryConditionCount != 2 {
		t.Errorf("EntryConditionCount = %d, expected 2", info.EntryConditionCount)
	}
	if info.ExitConditionCount != 1 {
		t.Errorf("ExitConditionCount = %d, expected 1", info.ExitConditionCount)
	}
	if !info.Enabled {
		t.Error("Enabled should default to true")
	}
	if info.RegisteredAt == 0 {
		t.Error("RegisteredAt should be set")
	}
}

// TestMockStrategy tests that the mock strategy works correctly
func TestMockStrategy(t *testing.T) {
	strategy := NewMockStrategy("trend_following", []MarketRegime{RegimeTrending})

	if strategy.Name() != "trend_following" {
		t.Errorf("Name() = %s, expected trend_following", strategy.Name())
	}

	regimes := strategy.SupportedRegimes()
	if len(regimes) != 1 || regimes[0] != RegimeTrending {
		t.Error("SupportedRegimes() not returning expected regimes")
	}

	indicators := strategy.RequiredIndicators()
	if len(indicators) != 3 {
		t.Errorf("RequiredIndicators() length = %d, expected 3", len(indicators))
	}

	state := NewCoinState("BTCUSDT")
	score := strategy.CalculateScore(state)
	if score.FinalScore != 75.0 {
		t.Errorf("FinalScore = %f, expected 75.0", score.FinalScore)
	}

	entryConditions := strategy.GetEntryConditions()
	if len(entryConditions) != 2 {
		t.Errorf("GetEntryConditions() length = %d, expected 2", len(entryConditions))
	}

	exitConditions := strategy.GetExitConditions()
	if len(exitConditions) != 1 {
		t.Errorf("GetExitConditions() length = %d, expected 1", len(exitConditions))
	}
}

// TestStrategyRecommendations tests recommendation constants
func TestStrategyRecommendations(t *testing.T) {
	recommendations := []StrategyRecommendation{
		RecommendEnterLong,
		RecommendEnterShort,
		RecommendHold,
		RecommendExit,
		RecommendAvoid,
	}

	for _, rec := range recommendations {
		if rec == "" {
			t.Error("Recommendation should not be empty")
		}
	}
}

// TestConditionTypes tests condition type constants
func TestConditionTypes(t *testing.T) {
	types := []ConditionType{
		ConditionTypeThreshold,
		ConditionTypeComparison,
		ConditionTypeBoolean,
		ConditionTypeRange,
	}

	for _, ct := range types {
		if ct == "" {
			t.Error("ConditionType should not be empty")
		}
	}
}

// TestConditionOperators tests condition operator constants
func TestConditionOperators(t *testing.T) {
	operators := []ConditionOperator{
		OperatorGreaterThan,
		OperatorGreaterOrEqual,
		OperatorLessThan,
		OperatorLessOrEqual,
		OperatorEqual,
		OperatorNotEqual,
		OperatorInRange,
	}

	for _, op := range operators {
		if op == "" {
			t.Error("ConditionOperator should not be empty")
		}
	}
}

// Benchmark for condition evaluation
func BenchmarkConditionEvaluate(b *testing.B) {
	condition := &Condition{
		Operator:  OperatorInRange,
		MinValue:  30.0,
		MaxValue:  70.0,
	}

	for i := 0; i < b.N; i++ {
		condition.Evaluate(50.0)
	}
}

// Benchmark for score calculation
func BenchmarkStrategyScoreCalculateFinal(b *testing.B) {
	score := NewStrategyScore("benchmark")
	score.TechnicalScore = 30.0
	score.ContextScore = 22.0
	score.LLMScore = 15.0
	score.HistoryScore = 8.0

	for i := 0; i < b.N; i++ {
		score.CalculateFinal()
	}
}

// Test that timestamps are recent
func TestTimestampsAreRecent(t *testing.T) {
	now := time.Now().UnixMilli()

	score := NewStrategyScore("test")
	if score.Timestamp < now-1000 || score.Timestamp > now+1000 {
		t.Errorf("Score timestamp %d not within 1 second of now %d", score.Timestamp, now)
	}

	strategy := NewMockStrategy("test", []MarketRegime{RegimeTrending})
	info := NewStrategyInfo(strategy)
	if info.RegisteredAt < now-1000 || info.RegisteredAt > now+1000 {
		t.Errorf("Info RegisteredAt %d not within 1 second of now %d", info.RegisteredAt, now)
	}
}
