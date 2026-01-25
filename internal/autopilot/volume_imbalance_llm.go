package autopilot

import (
	"binance-trading-bot/internal/ai/llm"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// VOLUME IMBALANCE LLM VALIDATION - Story 11.47
// ============================================================================
//
// This module adds LLM validation to filter volume imbalance pattern signals
// before entry. When a pattern reaches READY state, the LLM validates whether
// the setup is favorable for entry.
//
// Flow:
// 1. Pattern detection reaches READY state (Step 3: Breakout Entry)
// 2. GenerateLLMPrompt creates context with pattern data and trade setup
// 3. ValidatePatternWithLLM sends to LLM for approval
// 4. ParseLLMResponse extracts APPROVE/REJECT decision
// 5. Rejections are logged for analytics
//
// Configuration:
// - LLMValidation setting in VolumeImbalanceConfig controls enable/disable
// - 30-second timeout for LLM calls
// - Fallback: If LLM fails or times out, pattern is APPROVED by default
//
// ============================================================================
// INTEGRATION POINT DOCUMENTATION (for ginie_autopilot.go)
// ============================================================================
//
// The LLM validation should be called AFTER the pattern reaches READY state
// and BEFORE executing entry. The pattern already has Entry/SL/TP calculated
// at this point.
//
// Integration location in ginie_autopilot.go:
// - When processing volume imbalance signals from AnalyzeForVolumeImbalance()
// - After analysis.PatternDetected == true && analysis.PatternState == "READY"
// - Before calling the entry execution logic
//
// Example integration code (DO NOT modify ginie_autopilot.go yet):
//
//   // In the signal processing function, after getting analysis result:
//   if analysis.PatternDetected && analysis.PatternState == "READY" {
//       // Check if LLM validation is enabled
//       if detector.IsLLMValidationEnabled() {
//           validator := detector.GetLLMValidator()
//           result, err := validator.ValidatePatternWithLLM(ctx, analysis.Pattern, analysis)
//           if err != nil {
//               // Log error but continue (fail-open behavior)
//               logger.Warn("LLM validation error, proceeding with entry", "error", err)
//           } else if !result.Approved && !result.Skipped {
//               // Pattern rejected by LLM - reset and skip entry
//               detector.ResetPatternOnRejection(symbol, result.Reason)
//               logger.Info("Volume imbalance pattern rejected by LLM",
//                   "symbol", symbol,
//                   "reason", result.Reason)
//               return // Skip entry
//           }
//       }
//       // Proceed with entry execution...
//   }
//
// Initialization (in futures_controller.go or user_autopilot_manager.go):
//
//   // Create LLM validator with config
//   llmValidator := NewVolumeImbalanceLLMValidator(llmClient, config.LLMValidation)
//   llmValidator.SetLogger(logger)
//   detector.SetLLMValidator(llmValidator)
//
// ============================================================================

// LLMValidationResult holds the validation outcome
type LLMValidationResult struct {
	Approved    bool      `json:"approved"`
	Reason      string    `json:"reason"`
	Skipped     bool      `json:"skipped"`     // True if LLM validation was disabled or failed
	SkipReason  string    `json:"skip_reason"` // Why validation was skipped
	Timestamp   time.Time `json:"timestamp"`
	RawResponse string    `json:"raw_response,omitempty"`
}

// LLMRejectionLog stores rejection data for analytics
type LLMRejectionLog struct {
	Symbol          string                  `json:"symbol"`
	PatternID       string                  `json:"pattern_id"`      // Unique identifier for the pattern
	RejectionReason string                  `json:"rejection_reason"`
	RawLLMResponse  string                  `json:"raw_llm_response"`
	PatternSnapshot *VolumeImbalancePattern `json:"pattern_snapshot"`
	AnalysisSnapshot *VolumeImbalanceAnalysis `json:"analysis_snapshot,omitempty"`
	Timestamp       time.Time               `json:"timestamp"`
}

// VolumeImbalanceLLMValidator handles LLM validation for volume imbalance patterns
type VolumeImbalanceLLMValidator struct {
	llmClient     *llm.Client
	logger        interface{ Info(string, ...interface{}) }
	enabled       bool
	timeout       time.Duration
	mu            sync.RWMutex // Protects enabled field

	// Rejection log storage for analytics
	rejectionLogs []LLMRejectionLog
	maxLogs       int
	logMu         sync.RWMutex
}

// NewVolumeImbalanceLLMValidator creates a new LLM validator for volume imbalance patterns
func NewVolumeImbalanceLLMValidator(client *llm.Client, enabled bool) *VolumeImbalanceLLMValidator {
	return &VolumeImbalanceLLMValidator{
		llmClient:     client,
		enabled:       enabled,
		timeout:       30 * time.Second,
		rejectionLogs: make([]LLMRejectionLog, 0, 100),
		maxLogs:       100,
	}
}

// SetLogger sets the logger for the validator
func (v *VolumeImbalanceLLMValidator) SetLogger(logger interface{ Info(string, ...interface{}) }) {
	v.logger = logger
}

// SetEnabled enables or disables LLM validation
func (v *VolumeImbalanceLLMValidator) SetEnabled(enabled bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.enabled = enabled
}

// IsEnabled returns whether LLM validation is enabled
func (v *VolumeImbalanceLLMValidator) IsEnabled() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.enabled && v.llmClient != nil && v.llmClient.IsConfigured()
}

