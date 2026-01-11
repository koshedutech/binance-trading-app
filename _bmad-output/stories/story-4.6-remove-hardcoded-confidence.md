# Story 4.6: Remove All Hardcoded Confidence Values

**Story ID:** MODE-4.6
**Epic:** Epic 4 - Database-First Mode Configuration System
**Priority:** P1 (High - Code Quality & Reliability)
**Estimated Effort:** 4 hours
**Author:** BMAD Agent
**Status:** Ready for Development

---

## Description

Eliminate all remaining hardcoded confidence values from initialization, risk level setters, and configuration defaults. After Story 4.5 applies per-mode confidence to trade decisions, this story completes the cleanup by removing all hardcoded confidence fallbacks that mask configuration issues. This ensures 100% of confidence values come from the database, with explicit errors when configs are missing.

---

## User Story

> As a developer,
> I want zero hardcoded confidence values in the codebase,
> So that all thresholds are traceable to user configuration and missing configs are caught immediately, not silently defaulted.

---

## Acceptance Criteria

### AC4.6.1: Risk Level Switch Statements Removed
- [ ] `ginie_autopilot.go` line 1285-1293 switch statement deleted
- [ ] `futures_controller.go` line 630-640 switch statement deleted
- [ ] `futures_controller.go` line 1151-1168 switch statement deleted
- [ ] `SetRiskLevel()` functions removed or deprecated
- [ ] Grep for "case.*low.*medium.*high" returns no confidence-related matches

### AC4.6.2: Default Confidence Removed from Initialization
- [ ] `ginie_autopilot.go` line 322 default 50.0 removed
- [ ] `initializeGinieConfig()` does not set confidence defaults
- [ ] New configs get confidence from DefaultModeConfigs() only during onboarding
- [ ] Runtime code never sets confidence defaults

### AC4.6.3: Controller Hardcoded Values Removed
- [ ] `controller.go` line 487 hardcoded 0.65 removed
- [ ] `controller.go` line 713-732 `getStopLossPercent()` hardcoded values removed
- [ ] `controller.go` line 745-764 `getTakeProfitPercent()` hardcoded values removed
- [ ] All SL/TP percentages come from mode config

### AC4.6.4: SL/TP LLM Hardcoded Threshold Removed
- [ ] `ginie_autopilot.go` line 9566 hardcoded 0.5 removed
- [ ] Uses mode config instead (already done in Story 4.5, verify here)
- [ ] No fallback to 0.5 if mode config fetch fails

### AC4.6.5: All Locations Read from Mode Config
- [ ] Every location that needs confidence calls `GetModeConfig()`
- [ ] No inline confidence assignments like `confidence := 50.0`
- [ ] No switch statements mapping risk level to confidence
- [ ] Database is the single source of truth

### AC4.6.6: Validation Prevents Zero Confidence
- [ ] API handler rejects mode config updates with confidence = 0
- [ ] Database CHECK constraint prevents confidence = 0 (if not already exists)
- [ ] Error message: "confidence must be between 1.0 and 100.0"
- [ ] Test: Attempt to set confidence to 0, verify rejection

### AC4.6.7: Comprehensive Grep Verification
- [ ] `grep -rn "= 50" internal/autopilot/ | grep -i confidence` returns 0 results
- [ ] `grep -rn "= 0.5" internal/autopilot/ | grep -i confidence` returns 0 results
- [ ] `grep -rn "= 60" internal/autopilot/ | grep -i confidence` returns 0 results
- [ ] `grep -rn "= 0.65" internal/` returns 0 results
- [ ] Manual review confirms no hidden hardcoded values

---

## Technical Implementation Notes

### Files to Modify

#### 1. internal/autopilot/ginie_autopilot.go - Remove SetRiskLevel() Switch

**Lines to Delete:** 1285-1293

