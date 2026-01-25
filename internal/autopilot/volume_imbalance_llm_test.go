package autopilot

import (
	"context"
	"testing"
	"time"
)

// ============================================================================
// VOLUME IMBALANCE LLM VALIDATION TESTS - Story 11.47
// ============================================================================

// TestLLMValidationResultTypes verifies the validation result struct
func TestLLMValidationResultTypes(t *testing.T) {
	result := &LLMValidationResult{
		Approved:    true,
		Reason:      "Test approval",
		Skipped:     false,
		Timestamp:   time.Now(),
		RawResponse: "APPROVE: Test approval",
	}

	if !result.Approved {
		t.Error("Expected approved to be true")
	}
	if result.Skipped {
		t.Error("Expected skipped to be false")
	}
}

// TestParseApproveResponse tests parsing APPROVE responses
func TestParseApproveResponse(t *testing.T) {
	validator := NewVolumeImbalanceLLMValidator(nil, true)

	tests := []struct {
		name     string
		response string
		approved bool
	}{
		{"Standard approve", "APPROVE: Strong institutional buying detected", true},
		{"Approve with colon", "APPROVE: Volume spike confirms accumulation", true},
		{"Uppercase approve", "APPROVE: Pattern validated", true},
		{"Approve no colon", "APPROVE Good setup with clear breakout", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.ParseLLMResponse(tt.response)
			if err != nil {
				t.Fatalf("ParseLLMResponse failed: %v", err)
			}
			if result.Approved != tt.approved {
				t.Errorf("Expected approved=%v, got %v", tt.approved, result.Approved)
			}
		})
	}
}

// TestParseRejectResponse tests parsing REJECT responses
func TestParseRejectResponse(t *testing.T) {
	validator := NewVolumeImbalanceLLMValidator(nil, true)

	tests := []struct {
		name     string
		response string
		approved bool
	}{
		{"Standard reject", "REJECT: False breakout detected", false},
		{"Reject with colon", "REJECT: Insufficient volume confirmation", false},
		{"Uppercase reject", "REJECT: Pattern invalidated", false},
		{"Reject no colon", "REJECT Low probability setup", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.ParseLLMResponse(tt.response)
			if err != nil {
				t.Fatalf("ParseLLMResponse failed: %v", err)
			}
			if result.Approved != tt.approved {
				t.Errorf("Expected approved=%v, got %v", tt.approved, result.Approved)
			}
		})
	}
}

// TestParseImplicitApproval tests implicit approval detection
func TestParseImplicitApproval(t *testing.T) {
	validator := NewVolumeImbalanceLLMValidator(nil, true)

	tests := []struct {
		name     string
		response string
		approved bool
	}{
		{"High probability", "The setup shows HIGH PROBABILITY of success", true},
		{"Valid setup", "This is a VALID SETUP for entry", true},
		{"Confirmed pattern", "Pattern is CONFIRMED with strong volume", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.ParseLLMResponse(tt.response)
			if err != nil {
				t.Fatalf("ParseLLMResponse failed: %v", err)
			}
			if result.Approved != tt.approved {
				t.Errorf("Expected approved=%v, got %v", tt.approved, result.Approved)
			}
		})
	}
}

// TestParseImplicitRejection tests implicit rejection detection
func TestParseImplicitRejection(t *testing.T) {
	validator := NewVolumeImbalanceLLMValidator(nil, true)

	tests := []struct {
		name     string
		response string
		approved bool
	}{
		{"False breakout", "This appears to be a FALSE BREAKOUT", false},
		{"Red flag", "Multiple RED FLAGS in this setup", false},
		{"Not recommended", "Entry is NOT RECOMMENDED at this time", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.ParseLLMResponse(tt.response)
			if err != nil {
				t.Fatalf("ParseLLMResponse failed: %v", err)
			}
			if result.Approved != tt.approved {
				t.Errorf("Expected approved=%v, got %v", tt.approved, result.Approved)
			}
		})
	}
}