// GenerateLLMPrompt creates the validation prompt for the LLM
func (v *VolumeImbalanceLLMValidator) GenerateLLMPrompt(pattern *VolumeImbalancePattern, analysis *VolumeImbalanceAnalysis) (systemPrompt string, userPrompt string) {
	systemPrompt = `You are an expert cryptocurrency trader specializing in volume imbalance breakout patterns.
Your task is to validate a detected pattern and determine if it's a high-probability entry.

IMPORTANT: You must respond with EXACTLY one of these two formats:
- APPROVE: [reason]
- REJECT: [reason]

Be concise but specific in your reasoning. Focus on:
1. Quality of the volume spike (institutional buying)
2. Consolidation pattern quality (volume declining, price sideways)
3. Breakout confirmation (volume surge, price action)
4. Risk/reward ratio validity
5. Any red flags (false breakout signs, divergences)`

	// Build pattern step status
	stepStatus := "UNKNOWN"
	switch pattern.State {
	case VIStateWatching:
		stepStatus = "Step 1: Watching for accumulation start"
	case VIStateConsolidating:
		stepStatus = "Step 2: Consolidation in progress"
	case VIStateReady:
		stepStatus = "Step 3: BREAKOUT READY - Awaiting validation"
	}

	// Build consolidation info
	consolidationInfo := fmt.Sprintf(`- Candles in consolidation: %d
- Consolidation Low: %.8f
- Consolidation High: %.8f
- Average Volume during consolidation: %.2f
- Volume Trend (should be negative): %.4f`,
		pattern.ConsolidationCandles,
		pattern.ConsolidationLow,
		pattern.ConsolidationHigh,
		pattern.ConsolidationAvgVolume,
		pattern.VolumeTrend)

	// Build trade setup info
	tradeSetup := "N/A (not ready)"
	if analysis != nil && analysis.PatternDetected {
		tradeSetup = fmt.Sprintf(`- Direction: %s
- Entry Price: %.8f
- Stop Loss: %.8f (below consolidation low)
- Take Profit: %.8f (1:%.1f R:R)
- Risk Amount: %.2f%%
- Reward Amount: %.2f%%
- Confidence: %.1f%%`,
			analysis.Direction,
			analysis.SuggestedEntry,
			analysis.SuggestedSL,
			analysis.SuggestedTP,
			analysis.RiskReward,
			((analysis.SuggestedEntry-analysis.SuggestedSL)/analysis.SuggestedEntry)*100,
			((analysis.SuggestedTP-analysis.SuggestedEntry)/analysis.SuggestedEntry)*100,
			analysis.Confidence)
	}

	userPrompt = fmt.Sprintf(`VOLUME IMBALANCE PATTERN VALIDATION REQUEST

== PATTERN DATA ==
Symbol: %s
Mode: %s
Timeframe: Based on mode (scalp=15m, swing=1h, position=4h)
Current State: %s

== STEP 1: ACCUMULATION START (Reference Candle) ==
- Time: %s
- High (Entry Trigger): %.8f
- Low: %.8f
- Close: %.8f
- Volume (should be 2x+ average): %.2f
- Taker Buy Volume: %.2f

== STEP 2: CONSOLIDATION ==
%s

== STEP 3: BREAKOUT ==
- Breakout Time: %s
- Breakout High: %.8f
- Breakout Volume: %.2f (should be 50%%+ above consolidation avg)

== PROPOSED TRADE SETUP ==
%s

== VALIDATION REQUEST ==
Based on the Ravindra Volume Imbalance 3-step pattern above:
1. Is the accumulation start (Step 1) showing clear institutional buying?
2. Is the consolidation (Step 2) quality good (declining volume, sideways price)?
3. Is the breakout (Step 3) confirmed with volume surge?
4. Is the risk/reward ratio acceptable for this setup?
5. Are there any red flags suggesting a false breakout?

Respond with APPROVE or REJECT and your reasoning.`,
		pattern.Symbol,
		pattern.Mode,
		stepStatus,
		pattern.ReferenceCandle.Time.Format("2006-01-02 15:04:05"),
		pattern.ReferenceCandle.High,
		pattern.ReferenceCandle.Low,
		pattern.ReferenceCandle.Close,
		pattern.ReferenceCandle.Volume,
		pattern.ReferenceCandle.TakerBuyVolume,
		consolidationInfo,
		pattern.BreakoutCandle.Time.Format("2006-01-02 15:04:05"),
		pattern.BreakoutCandle.High,
		pattern.BreakoutCandle.Volume,
		tradeSetup)

	return systemPrompt, userPrompt
}