**Before (INCORRECT):**
```go
func (ga *GinieAutopilot) SetRiskLevel(riskLevel string) {
    ga.mu.Lock()
    defer ga.mu.Unlock()

    // Hardcoded confidence mapping
    switch riskLevel {
    case "low":
        ga.config.MinConfidence = 60.0
    case "medium":
        ga.config.MinConfidence = 50.0
    case "high":
        ga.config.MinConfidence = 40.0
    default:
        ga.config.MinConfidence = 50.0
    }
}
```

**After (CORRECT):**
```go
// Function removed entirely
// Risk level should update mode config in database via API
// Runtime code should always read from database
```

**Alternative (if SetRiskLevel must remain for backward compatibility):**
```go
func (ga *GinieAutopilot) SetRiskLevel(riskLevel string) error {
    // Don't set confidence directly - must update database
    return fmt.Errorf("SetRiskLevel is deprecated - use UpdateModeConfig API instead")
}
```

#### 2. internal/autopilot/futures_controller.go - Remove SetRiskLevel() Switch

**Lines to Delete:** 630-640 and 1151-1168

**Before (INCORRECT):**
```go
func (fc *FuturesController) SetRiskLevel(riskLevel string) {
    switch riskLevel {
    case "low":
        fc.minConfidence = 60.0
    case "medium":
        fc.minConfidence = 50.0
    case "high":
        fc.minConfidence = 40.0
    default:
        fc.minConfidence = 50.0
    }
}
```

**After (CORRECT):**
```go
// Function removed entirely or deprecated
func (fc *FuturesController) SetRiskLevel(riskLevel string) error {
    return fmt.Errorf("SetRiskLevel is deprecated - confidence comes from database mode config")
}
```

#### 3. internal/autopilot/ginie_autopilot.go - Remove Initialization Default

**Line to Delete:** 322

**Before (INCORRECT):**
```go
func initializeGinieConfig() *GinieConfig {
    return &GinieConfig{
        MinConfidence: 50.0, // ❌ Hardcoded default
        MaxLeverage: 10,
        // ... other fields
    }
}
```

**After (CORRECT):**
```go
func initializeGinieConfig() *GinieConfig {
    return &GinieConfig{
        // MinConfidence removed - comes from database
        MaxLeverage: 10,
        // ... other fields
    }
}

// OR mark struct field as deprecated
type GinieConfig struct {
    MinConfidence float64 `json:"min_confidence" deprecated:"use mode config from database"`
    // ...
}
```

#### 4. internal/autopilot/controller.go - Remove Hardcoded 0.65

**Line to Delete:** 487

**Before (INCORRECT):**
```go
func (c *Controller) shouldExecuteTrade() bool {
    confidence := 0.65 // ❌ Hardcoded
    return c.signal.Confidence >= confidence
}
```

**After (CORRECT):**
```go
func (c *Controller) shouldExecuteTrade(modeName string) bool {
    // Get mode config from database
    modeConfig, err := c.settings.GetModeConfig(c.userID, modeName)
    if err != nil {
        log.Errorf("Failed to get mode config for %s: %v", modeName, err)
        return false
    }

    threshold := modeConfig.Confidence.MinConfidence
    return c.signal.Confidence >= threshold
}
```

#### 5. internal/autopilot/controller.go - Remove SL/TP Hardcoded Values

**Lines to Delete:** 713-732, 745-764

**Before (INCORRECT):**
```go
func (c *Controller) getStopLossPercent() float64 {
    // Hardcoded stop loss percentages
    switch c.riskLevel {
    case "low":
        return 2.0
    case "medium":
        return 3.0
    case "high":
        return 5.0
    default:
        return 3.0
    }
}

func (c *Controller) getTakeProfitPercent() float64 {
    // Hardcoded take profit percentages
    switch c.riskLevel {
    case "low":
        return 4.0
    case "medium":
        return 6.0
    case "high":
        return 10.0
    default:
        return 6.0
    }
}
```