// TestParseAmbiguousResponse tests handling of ambiguous responses
func TestParseAmbiguousResponse(t *testing.T) {
	validator := NewVolumeImbalanceLLMValidator(nil, true)

	// A response without clear APPROVE/REJECT should return error
	_, err := validator.ParseLLMResponse("The pattern looks interesting but needs more data")
	if err == nil {
		t.Error("Expected error for ambiguous response, got nil")
	}
}

// TestValidatorDisabled tests that disabled validator returns skipped=true
func TestValidatorDisabled(t *testing.T) {
	validator := NewVolumeImbalanceLLMValidator(nil, false) // disabled

	pattern := &VolumeImbalancePattern{
		Symbol: "BTCUSDT",
		Mode:   GinieModeScalp,
	}

	result, err := validator.ValidatePatternWithLLM(context.Background(), pattern, nil)
	if err != nil {
		t.Fatalf("ValidatePatternWithLLM failed: %v", err)
	}

	if !result.Approved {
		t.Error("Expected approved=true when validation is disabled")
	}
	if !result.Skipped {
		t.Error("Expected skipped=true when validation is disabled")
	}
	if result.SkipReason == "" {
		t.Error("Expected skip reason when validation is disabled")
	}
}

// TestValidatorNoClient tests that missing LLM client returns skipped=true
func TestValidatorNoClient(t *testing.T) {
	validator := NewVolumeImbalanceLLMValidator(nil, true) // enabled but no client

	pattern := &VolumeImbalancePattern{
		Symbol: "BTCUSDT",
		Mode:   GinieModeScalp,
	}

	result, err := validator.ValidatePatternWithLLM(context.Background(), pattern, nil)
	if err != nil {
		t.Fatalf("ValidatePatternWithLLM failed: %v", err)
	}

	if !result.Approved {
		t.Error("Expected approved=true when LLM client is missing")
	}
	if !result.Skipped {
		t.Error("Expected skipped=true when LLM client is missing")
	}
}

// TestGenerateLLMPrompt tests prompt generation
func TestGenerateLLMPrompt(t *testing.T) {
	validator := NewVolumeImbalanceLLMValidator(nil, true)

	pattern := &VolumeImbalancePattern{
		Symbol: "BTCUSDT",
		Mode:   GinieModeScalp,
		State:  VIStateReady,
		ReferenceCandle: struct {
			Time           time.Time `json:"time"`
			High           float64   `json:"high"`
			Low            float64   `json:"low"`
			Close          float64   `json:"close"`
			Volume         float64   `json:"volume"`
			TakerBuyVolume float64   `json:"taker_buy_volume"`
			Index          int       `json:"index"`
		}{
			Time:   time.Now(),
			High:   100.0,
			Low:    98.0,
			Close:  99.5,
			Volume: 500000,
			TakerBuyVolume: 300000,
		},
		ConsolidationCandles:   3,
		ConsolidationLow:       98.5,
		ConsolidationHigh:      99.8,
		ConsolidationAvgVolume: 100000,
		VolumeTrend:            -0.15,
	}

	analysis := &VolumeImbalanceAnalysis{
		PatternDetected: true,
		Direction:       "LONG",
		SuggestedEntry:  100.5,
		SuggestedSL:     98.0,
		SuggestedTP:     110.0,
		RiskReward:      4.0,
		Confidence:      75.0,
	}

	systemPrompt, userPrompt := validator.GenerateLLMPrompt(pattern, analysis)

	// Verify system prompt contains key instructions
	if systemPrompt == "" {
		t.Error("Expected non-empty system prompt")
	}

	// Verify user prompt contains pattern data
	if userPrompt == "" {
		t.Error("Expected non-empty user prompt")
	}

	// Check key elements are in the prompt
	if !contains(userPrompt, "BTCUSDT") {
		t.Error("User prompt should contain symbol")
	}
	if !contains(userPrompt, "READY") {
		t.Error("User prompt should contain READY state")
	}
	if !contains(userPrompt, "LONG") {
		t.Error("User prompt should contain direction")
	}
}