// ValidatePatternWithLLM sends the pattern to LLM for validation
func (v *VolumeImbalanceLLMValidator) ValidatePatternWithLLM(ctx context.Context, pattern *VolumeImbalancePattern, analysis *VolumeImbalanceAnalysis) (*LLMValidationResult, error) {
	// If validation is disabled, return approved with skip flag
	if !v.IsEnabled() {
		return &LLMValidationResult{
			Approved:   true,
			Skipped:    true,
			SkipReason: "LLM validation disabled in settings",
			Timestamp:  time.Now(),
		}, nil
	}

	// If LLM client is not configured, return approved with skip flag
	if v.llmClient == nil || !v.llmClient.IsConfigured() {
		if v.logger != nil {
			v.logger.Info("[VOLUME_IMBALANCE_LLM] LLM client not configured, skipping validation",
				"symbol", pattern.Symbol)
		}
		return &LLMValidationResult{
			Approved:   true,
			Skipped:    true,
			SkipReason: "LLM client not configured",
			Timestamp:  time.Now(),
		}, nil
	}

	// Generate prompt
	systemPrompt, userPrompt := v.GenerateLLMPrompt(pattern, analysis)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	// Call LLM
	responseChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		// Check if context is already cancelled before making call
		select {
		case <-ctx.Done():
			return
		default:
		}

		response, err := v.llmClient.Complete(systemPrompt, userPrompt)
		if err != nil {
			// Try to send error, but respect context cancellation
			select {
			case errChan <- err:
			case <-ctx.Done():
			}
			return
		}

		// Try to send response, but respect context cancellation
		select {
		case responseChan <- response:
		case <-ctx.Done():
		}
	}()

	// Wait for response or timeout
	select {
	case <-ctx.Done():
		if v.logger != nil {
			v.logger.Info("[VOLUME_IMBALANCE_LLM] LLM validation timed out, proceeding without validation",
				"symbol", pattern.Symbol,
				"timeout", v.timeout.String())
		}
		return &LLMValidationResult{
			Approved:   true,
			Skipped:    true,
			SkipReason: fmt.Sprintf("LLM request timed out after %s", v.timeout.String()),
			Timestamp:  time.Now(),
		}, nil

	case err := <-errChan:
		if v.logger != nil {
			v.logger.Info("[VOLUME_IMBALANCE_LLM] LLM validation failed, proceeding without validation",
				"symbol", pattern.Symbol,
				"error", err.Error())
		}
		return &LLMValidationResult{
			Approved:   true,
			Skipped:    true,
			SkipReason: fmt.Sprintf("LLM request failed: %s", err.Error()),
			Timestamp:  time.Now(),
		}, nil

	case response := <-responseChan:
		// Parse the response
		result, err := v.ParseLLMResponse(response)
		if err != nil {
			if v.logger != nil {
				v.logger.Info("[VOLUME_IMBALANCE_LLM] Failed to parse LLM response, proceeding without validation",
					"symbol", pattern.Symbol,
					"error", err.Error(),
					"response", truncateString(response, 200))
			}
			return &LLMValidationResult{
				Approved:    true,
				Skipped:     true,
				SkipReason:  fmt.Sprintf("Failed to parse LLM response: %s", err.Error()),
				RawResponse: response,
				Timestamp:   time.Now(),
			}, nil
		}

		result.RawResponse = response
		result.Timestamp = time.Now()

		// Log the result
		if v.logger != nil {
			status := "APPROVED"
			if !result.Approved {
				status = "REJECTED"
			}
			v.logger.Info("[VOLUME_IMBALANCE_LLM] Pattern validation complete",
				"symbol", pattern.Symbol,
				"status", status,
				"reason", result.Reason)
		}

		// If rejected, log for analytics
		if !result.Approved {
			v.logRejection(pattern, analysis, result)
		}

		return result, nil
	}
}

