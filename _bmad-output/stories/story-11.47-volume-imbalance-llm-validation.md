# Story 11.47: Volume Imbalance - LLM Validation Integration

## Story Overview

**Story ID:** 11.47
**Epic:** Epic 11 - Position Decision Engine
**Parent Story:** 11.43 (Ravindra Volume Imbalance Strategy)
**Priority:** P2 (Medium)
**Status:** Blocked (depends on 11.43 core logic)
**Created:** 2026-01-24

---

## Business Context

LLM validation adds an additional filter to reduce false signals. When a pattern reaches READY state, the pattern data is sent to the LLM for validation before entry is allowed.

---

## Dependencies

- **Story 11.43:** Core pattern detection (completed)
- **Story 11.46:** UI components (for displaying validation status)

---

## Scope

### In Scope
- LLM prompt for pattern validation
- Integration with existing LLM decision engine
- Validation result storage and display
- Rejection reason logging

### Out of Scope
- UI components (Story 11.46)
- New LLM provider integrations

---

## Technical Implementation

### Task 1: LLM Validation Prompt

**File:** `internal/autopilot/volume_imbalance_llm.go`

```go
// VolumeImbalanceLLMPrompt generates the prompt for LLM validation
func (v *VolumeImbalanceDetector) GenerateLLMPrompt(pattern *VolumeImbalancePattern, analysis *VolumeImbalanceAnalysis) string {
    return fmt.Sprintf(`
You are validating a Volume Imbalance pattern for potential entry.

## Pattern Data
Symbol: %s
Mode: %s (timeframe: %s)
Current Price: %.2f

## 3-Step Pattern Status
Step 1 - Accumulation Start:
  - Reference Candle High: %.2f
  - Reference Candle Low: %.2f
  - Volume: %.2f (%.1fx average)
  - Time: %s

Step 2 - Sideways Consolidation:
  - Duration: %d candles
  - Consolidation Low: %.2f
  - Consolidation High: %.2f
  - Volume Trend: %.2f (declining)

Step 3 - Breakout:
  - Breakout Price: %.2f
  - Breakout Volume: %.2f (%.1fx consolidation avg)
  - Price vs Reference High: %.2f%%

## Proposed Trade
Entry: %.2f
Stop-Loss: %.2f (%.2f%% risk)
Take-Profit: %.2f (%.2f%% reward)
R:R Ratio: 1:%.1f

## Your Task
Validate this pattern. Consider:
1. Is the volume spike significant enough for institutional activity?
2. Is the consolidation clean (not too volatile)?
3. Is the breakout convincing (not a false breakout)?
4. Are there any concerning patterns that suggest reversal?

Respond with:
- APPROVE: If the pattern is valid and trade should proceed
- REJECT: [reason] - If the pattern has issues

Examples:
- APPROVE
- REJECT: Volume spike not significant enough (only 1.5x, needs 2x+)
- REJECT: Consolidation too short (2 candles, needs 3+)
- REJECT: Breakout without volume confirmation
`,
        pattern.Symbol,
        pattern.Mode,
        v.config.Timeframe,
        analysis.CurrentPrice,
        pattern.ReferenceCandle.High,
        pattern.ReferenceCandle.Low,
        pattern.ReferenceCandle.Volume,
        pattern.ReferenceCandle.VolumeMultiple,
        pattern.ReferenceCandle.Time.Format(time.RFC3339),
        pattern.ConsolidationCandles,
        pattern.ConsolidationLow,
        pattern.ConsolidationHigh,
        pattern.ConsolidationVolumeTrend,
        analysis.EntryPrice,
        analysis.BreakoutVolume,
        analysis.BreakoutVolumeMultiple,
        ((analysis.CurrentPrice - pattern.ReferenceCandle.High) / pattern.ReferenceCandle.High) * 100,
        analysis.EntryPrice,
        analysis.StopLoss,
        analysis.RiskPercent,
        analysis.TakeProfit,
        analysis.RewardPercent,
        analysis.RiskRewardRatio,
    )
}
```

### Task 2: LLM Validation Service

**File:** `internal/autopilot/volume_imbalance_llm.go`