// TestRejectionLogging tests rejection log storage
func TestRejectionLogging(t *testing.T) {
	validator := NewVolumeImbalanceLLMValidator(nil, true)

	pattern := &VolumeImbalancePattern{
		Symbol:     "BTCUSDT",
		Mode:       GinieModeScalp,
		DetectedAt: time.Now(),
	}

	analysis := &VolumeImbalanceAnalysis{
		PatternDetected: true,
	}

	result := &LLMValidationResult{
		Approved:    false,
		Reason:      "Test rejection reason",
		RawResponse: "REJECT: Test rejection reason",
	}

	// Call the private method through reflection or access the logs after
	validator.logRejection(pattern, analysis, result)

	logs := validator.GetRejectionLogs()
	if len(logs) != 1 {
		t.Fatalf("Expected 1 rejection log, got %d", len(logs))
	}

	if logs[0].Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %s", logs[0].Symbol)
	}
	if logs[0].RejectionReason != "Test rejection reason" {
		t.Errorf("Expected rejection reason, got %s", logs[0].RejectionReason)
	}
}

// TestRejectionStats tests rejection statistics
func TestRejectionStats(t *testing.T) {
	validator := NewVolumeImbalanceLLMValidator(nil, true)

	// Add some rejections
	for i := 0; i < 5; i++ {
		pattern := &VolumeImbalancePattern{
			Symbol:     "BTCUSDT",
			Mode:       GinieModeScalp,
			DetectedAt: time.Now(),
		}
		result := &LLMValidationResult{
			Approved: false,
			Reason:   "Test",
		}
		validator.logRejection(pattern, nil, result)
	}

	for i := 0; i < 3; i++ {
		pattern := &VolumeImbalancePattern{
			Symbol:     "ETHUSDT",
			Mode:       GinieModeScalp,
			DetectedAt: time.Now(),
		}
		result := &LLMValidationResult{
			Approved: false,
			Reason:   "Test",
		}
		validator.logRejection(pattern, nil, result)
	}

	stats := validator.GetRejectionStats()

	if stats["total_rejections"].(int) != 8 {
		t.Errorf("Expected 8 total rejections, got %v", stats["total_rejections"])
	}

	bySymbol := stats["by_symbol"].(map[string]int)
	if bySymbol["BTCUSDT"] != 5 {
		t.Errorf("Expected 5 BTCUSDT rejections, got %d", bySymbol["BTCUSDT"])
	}
	if bySymbol["ETHUSDT"] != 3 {
		t.Errorf("Expected 3 ETHUSDT rejections, got %d", bySymbol["ETHUSDT"])
	}
}

// TestClearRejectionLogs tests clearing rejection logs
func TestClearRejectionLogs(t *testing.T) {
	validator := NewVolumeImbalanceLLMValidator(nil, true)

	// Add a rejection
	pattern := &VolumeImbalancePattern{
		Symbol:     "BTCUSDT",
		DetectedAt: time.Now(),
	}
	result := &LLMValidationResult{Approved: false, Reason: "Test"}
	validator.logRejection(pattern, nil, result)

	if len(validator.GetRejectionLogs()) != 1 {
		t.Error("Expected 1 log before clear")
	}

	validator.ClearRejectionLogs()

	if len(validator.GetRejectionLogs()) != 0 {
		t.Error("Expected 0 logs after clear")
	}
}

// TestDetectorLLMValidatorIntegration tests detector-validator integration
func TestDetectorLLMValidatorIntegration(t *testing.T) {
	config := DefaultVolumeImbalanceConfig()
	detector := NewVolumeImbalanceDetector(nil, config, nil)

	// Initially no validator
	if detector.GetLLMValidator() != nil {
		t.Error("Expected nil validator initially")
	}

	// Set validator
	validator := NewVolumeImbalanceLLMValidator(nil, true)
	detector.SetLLMValidator(validator)

	if detector.GetLLMValidator() == nil {
		t.Error("Expected non-nil validator after set")
	}

	// Test IsLLMValidationEnabled (should be false because no LLM client)
	if detector.IsLLMValidationEnabled() {
		t.Error("Expected IsLLMValidationEnabled to be false without LLM client")
	}
}