**After (CORRECT):**
```go
func (c *Controller) getStopLossPercent(modeName string) (float64, error) {
    // Get from mode config
    modeConfig, err := c.settings.GetModeConfig(c.userID, modeName)
    if err != nil {
        return 0, fmt.Errorf("failed to get mode config for %s: %w", modeName, err)
    }
    return modeConfig.StopLoss.Percent, nil
}

func (c *Controller) getTakeProfitPercent(modeName string) (float64, error) {
    // Get from mode config
    modeConfig, err := c.settings.GetModeConfig(c.userID, modeName)
    if err != nil {
        return 0, fmt.Errorf("failed to get mode config for %s: %w", modeName, err)
    }
    return modeConfig.TakeProfit.Percent, nil
}
```

### Database Constraint for Confidence Validation

**Add to Migration (if not already exists):**
```sql
-- Ensure confidence is never 0 or negative
ALTER TABLE user_mode_configs
ADD CONSTRAINT check_confidence_positive
CHECK ((config_json->>'min_confidence')::numeric > 0 AND (config_json->>'min_confidence')::numeric <= 100);
```

### API Handler Validation

**File:** `internal/api/handlers_settings.go`

**Add Validation:**
```go
func (h *Handler) UpdateModeConfig(c *gin.Context) {
    var req UpdateModeConfigRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // Validate confidence
    if req.Confidence.MinConfidence <= 0 || req.Confidence.MinConfidence > 100 {
        c.JSON(400, gin.H{"error": "confidence must be between 1.0 and 100.0"})
        return
    }

    // ... rest of handler
}
```

---

## Testing Requirements

### Test 1: Grep Verification for Hardcoded Values
```bash
# Search for common hardcoded confidence values
cd /mnt/c/KOSH/binance-trading-bot

# Check for "= 50" in confidence context
grep -rn "= 50" internal/autopilot/ | grep -i confidence
# Expected: 0 results

# Check for "= 0.5" in confidence context
grep -rn "= 0.5" internal/autopilot/ | grep -i confidence
# Expected: 0 results

# Check for "= 60" in confidence context
grep -rn "= 60" internal/autopilot/ | grep -i confidence
# Expected: 0 results

# Check for "= 0.65"
grep -rn "= 0.65" internal/
# Expected: 0 results

# Check for risk level switch statements
grep -rn "case.*low.*medium.*high" internal/autopilot/ | grep -i confidence
# Expected: 0 results

# Check for confidence assignment in switch statements
grep -rn "MinConfidence.*=" internal/autopilot/ | grep -E "(case|switch)"
# Expected: 0 results
```

### Test 2: SetRiskLevel Deprecated
```bash
# Test SetRiskLevel returns error
curl -X POST http://localhost:8094/api/autopilot/risk-level \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"risk_level": "high"}'

# Expected: 400 Bad Request
# Expected body: {"error": "SetRiskLevel is deprecated - use UpdateModeConfig API instead"}
```

### Test 3: Zero Confidence Rejected
```bash
# Attempt to set confidence to 0
curl -X PUT http://localhost:8094/api/autopilot/mode/scalp \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "confidence": {
      "min_confidence": 0.0
    }
  }'

# Expected: 400 Bad Request
# Expected body: {"error": "confidence must be between 1.0 and 100.0"}
```

### Test 4: Negative Confidence Rejected
```bash
# Attempt to set confidence to negative value
curl -X PUT http://localhost:8094/api/autopilot/mode/scalp \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "confidence": {
      "min_confidence": -10.0
    }
  }'

# Expected: 400 Bad Request
# Expected body: {"error": "confidence must be between 1.0 and 100.0"}
```

### Test 5: Confidence Above 100 Rejected
```bash
# Attempt to set confidence above 100
curl -X PUT http://localhost:8094/api/autopilot/mode/scalp \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "confidence": {
      "min_confidence": 150.0
    }
  }'

# Expected: 400 Bad Request
# Expected body: {"error": "confidence must be between 1.0 and 100.0"}
```