```go
// ValidatePatternWithLLM sends pattern to LLM for validation
func (v *VolumeImbalanceDetector) ValidatePatternWithLLM(
    ctx context.Context,
    pattern *VolumeImbalancePattern,
    analysis *VolumeImbalanceAnalysis,
    llmService LLMService,
) (*LLMValidationResult, error) {
    if !v.config.LLMValidation {
        return &LLMValidationResult{
            Approved: true,
            Reason:   "LLM validation disabled",
            Skipped:  true,
        }, nil
    }

    prompt := v.GenerateLLMPrompt(pattern, analysis)

    response, err := llmService.Query(ctx, prompt)
    if err != nil {
        return nil, fmt.Errorf("LLM query failed: %w", err)
    }

    return v.ParseLLMResponse(response)
}

// LLMValidationResult holds the validation outcome
type LLMValidationResult struct {
    Approved  bool      `json:"approved"`
    Reason    string    `json:"reason"`
    Skipped   bool      `json:"skipped"`
    Timestamp time.Time `json:"timestamp"`
    RawResponse string  `json:"raw_response,omitempty"`
}

// ParseLLMResponse extracts APPROVE/REJECT from LLM response
func (v *VolumeImbalanceDetector) ParseLLMResponse(response string) (*LLMValidationResult, error) {
    response = strings.TrimSpace(strings.ToUpper(response))

    if strings.HasPrefix(response, "APPROVE") {
        return &LLMValidationResult{
            Approved:  true,
            Reason:    "Pattern validated by LLM",
            Timestamp: time.Now(),
        }, nil
    }

    if strings.HasPrefix(response, "REJECT:") {
        reason := strings.TrimPrefix(response, "REJECT:")
        reason = strings.TrimSpace(reason)
        return &LLMValidationResult{
            Approved:    false,
            Reason:      reason,
            Timestamp:   time.Now(),
            RawResponse: response,
        }, nil
    }

    // Fallback: try to detect approval/rejection from response
    if strings.Contains(response, "APPROVE") {
        return &LLMValidationResult{Approved: true, Reason: "Implicit approval"}, nil
    }

    return &LLMValidationResult{
        Approved:    false,
        Reason:      "Could not parse LLM response",
        RawResponse: response,
    }, nil
}
```

### Task 3: Integration with Autopilot

**File:** `internal/autopilot/ginie_autopilot.go` (update)

```go
// In entry decision flow, after pattern reaches READY:
func (ga *GinieAutopilot) handleVolumeImbalanceEntry(symbol string, analysis *VolumeImbalanceAnalysis) error {
    pattern := ga.volumeImbalanceDetector.GetPattern(symbol)

    // Step 1: Check if LLM validation is enabled
    if ga.volumeImbalanceConfig.LLMValidation {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        result, err := ga.volumeImbalanceDetector.ValidatePatternWithLLM(
            ctx, pattern, analysis, ga.llmService,
        )

        if err != nil {
            ga.logger.Error("LLM validation failed", "error", err)
            // Fallback: proceed without LLM validation
        } else if !result.Approved {
            ga.logger.Warn("LLM rejected pattern",
                "symbol", symbol,
                "reason", result.Reason,
            )

            // Log rejection for analytics
            ga.logLLMRejection(symbol, pattern, result)

            // Reset pattern and continue watching
            ga.volumeImbalanceDetector.ResetPattern(symbol, "LLM rejection: " + result.Reason)
            return nil
        }

        ga.logger.Info("LLM approved pattern",
            "symbol", symbol,
        )
    }

    // Step 2: Proceed with entry
    return ga.executeVolumeImbalanceEntry(symbol, analysis)
}
```

### Task 4: Rejection Logging

**File:** `internal/autopilot/volume_imbalance_llm.go`

```go
// LogLLMRejection stores rejection for analytics
func (ga *GinieAutopilot) logLLMRejection(
    symbol string,
    pattern *VolumeImbalancePattern,
    result *LLMValidationResult,
) {
    rejection := LLMRejectionLog{
        Symbol:      symbol,
        PatternID:   pattern.ID,
        Reason:      result.Reason,
        RawResponse: result.RawResponse,
        PatternData: map[string]interface{}{
            "reference_high":       pattern.ReferenceCandle.High,
            "reference_volume":     pattern.ReferenceCandle.Volume,
            "consolidation_candles": pattern.ConsolidationCandles,
            "breakout_volume":      pattern.BreakoutVolume,
        },
        Timestamp: time.Now(),
    }

    // Store in Redis for recent rejections
    ga.cache.AddLLMRejection(symbol, rejection)

    // Log to database for analytics
    ga.db.InsertLLMRejection(rejection)
}
```

---

## Acceptance Criteria

### AC1: LLM Prompt Generation
- [ ] Prompt includes all pattern data
- [ ] Prompt includes proposed trade setup
- [ ] Prompt asks for APPROVE/REJECT decision

### AC2: LLM Validation Flow
- [ ] Pattern sent to LLM when READY
- [ ] APPROVE proceeds to entry
- [ ] REJECT resets pattern and logs reason

### AC3: Rejection Logging
- [ ] Rejections logged with reason
- [ ] Pattern data stored for analysis
- [ ] Accessible via API for UI display

### AC4: Fallback Handling
- [ ] LLM timeout doesn't block entry
- [ ] LLM errors logged but don't crash
- [ ] Config to disable LLM validation

---

## Test Plan

1. **Unit Tests:** Prompt generation, response parsing
2. **Integration Tests:** Full validation flow with mock LLM
3. **Fallback Tests:** Timeout handling, error handling

---

## Estimation

| Task | Effort |
|------|--------|
| LLM prompt generation | Medium |
| Validation service | Medium |
| Autopilot integration | Medium |
| Rejection logging | Small |
| Testing | Medium |

**Total:** Medium