// TestResetPatternOnRejection tests pattern reset after LLM rejection
func TestResetPatternOnRejection(t *testing.T) {
	config := DefaultVolumeImbalanceConfig()
	detector := NewVolumeImbalanceDetector(nil, config, nil)

	// Manually add a pattern
	pattern := &VolumeImbalancePattern{
		Symbol:               "BTCUSDT",
		Mode:                 GinieModeScalp,
		State:                VIStateReady,
		ConsolidationCandles: 5,
		ReferenceCandle: struct {
			Time           time.Time `json:"time"`
			High           float64   `json:"high"`
			Low            float64   `json:"low"`
			Close          float64   `json:"close"`
			Volume         float64   `json:"volume"`
			TakerBuyVolume float64   `json:"taker_buy_volume"`
			Index          int       `json:"index"`
		}{
			High:   100.0,
			Volume: 500000,
		},
	}
	detector.mu.Lock()
	detector.patterns["BTCUSDT"] = pattern
	detector.mu.Unlock()

	// Reset the pattern
	detector.ResetPatternOnRejection("BTCUSDT", "Test rejection")

	// Verify pattern was reset
	resetPattern := detector.GetPattern("BTCUSDT")
	if resetPattern == nil {
		t.Fatal("Expected pattern to exist after reset")
	}

	if resetPattern.State != VIStateWatching {
		t.Errorf("Expected state WATCHING, got %s", resetPattern.State)
	}

	if !contains(resetPattern.InvalidReason, "LLM rejected") {
		t.Error("Expected InvalidReason to contain 'LLM rejected'")
	}

	if resetPattern.ConsolidationCandles != 0 {
		t.Error("Expected ConsolidationCandles to be reset to 0")
	}
}

// TestValidateWithFallback tests the fallback behavior when LLM is unavailable
func TestValidateWithFallback(t *testing.T) {
	validator := NewVolumeImbalanceLLMValidator(nil, true) // enabled but no client

	pattern := &VolumeImbalancePattern{
		Symbol: "BTCUSDT",
		Mode:   GinieModeScalp,
	}

	tests := []struct {
		name       string
		confidence float64
		minScore   float64
		approved   bool
	}{
		{"High confidence meets minimum", 80.0, 70.0, true},
		{"Confidence exactly at minimum", 70.0, 70.0, true},
		{"Confidence below minimum", 60.0, 70.0, true}, // When skipped due to no client, still approved
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &VolumeImbalanceAnalysis{
				PatternDetected: true,
				Confidence:      tt.confidence,
			}

			result, err := validator.ValidateWithFallback(context.Background(), pattern, analysis, tt.minScore)
			if err != nil {
				t.Fatalf("ValidateWithFallback failed: %v", err)
			}

			if result.Approved != tt.approved {
				t.Errorf("Expected approved=%v, got %v (confidence=%.1f, min=%.1f)",
					tt.approved, result.Approved, tt.confidence, tt.minScore)
			}
		})
	}
}

// TestValidateWithFallbackNoAnalysis tests fallback with nil analysis
func TestValidateWithFallbackNoAnalysis(t *testing.T) {
	validator := NewVolumeImbalanceLLMValidator(nil, true) // enabled but no client

	pattern := &VolumeImbalancePattern{
		Symbol: "BTCUSDT",
		Mode:   GinieModeScalp,
	}

	// With nil analysis, should still return a result (approved when skipped)
	result, err := validator.ValidateWithFallback(context.Background(), pattern, nil, 70.0)
	if err != nil {
		t.Fatalf("ValidateWithFallback failed: %v", err)
	}

	// When LLM is skipped (no client), approved=true, skipped=true
	if !result.Approved {
		t.Error("Expected approved=true when LLM validation is skipped")
	}
	if !result.Skipped {
		t.Error("Expected skipped=true when no LLM client")
	}
}

// TestSetLLMClient tests the convenience method for setting LLM client
func TestSetLLMClient(t *testing.T) {
	config := DefaultVolumeImbalanceConfig()
	detector := NewVolumeImbalanceDetector(nil, config, nil)

	// Initially no validator
	if detector.GetLLMValidator() != nil {
		t.Error("Expected nil validator initially")
	}

	// Set nil client - should not create validator
	detector.SetLLMClient(nil)
	if detector.GetLLMValidator() != nil {
		t.Error("Expected nil validator after setting nil client")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