### Test 6: SL/TP from Mode Config
```bash
# 1. Set custom SL/TP for scalp mode
curl -X PUT http://localhost:8094/api/autopilot/mode/scalp \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "stop_loss": {
      "percent": 1.5
    },
    "take_profit": {
      "percent": 3.0
    }
  }'

# 2. Execute trade (paper mode)
# 3. Check logs for SL/TP values
docker logs binance-trading-bot-dev 2>&1 | grep "SL.*1.5.*TP.*3.0"

# Expected: Position opened with SL=1.5%, TP=3.0%
# NOT default values like SL=3.0%, TP=6.0%
```

### Test 7: All Config Values from Database
```go
func TestNoHardcodedConfidence(t *testing.T) {
    // Load all Go files in autopilot package
    files := []string{
        "internal/autopilot/ginie_autopilot.go",
        "internal/autopilot/futures_controller.go",
        "internal/autopilot/controller.go",
        "internal/autopilot/settings.go",
    }

    // Pattern to detect hardcoded confidence assignments
    pattern := regexp.MustCompile(`(?i)(confidence|threshold)\s*[:=]\s*\d+\.?\d*`)

    for _, file := range files {
        content, err := os.ReadFile(file)
        if err != nil {
            t.Fatalf("Failed to read %s: %v", file, err)
        }

        matches := pattern.FindAllString(string(content), -1)
        if len(matches) > 0 {
            t.Errorf("Found hardcoded confidence in %s: %v", file, matches)
        }
    }
}
```

---

## Dependencies

### Prerequisites
- **Story 4.5:** Apply per-mode confidence (all decision points use mode config)
- **Story 4.4:** Remove mergeWithDefaultConfigs (GetModeConfig returns DB values)
- **Story 4.2:** Repository layer implemented
- **Story 4.3:** API handlers updated

### Blocks
- None (final cleanup story in confidence refactoring)

---

## Deployment Notes

### Development Environment
```bash
# 1. Verify Story 4.5 is complete
# All decision points should use GetModeConfig()

# 2. Make code changes
# - Delete SetRiskLevel() switch statements
# - Delete initialization defaults
# - Delete controller hardcoded values
# - Add API validation for confidence range

# 3. Run grep tests to verify no hardcoded values remain
cd /mnt/c/KOSH/binance-trading-bot
grep -rn "= 50" internal/autopilot/ | grep -i confidence
grep -rn "= 0.5" internal/autopilot/ | grep -i confidence
grep -rn "= 60" internal/autopilot/ | grep -i confidence
grep -rn "= 0.65" internal/

# Expected: All return 0 results

# 4. Restart container
./scripts/docker-dev.sh

# 5. Wait for build
sleep 60

# 6. Run Test 3: Zero confidence rejected
# 7. Run Test 6: SL/TP from mode config
```

### Production Environment
1. **Pre-Deployment:**
   - Verify Story 4.5 deployed successfully
   - Verify all trades use mode config confidence
   - Backup database
   - Review all code deletions

2. **Deployment:**
   - Deploy updated code
   - Restart production containers
   - Monitor logs for errors

3. **Post-Deployment Verification:**
   - Test SetRiskLevel returns deprecation error
   - Test zero confidence rejected
   - Verify trades use database SL/TP values
   - Monitor for any fallback behavior (should not exist)

4. **Rollback Procedure (if needed):**
   - Restore previous code version
   - Restart containers

---

## Definition of Done

- [ ] All risk level switch statements removed (5 locations)
- [ ] Initialization default confidence removed (line 322)
- [ ] Controller hardcoded 0.65 removed (line 487)
- [ ] getStopLossPercent/getTakeProfitPercent hardcoded values removed
- [ ] SL/TP LLM hardcoded 0.5 removed (verified from Story 4.5)
- [ ] API handler validates confidence range (1.0 - 100.0)
- [ ] Database constraint prevents confidence = 0 (optional but recommended)
- [ ] Code compiles successfully
- [ ] Test 1: Grep verification passes (0 results for all searches)
- [ ] Test 2: SetRiskLevel deprecated test passes
- [ ] Test 3: Zero confidence rejected test passes
- [ ] Test 4: Negative confidence rejected test passes
- [ ] Test 5: Confidence above 100 rejected test passes
- [ ] Test 6: SL/TP from mode config test passes
- [ ] Test 7: No hardcoded confidence test passes
- [ ] Code review approved
- [ ] Changes tested in development environment
- [ ] Documentation updated