// ParseLLMResponse extracts APPROVE/REJECT from the LLM response
func (v *VolumeImbalanceLLMValidator) ParseLLMResponse(response string) (*LLMValidationResult, error) {
	response = strings.TrimSpace(response)
	upperResponse := strings.ToUpper(response)

	// Try to match APPROVE: [reason] or REJECT: [reason] patterns
	approvePattern := regexp.MustCompile(`(?i)^APPROVE[:\s]+(.*)`)
	rejectPattern := regexp.MustCompile(`(?i)^REJECT[:\s]+(.*)`)

	// Check for APPROVE first
	if matches := approvePattern.FindStringSubmatch(response); len(matches) > 1 {
		return &LLMValidationResult{
			Approved: true,
			Reason:   strings.TrimSpace(matches[1]),
		}, nil
	}

	// Check for REJECT
	if matches := rejectPattern.FindStringSubmatch(response); len(matches) > 1 {
		return &LLMValidationResult{
			Approved: false,
			Reason:   strings.TrimSpace(matches[1]),
		}, nil
	}

	// Fallback: Check if response contains APPROVE or REJECT keywords
	if strings.Contains(upperResponse, "APPROVE") && !strings.Contains(upperResponse, "REJECT") {
		// Extract reason after APPROVE
		idx := strings.Index(upperResponse, "APPROVE")
		reason := strings.TrimSpace(response[idx+7:])
		if len(reason) > 0 && reason[0] == ':' {
			reason = strings.TrimSpace(reason[1:])
		}
		return &LLMValidationResult{
			Approved: true,
			Reason:   reason,
		}, nil
	}

	if strings.Contains(upperResponse, "REJECT") {
		// Extract reason after REJECT
		idx := strings.Index(upperResponse, "REJECT")
		reason := strings.TrimSpace(response[idx+6:])
		if len(reason) > 0 && reason[0] == ':' {
			reason = strings.TrimSpace(reason[1:])
		}
		return &LLMValidationResult{
			Approved: false,
			Reason:   reason,
		}, nil
	}

	// If neither found, try to infer from context
	// This is a fallback for malformed responses
	approvalKeywords := []string{"HIGH PROBABILITY", "VALID SETUP", "FAVORABLE", "CONFIRMED"}
	for _, keyword := range approvalKeywords {
		if strings.Contains(upperResponse, keyword) {
			// Log when implicit approval is used
			if v.logger != nil {
				v.logger.Info("[VOLUME_IMBALANCE_LLM] Using implicit approval inference",
					"keyword_matched", keyword)
			}
			return &LLMValidationResult{
				Approved: true,
				Reason:   "Implicit approval inferred from response",
			}, nil
		}
	}

	rejectionKeywords := []string{"FALSE BREAKOUT", "LOW PROBABILITY", "RED FLAG", "INVALID", "NOT RECOMMENDED"}
	for _, keyword := range rejectionKeywords {
		if strings.Contains(upperResponse, keyword) {
			// Log when implicit rejection is used
			if v.logger != nil {
				v.logger.Info("[VOLUME_IMBALANCE_LLM] Using implicit rejection inference",
					"keyword_matched", keyword)
			}
			return &LLMValidationResult{
				Approved: false,
				Reason:   "Implicit rejection inferred from response",
			}, nil
		}
	}

	// Cannot determine - return error
	return nil, fmt.Errorf("unable to parse LLM response: no APPROVE or REJECT keyword found")
}

// logRejection stores a rejection for analytics
func (v *VolumeImbalanceLLMValidator) logRejection(pattern *VolumeImbalancePattern, analysis *VolumeImbalanceAnalysis, result *LLMValidationResult) {
	v.logMu.Lock()
	defer v.logMu.Unlock()

	// Create pattern ID
	patternID := fmt.Sprintf("%s_%s_%d", pattern.Symbol, pattern.Mode, pattern.DetectedAt.Unix())

	// Deep copy pattern for snapshot
	patternCopy := *pattern

	log := LLMRejectionLog{
		Symbol:          pattern.Symbol,
		PatternID:       patternID,
		RejectionReason: result.Reason,
		RawLLMResponse:  result.RawResponse,
		PatternSnapshot: &patternCopy,
		Timestamp:       time.Now(),
	}

	// Include analysis snapshot if available
	if analysis != nil {
		analysisCopy := *analysis
		analysisCopy.Pattern = nil // Avoid circular reference
		analysisCopy.TrailingStop = nil
		log.AnalysisSnapshot = &analysisCopy
	}

	// Add to logs
	v.rejectionLogs = append(v.rejectionLogs, log)

	// Trim if over max
	if len(v.rejectionLogs) > v.maxLogs {
		v.rejectionLogs = v.rejectionLogs[len(v.rejectionLogs)-v.maxLogs:]
	}
}

// GetRejectionLogs returns the rejection logs for analytics
func (v *VolumeImbalanceLLMValidator) GetRejectionLogs() []LLMRejectionLog {
	v.logMu.RLock()
	defer v.logMu.RUnlock()

	// Return a copy
	logs := make([]LLMRejectionLog, len(v.rejectionLogs))
	copy(logs, v.rejectionLogs)
	return logs
}