---

## Notes for Developer

### Why This Story Matters

This story completes the confidence refactoring trilogy:

1. **Story 4.4:** Removed merge logic (database becomes source of truth)
2. **Story 4.5:** Applied mode config to decisions (runtime uses database)
3. **Story 4.6:** Removed all hardcoded fallbacks (no escape hatches)

**Result:** 100% of confidence values traceable to user configuration in database.

### Benefits After Completion

| Benefit | Description |
|---------|-------------|
| **Traceability** | Every confidence value has clear origin in database |
| **No Silent Failures** | Missing configs return errors, not hidden defaults |
| **Easier Debugging** | All values logged with "(from DB)" indicator |
| **Consistent Behavior** | No code path bypasses user configuration |
| **Testability** | Can verify all confidence comes from database |

### Common Pitfalls to Avoid

- ❌ **DON'T** add "temporary" hardcoded value "just for testing"
- ❌ **DON'T** keep SetRiskLevel() functional "for backward compatibility"
- ❌ **DON'T** allow confidence = 0 (would cause divide-by-zero or logic errors)
- ✅ **DO** deprecate old APIs instead of removing (safer)
- ✅ **DO** validate confidence range at API boundary
- ✅ **DO** run all grep tests to verify cleanup

### Deprecation vs Deletion

**Option A: Delete SetRiskLevel() entirely**
- Pros: Clean codebase, no confusion
- Cons: Breaks any external callers (if any)

**Option B: Deprecate SetRiskLevel()**
- Pros: Safer, gives migration time
- Cons: Keeps dead code around

**Recommended:** Deprecate in this story, delete in future cleanup story.

```go
// Deprecated: SetRiskLevel is deprecated. Use UpdateModeConfig API instead.
// This function will be removed in v2.0.0.
func (ga *GinieAutopilot) SetRiskLevel(riskLevel string) error {
    return fmt.Errorf("SetRiskLevel is deprecated - use UpdateModeConfig API")
}
```

### Grep Patterns to Check

```bash
# After story completion, these should ALL return 0 results:

# Common hardcoded percentages
grep -rn "= 50" internal/autopilot/ | grep -i confidence
grep -rn "= 60" internal/autopilot/ | grep -i confidence
grep -rn "= 40" internal/autopilot/ | grep -i confidence
grep -rn "= 70" internal/autopilot/ | grep -i confidence

# Decimal format
grep -rn "= 0.5" internal/autopilot/ | grep -i confidence
grep -rn "= 0.6" internal/autopilot/ | grep -i confidence
grep -rn "= 0.65" internal/

# Risk level switches
grep -rn "case.*low.*medium.*high" internal/autopilot/ | grep -i confidence
grep -rn "switch.*risk" internal/autopilot/ | grep -i confidence

# Direct MinConfidence assignment
grep -rn "MinConfidence.*=" internal/autopilot/ | grep -v "GetModeConfig"
```

---

## Related Stories

- **Story 4.4:** Remove mergeWithDefaultConfigs (prerequisite)
- **Story 4.5:** Apply per-mode confidence (prerequisite)
- **Story 4.7:** Frontend mode display (independent, can be parallel)
- **Story 4.8:** User onboarding (independent, can be parallel)

---

## Approval Sign-Off

- **Scrum Master**: ✅ Story Ready for Development
- **Developer**: _Pending Assignment_
- **Test Architect**: _Pending Test Review_
- **Product Manager**: _Pending Acceptance_