// GetRejectionLogsJSON returns the rejection logs as JSON for API exposure
func (v *VolumeImbalanceLLMValidator) GetRejectionLogsJSON() (string, error) {
	logs := v.GetRejectionLogs()
	data, err := json.MarshalIndent(logs, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ClearRejectionLogs clears the rejection logs
func (v *VolumeImbalanceLLMValidator) ClearRejectionLogs() {
	v.logMu.Lock()
	defer v.logMu.Unlock()
	v.rejectionLogs = v.rejectionLogs[:0]
}

// GetRejectionStats returns statistics about rejections
func (v *VolumeImbalanceLLMValidator) GetRejectionStats() map[string]interface{} {
	v.logMu.RLock()
	defer v.logMu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_rejections"] = len(v.rejectionLogs)

	// Count by symbol
	bySymbol := make(map[string]int)
	for _, log := range v.rejectionLogs {
		bySymbol[log.Symbol]++
	}
	stats["by_symbol"] = bySymbol

	// Recent rejections (last 24 hours)
	recent := 0
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, log := range v.rejectionLogs {
		if log.Timestamp.After(cutoff) {
			recent++
		}
	}
	stats["last_24h"] = recent

	return stats
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ValidateWithFallback validates a pattern with LLM, with fallback to confidence score.
// If LLM validation fails (error, timeout) and pattern confidence >= minScore, approves anyway.
// This provides graceful degradation when LLM is unavailable.
//
// Parameters:
//   - ctx: Context for cancellation
//   - pattern: The volume imbalance pattern to validate
//   - analysis: The analysis result containing confidence score
//   - minScore: Minimum confidence score (0-100) to approve on LLM failure
//
// Returns:
//   - *LLMValidationResult: The validation result
//   - error: Only returned for critical errors, not LLM failures
func (v *VolumeImbalanceLLMValidator) ValidateWithFallback(ctx context.Context, pattern *VolumeImbalancePattern, analysis *VolumeImbalanceAnalysis, minScore float64) (*LLMValidationResult, error) {
	// Try LLM validation first
	result, err := v.ValidatePatternWithLLM(ctx, pattern, analysis)

	// If validation succeeded (even if skipped), return as-is
	if err == nil && result != nil {
		// Special case: If LLM was skipped but confidence meets minimum, mark as approved
		if result.Skipped && analysis != nil && analysis.Confidence >= minScore {
			result.Approved = true
			result.Reason = fmt.Sprintf("LLM skipped (%s), approved by confidence score (%.1f%% >= %.1f%%)",
				result.SkipReason, analysis.Confidence, minScore)
			if v.logger != nil {
				v.logger.Info("[VOLUME_IMBALANCE_LLM] Pattern approved via confidence fallback",
					"symbol", pattern.Symbol,
					"confidence", analysis.Confidence,
					"min_score", minScore,
					"skip_reason", result.SkipReason)
			}
		}
		return result, nil
	}

	// LLM failed with error - check confidence fallback
	if analysis != nil && analysis.Confidence >= minScore {
		if v.logger != nil {
			v.logger.Info("[VOLUME_IMBALANCE_LLM] LLM error, approved via confidence fallback",
				"symbol", pattern.Symbol,
				"confidence", analysis.Confidence,
				"min_score", minScore,
				"error", err.Error())
		}
		return &LLMValidationResult{
			Approved:   true,
			Reason:     fmt.Sprintf("LLM error, approved by confidence score (%.1f%% >= %.1f%%)", analysis.Confidence, minScore),
			Skipped:    true,
			SkipReason: fmt.Sprintf("LLM error: %s", err.Error()),
			Timestamp:  time.Now(),
		}, nil
	}

	// Confidence too low - reject
	if v.logger != nil {
		confidence := 0.0
		if analysis != nil {
			confidence = analysis.Confidence
		}
		v.logger.Info("[VOLUME_IMBALANCE_LLM] LLM error and confidence too low",
			"symbol", pattern.Symbol,
			"confidence", confidence,
			"min_score", minScore,
			"error", err.Error())
	}

	return &LLMValidationResult{
		Approved:   false,
		Reason:     fmt.Sprintf("LLM validation failed and confidence (%.1f%%) below minimum (%.1f%%)", analysis.Confidence, minScore),
		Skipped:    false,
		SkipReason: "",
		Timestamp:  time.Now(),
	}, nil
}

// ============================================================================
// INTEGRATION HELPERS
// ============================================================================

// SetLLMClient creates and sets an LLM validator for the VolumeImbalanceDetector.
// This is a convenience method that creates a new validator with the given client.
// For more control over validator configuration, use SetLLMValidator directly.
func (v *VolumeImbalanceDetector) SetLLMClient(client *llm.Client) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if client == nil {
		return
	}

	// Create a new validator with the client
	// Enable by default if client is provided
	validator := NewVolumeImbalanceLLMValidator(client, true)
	if v.logger != nil {
		validator.SetLogger(v.logger)
	}
	v.llmValidator = validator
}

// ResetPatternOnRejection resets a pattern when rejected by LLM
func (v *VolumeImbalanceDetector) ResetPatternOnRejection(symbol string, reason string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if pattern, exists := v.patterns[symbol]; exists {
		// Reset the pattern state
		pattern.State = VIStateWatching
		pattern.InvalidReason = fmt.Sprintf("LLM rejected: %s", reason)
		pattern.IsValid = false
		pattern.StateChanges = append(pattern.StateChanges, fmt.Sprintf("LLM_REJECTED: %s at %s", reason, time.Now().Format("15:04:05")))
		pattern.UpdatedAt = time.Now()

		// Clear pattern data
		pattern.ReferenceCandle = struct {
			Time           time.Time `json:"time"`
			High           float64   `json:"high"`
			Low            float64   `json:"low"`
			Close          float64   `json:"close"`
			Volume         float64   `json:"volume"`
			TakerBuyVolume float64   `json:"taker_buy_volume"`
			Index          int       `json:"index"`
		}{}
		pattern.ConsolidationCandles = 0
		pattern.ConsolidationLow = 0
		pattern.ConsolidationHigh = 0
		pattern.ConsolidationAvgVolume = 0
		pattern.VolumeTrend = 0
		pattern.BreakoutCandle = struct {
			Time   time.Time `json:"time"`
			High   float64   `json:"high"`
			Volume float64   `json:"volume"`
		}{}
		pattern.ExpiresAt = time.Time{}

		if v.logger != nil {
			v.logger.Info("[VOLUME_IMBALANCE_LLM] Pattern reset after LLM rejection",
				"symbol", symbol,
				"reason", reason)
		}
	}
}
