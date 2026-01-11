# Story 4.5: Apply Per-Mode Confidence to Trade Decisions

**Story ID:** MODE-4.5
**Epic:** Epic 4 - Database-First Mode Configuration System
**Priority:** P1 (High - Critical Business Logic)
**Estimated Effort:** 6 hours
**Author:** BMAD Agent
**Status:** Ready for Development

---

## Description

Update all trade decision points (11 locations across 4 files) to use per-mode confidence thresholds from the database instead of hardcoded 50% values. This ensures that when a user sets scalp mode to 45% confidence, trades are actually executed at 45% confidence, not 50%. This story implements the core business logic fix that makes user-configured confidence settings actually work.

---

## User Story

> As a trader,
> I want my configured 45% scalp confidence setting to be used in trade decisions,
> So that trades execute at my specified threshold, not a hardcoded value that ignores my preferences.

---

## Acceptance Criteria

### AC4.5.1: Ultra-Fast Entry Uses Mode Config
- [ ] `ginie_autopilot.go` line 2670 queries database for ultra-fast mode config
- [ ] Confidence check uses `modeConfig.Confidence.MinConfidence` (not hardcoded 50%)
- [ ] Logs show: `[ULTRA-FAST] Checking confidence: signal=47%, threshold=45% (from DB)`
- [ ] Test: Set ultra-fast to 45%, verify 47% signal triggers entry

### AC4.5.2: Scalp Entry Uses Mode Config
- [ ] `ginie_autopilot.go` line 2276 queries database for scalp mode config
- [ ] Confidence check uses `modeConfig.Confidence.MinConfidence`
- [ ] Logs show: `[SCALP] Checking confidence: signal=52%, threshold=50% (from DB)`
- [ ] Test: Set scalp to 50%, verify 52% signal triggers entry

### AC4.5.3: Swing Entry Uses Mode Config
- [ ] `ginie_autopilot.go` entry logic queries database for swing mode config
- [ ] Confidence check uses `modeConfig.Confidence.MinConfidence`
- [ ] Logs show: `[SWING] Checking confidence: signal=61%, threshold=60% (from DB)`
- [ ] Test: Set swing to 60%, verify 61% signal triggers entry

### AC4.5.4: Position Entry Uses Mode Config
- [ ] `ginie_autopilot.go` entry logic queries database for position mode config
- [ ] Confidence check uses `modeConfig.Confidence.MinConfidence`
- [ ] Logs show: `[POSITION] Checking confidence: signal=71%, threshold=70% (from DB)`
- [ ] Test: Set position to 70%, verify 71% signal triggers entry

### AC4.5.5: Averaging Decision Uses Mode Config
- [ ] `futures_controller.go` line 4154 queries database for current mode config
- [ ] Averaging check uses `modeConfig.Averaging.MinConfidenceForAverage`
- [ ] Logs show: `[AVERAGING] Checking confidence: signal=55%, threshold=52% (from DB)`
- [ ] Test: Set averaging confidence to 52%, verify 55% signal triggers averaging

### AC4.5.6: Hedging Decision Uses Mode Config
- [ ] `hedging.go` line 200 queries database for mode config
- [ ] Hedging confidence check uses `modeConfig.Hedging.MinConfidence`
- [ ] Logs show: `[HEDGING] Checking confidence: signal=48%, threshold=45% (from DB)`
- [ ] Test: Set hedging to 45%, verify 48% signal triggers hedge

### AC4.5.7: Counter-Trend Check Uses Mode Config
- [ ] `ginie_analyzer.go` line 3056 queries database for mode config
- [ ] Counter-trend confidence check uses mode-specific threshold
- [ ] Logs show mode and threshold used
- [ ] Test: Verify mode config applied to counter-trend decisions

### AC4.5.8: Early Warning Uses Mode Config
- [ ] `ginie_autopilot.go` line 9267 queries database for mode config
- [ ] Early warning confidence uses mode-specific threshold
- [ ] Logs show mode and threshold used
- [ ] Test: Verify mode config applied to early warnings

### AC4.5.9: SL/TP LLM Uses Mode Config
- [ ] `ginie_autopilot.go` line 9566 queries database for mode config
- [ ] Removes hardcoded 0.5 confidence threshold
- [ ] Uses `modeConfig.Confidence.MinConfidence` for SL/TP decisions
- [ ] Test: Verify SL/TP decisions respect mode confidence

### AC4.5.10: Confidence Format Standardization
- [ ] All confidence values use 0-100 format consistently
- [ ] No mixing of 0.0-1.0 and 0-100 formats
- [ ] Conversion functions used where needed
- [ ] Logs display confidence as percentage (e.g., "45%" not "0.45")

### AC4.5.11: All 11 Locations Updated
- [ ] All confidence check points identified and updated
- [ ] Each location queries appropriate mode config from database
- [ ] Each location logs mode name and threshold used
- [ ] No hardcoded confidence values remain in decision logic

---

## Technical Implementation Notes

### Files to Modify

#### 1. internal/autopilot/ginie_autopilot.go - Ultra-Fast Entry (Line ~2670)

**Before (INCORRECT):**
```go
// Hardcoded 50% threshold
if signal.Confidence < 50.0 {
    log.Debugf("[ULTRA-FAST] Signal confidence too low: %.2f%%", signal.Confidence)
    return
}
```

**After (CORRECT):**
```go
// Get ultra-fast mode config from database
ultraFastConfig, err := ga.settings.GetModeConfig(ga.userID, "ultra_fast")
if err != nil {
    log.Errorf("[ULTRA-FAST] Failed to get mode config: %v", err)
    return
}

// Use configured threshold
threshold := ultraFastConfig.Confidence.MinConfidence
if signal.Confidence < threshold {
    log.Debugf("[ULTRA-FAST] Signal confidence too low: signal=%.2f%%, threshold=%.2f%% (from DB)",
        signal.Confidence, threshold)
    return
}
log.Infof("[ULTRA-FAST] Signal confidence passed: signal=%.2f%%, threshold=%.2f%% (from DB)",
    signal.Confidence, threshold)
```

#### 2. internal/autopilot/ginie_autopilot.go - Scalp Entry (Line ~2276)

**Before (INCORRECT):**
```go
// Hardcoded 50% threshold
if decision.Confidence < 50.0 {
    return
}
```

**After (CORRECT):**
```go
// Get scalp mode config from database
scalpConfig, err := ga.settings.GetModeConfig(ga.userID, "scalp")
if err != nil {
    log.Errorf("[SCALP] Failed to get mode config: %v", err)
    return
}

threshold := scalpConfig.Confidence.MinConfidence
if decision.Confidence < threshold {
    log.Debugf("[SCALP] Confidence too low: signal=%.2f%%, threshold=%.2f%% (from DB)",
        decision.Confidence, threshold)
    return
}
log.Infof("[SCALP] Confidence passed: signal=%.2f%%, threshold=%.2f%% (from DB)",
    decision.Confidence, threshold)
```

#### 3. internal/autopilot/futures_controller.go - Averaging Check (Line ~4154)

**Before (INCORRECT):**
```go
// Hardcoded threshold or wrong mode's config
if confidence < 50.0 {
    return false
}
```

**After (CORRECT):**
```go
// Get current mode config from position metadata
modeName := position.TradingMode // Get mode from position
if modeName == "" {
    modeName = "scalp" // Fallback for legacy positions
}

modeConfig, err := fc.settings.GetModeConfig(fc.userID, modeName)
if err != nil {
    log.Errorf("[AVERAGING] Failed to get mode config for %s: %v", modeName, err)
    return false
}

// Use mode-specific averaging threshold
threshold := modeConfig.Averaging.MinConfidenceForAverage
if confidence < threshold {
    log.Debugf("[AVERAGING] Confidence too low for %s: signal=%.2f%%, threshold=%.2f%% (from DB)",
        modeName, confidence, threshold)
    return false
}
log.Infof("[AVERAGING] Confidence passed for %s: signal=%.2f%%, threshold=%.2f%% (from DB)",
    modeName, confidence, threshold)
return true
```

#### 4. internal/autopilot/hedging.go - Hedging Decision (Line ~200)

**Before (INCORRECT):**
```go
// Hardcoded or missing threshold
if hedgeSignal.Confidence < 0.5 { // 50% in 0-1 format
    return
}
```

**After (CORRECT):**
```go
// Get mode config for current position's mode
modeConfig, err := h.settings.GetModeConfig(h.userID, position.TradingMode)
if err != nil {
    log.Errorf("[HEDGING] Failed to get mode config for %s: %v", position.TradingMode, err)
    return
}

// Convert 0-1 format to 0-100 for comparison
threshold := modeConfig.Hedging.MinConfidence
if hedgeSignal.Confidence*100 < threshold {
    log.Debugf("[HEDGING] Confidence too low for %s: signal=%.2f%%, threshold=%.2f%% (from DB)",
        position.TradingMode, hedgeSignal.Confidence*100, threshold)
    return
}
log.Infof("[HEDGING] Confidence passed for %s: signal=%.2f%%, threshold=%.2f%% (from DB)",
    position.TradingMode, hedgeSignal.Confidence*100, threshold)
```

#### 5. internal/autopilot/ginie_analyzer.go - Counter-Trend Check (Line ~3056)

**Before (INCORRECT):**
```go
// Hardcoded threshold
if counterTrendConfidence < 0.6 { // 60%
    return false
}
```

**After (CORRECT):**
```go
// Get mode config (mode name should be passed to this function)
modeConfig, err := ga.settings.GetModeConfig(ga.userID, modeName)
if err != nil {
    log.Errorf("[COUNTER-TREND] Failed to get mode config for %s: %v", modeName, err)
    return false
}

// Use mode-specific counter-trend threshold
threshold := modeConfig.CounterTrend.MinConfidence
if counterTrendConfidence*100 < threshold {
    log.Debugf("[COUNTER-TREND] Confidence too low for %s: signal=%.2f%%, threshold=%.2f%% (from DB)",
        modeName, counterTrendConfidence*100, threshold)
    return false
}
return true
```

#### 6. internal/autopilot/ginie_autopilot.go - Early Warning (Line ~9267)

**Before (INCORRECT):**
```go
// Hardcoded or missing threshold
if warningConfidence < 0.55 { // 55%
    return
}
```

**After (CORRECT):**
```go
// Get mode config for the position's mode
modeConfig, err := ga.settings.GetModeConfig(ga.userID, position.TradingMode)
if err != nil {
    log.Errorf("[EARLY-WARNING] Failed to get mode config for %s: %v", position.TradingMode, err)
    return
}

threshold := modeConfig.EarlyWarning.MinConfidence
if warningConfidence*100 < threshold {
    log.Debugf("[EARLY-WARNING] Confidence too low for %s: signal=%.2f%%, threshold=%.2f%% (from DB)",
        position.TradingMode, warningConfidence*100, threshold)
    return
}
log.Infof("[EARLY-WARNING] Confidence passed for %s: signal=%.2f%%, threshold=%.2f%% (from DB)",
    position.TradingMode, warningConfidence*100, threshold)
```

#### 7. internal/autopilot/ginie_autopilot.go - SL/TP LLM (Line ~9566)

**Before (INCORRECT):**
```go
// Hardcoded 0.5 (50%) threshold
if llmConfidence < 0.5 {
    log.Debugf("[SL/TP-LLM] Confidence too low: %.2f", llmConfidence)
    return
}
```

**After (CORRECT):**
```go
// Get mode config for the position's mode
modeConfig, err := ga.settings.GetModeConfig(ga.userID, position.TradingMode)
if err != nil {
    log.Errorf("[SL/TP-LLM] Failed to get mode config for %s: %v", position.TradingMode, err)
    return
}

threshold := modeConfig.Confidence.MinConfidence
if llmConfidence*100 < threshold {
    log.Debugf("[SL/TP-LLM] Confidence too low for %s: signal=%.2f%%, threshold=%.2f%% (from DB)",
        position.TradingMode, llmConfidence*100, threshold)
    return
}
log.Infof("[SL/TP-LLM] Confidence passed for %s: signal=%.2f%%, threshold=%.2f%% (from DB)",
    position.TradingMode, llmConfidence*100, threshold)
```

### Confidence Format Standardization

**Problem:** Codebase mixes two formats:
- Format A: 0.0-1.0 (e.g., 0.45 = 45%)
- Format B: 0-100 (e.g., 45.0 = 45%)

**Solution:** Standardize to 0-100 everywhere

**Conversion Helper Functions:**
```go
// Add to internal/autopilot/helpers.go

// ConvertConfidenceToPercent converts 0-1 format to 0-100 format
func ConvertConfidenceToPercent(confidence float64) float64 {
    if confidence <= 1.0 {
        return confidence * 100
    }
    return confidence // Already in percent format
}

// ConvertPercentToConfidence converts 0-100 format to 0-1 format
func ConvertPercentToConfidence(percent float64) float64 {
    if percent > 1.0 {
        return percent / 100
    }
    return percent // Already in 0-1 format
}
```

### All 11 Locations Summary

| # | File | Line | Function | Check Type | Mode |
|---|------|------|----------|------------|------|
| 1 | `ginie_autopilot.go` | ~2670 | Entry decision | MinConfidence | ultra_fast |
| 2 | `ginie_autopilot.go` | ~2276 | Entry decision | MinConfidence | scalp |
| 3 | `ginie_autopilot.go` | ~2450 | Entry decision | MinConfidence | swing |
| 4 | `ginie_autopilot.go` | ~2620 | Entry decision | MinConfidence | position |
| 5 | `futures_controller.go` | ~4154 | Averaging | MinConfidenceForAverage | dynamic |
| 6 | `hedging.go` | ~200 | Hedge entry | MinConfidence | dynamic |
| 7 | `ginie_analyzer.go` | ~3056 | Counter-trend | MinConfidence | dynamic |
| 8 | `ginie_autopilot.go` | ~9267 | Early warning | MinConfidence | dynamic |
| 9 | `ginie_autopilot.go` | ~9566 | SL/TP LLM | MinConfidence | dynamic |
| 10 | `ginie_autopilot.go` | ~3100 | Re-entry check | MinConfidence | scalp_reentry |
| 11 | `ginie_autopilot.go` | ~5200 | Exit confidence | MinConfidence | dynamic |

---

## Testing Requirements

### Test 1: Scalp Mode Confidence Threshold
```bash
# 1. Set scalp mode to 45% confidence
curl -X PUT http://localhost:8094/api/autopilot/mode/scalp \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "confidence": {
      "min_confidence": 45.0
    }
  }'

# 2. Generate AI signal with 47% confidence
# (This requires mocking or waiting for real signal)

# 3. Check logs
docker logs binance-trading-bot-dev 2>&1 | grep "SCALP.*Confidence passed.*signal=47.*threshold=45"

# Expected log:
# [SCALP] Confidence passed: signal=47.00%, threshold=45.00% (from DB)

# 4. Verify trade executed
curl -X GET http://localhost:8094/api/futures/trades \
  -H "Authorization: Bearer <token>" | jq -r '.[0] | "\(.trading_mode) \(.confidence)"'

# Expected: "scalp 47.0"
```

### Test 2: Reject Trade Below Threshold
```bash
# 1. Set scalp mode to 50% confidence
curl -X PUT http://localhost:8094/api/autopilot/mode/scalp \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true, "confidence": {"min_confidence": 50.0}}'

# 2. Generate AI signal with 44% confidence

# 3. Check logs
docker logs binance-trading-bot-dev 2>&1 | grep "SCALP.*Confidence too low.*signal=44.*threshold=50"

# Expected log:
# [SCALP] Confidence too low: signal=44.00%, threshold=50.00% (from DB)

# 4. Verify NO trade executed
curl -X GET http://localhost:8094/api/futures/trades \
  -H "Authorization: Bearer <token>" | jq 'length'

# Expected: 0 (no new trades)
```

### Test 3: Ultra-Fast Mode Confidence
```bash
# 1. Set ultra-fast mode to 45% confidence
curl -X PUT http://localhost:8094/api/autopilot/mode/ultra_fast \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true, "confidence": {"min_confidence": 45.0}}'

# 2. Generate ultra-fast signal with 47% confidence

# 3. Check logs
docker logs binance-trading-bot-dev 2>&1 | grep "ULTRA-FAST.*Confidence passed.*signal=47.*threshold=45"

# Expected: Trade executed with 47% confidence
```

### Test 4: Averaging Decision Uses Mode Config
```bash
# 1. Set scalp averaging confidence to 52%
curl -X PUT http://localhost:8094/api/autopilot/mode/scalp \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "averaging": {
      "min_confidence_for_average": 52.0
    }
  }'

# 2. Create position in loss
# 3. Generate averaging signal with 55% confidence

# 4. Check logs
docker logs binance-trading-bot-dev 2>&1 | grep "AVERAGING.*Confidence passed.*signal=55.*threshold=52"

# Expected: Averaging order placed
```

### Test 5: All Modes Use Different Thresholds
```bash
# 1. Set different confidence for each mode
curl -X PUT http://localhost:8094/api/autopilot/mode/ultra_fast \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true, "confidence": {"min_confidence": 45.0}}'

curl -X PUT http://localhost:8094/api/autopilot/mode/scalp \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true, "confidence": {"min_confidence": 50.0}}'

curl -X PUT http://localhost:8094/api/autopilot/mode/swing \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true, "confidence": {"min_confidence": 60.0}}'

curl -X PUT http://localhost:8094/api/autopilot/mode/position \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true, "confidence": {"min_confidence": 70.0}}'

# 2. Generate signals for each mode
# 3. Verify logs show correct threshold for each mode

# Expected logs:
# [ULTRA-FAST] threshold=45.00% (from DB)
# [SCALP] threshold=50.00% (from DB)
# [SWING] threshold=60.00% (from DB)
# [POSITION] threshold=70.00% (from DB)
```

### Test 6: Confidence Format Standardization
```go
func TestConfidenceFormatStandardization(t *testing.T) {
    tests := []struct {
        name     string
        input    float64
        expected float64
    }{
        {"Already percent format", 45.0, 45.0},
        {"Decimal format", 0.45, 45.0},
        {"100 percent", 1.0, 100.0},
        {"0 percent", 0.0, 0.0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := ConvertConfidenceToPercent(tt.input)
            if result != tt.expected {
                t.Errorf("ConvertConfidenceToPercent(%v) = %v, want %v",
                    tt.input, result, tt.expected)
            }
        })
    }
}
```

---

## Dependencies

### Prerequisites
- **Story 4.4:** mergeWithDefaultConfigs() removed (GetModeConfig returns DB values)
- **Story 4.2:** Repository layer implemented
- **Story 4.3:** API handlers updated
- Database has user_mode_configs table with test data

### Blocks
- **Story 4.6:** Remove hardcoded confidence (natural follow-up after this story)

---

## Deployment Notes

### Development Environment
```bash
# 1. Verify Story 4.4 is complete
# GetModeConfig should return database values without merging

# 2. Make code changes to all 11 locations
# - Add GetModeConfig calls
# - Replace hardcoded thresholds
# - Add logging

# 3. Restart container
./scripts/docker-dev.sh

# 4. Wait for build
sleep 60

# 5. Verify health
curl http://localhost:8094/health

# 6. Run Test 1: Scalp mode confidence threshold (see above)
# 7. Run Test 2: Reject trade below threshold (see above)
# 8. Monitor logs for "(from DB)" indicators
docker logs binance-trading-bot-dev -f | grep "from DB"
```

### Production Environment
1. **Pre-Deployment:**
   - Verify Story 4.4 deployed successfully
   - Backup database
   - Review all 11 code changes
   - Schedule monitoring window (2 hours)

2. **Deployment:**
   - Deploy updated code
   - Restart production containers
   - Monitor logs immediately for errors

3. **Post-Deployment Verification:**
   - Check logs for "(from DB)" in confidence checks
   - Verify trades execute at configured thresholds
   - Monitor for "Failed to get mode config" errors (should be zero)
   - Test with paper trading first for 1 hour
   - Enable live trading after validation

4. **Rollback Procedure (if needed):**
   - Restore previous code version
   - Restart containers
   - Verify service restored

---

## Definition of Done

- [ ] All 11 confidence check locations identified
- [ ] All 11 locations query database for mode config
- [ ] All 11 locations use `modeConfig.Confidence.MinConfidence` (or appropriate field)
- [ ] All 11 locations log mode name and threshold used
- [ ] Hardcoded confidence values removed from decision logic (lines marked for deletion)
- [ ] Confidence format standardized to 0-100 everywhere
- [ ] Conversion helper functions added if needed
- [ ] Code compiles successfully
- [ ] Test 1: Scalp confidence threshold test passes
- [ ] Test 2: Reject trade below threshold test passes
- [ ] Test 3: Ultra-fast confidence test passes
- [ ] Test 4: Averaging decision test passes
- [ ] Test 5: All modes different thresholds test passes
- [ ] Test 6: Confidence format standardization test passes
- [ ] Logs show "(from DB)" for all confidence checks
- [ ] Code review approved
- [ ] Changes tested in development environment
- [ ] Documentation updated

---

## Notes for Developer

### Critical Success Metrics

After this story, the following MUST be true:

1. **User sets scalp to 45%** → System uses 45% (not 50%)
2. **User sets swing to 60%** → System uses 60% (not 50%)
3. **Signal at 47% with 45% threshold** → Trade executes ✅
4. **Signal at 44% with 45% threshold** → Trade rejected ✅
5. **Logs show "(from DB)"** → Confirms database lookup happening

### Common Mistakes to Avoid

- ❌ **DON'T** add fallback to hardcoded value if GetModeConfig fails
- ❌ **DON'T** ignore errors from GetModeConfig
- ❌ **DON'T** mix 0-1 and 0-100 confidence formats
- ✅ **DO** return error if mode config not found
- ✅ **DO** log every confidence check with mode and threshold
- ✅ **DO** standardize confidence format project-wide

### Logging Best Practices

```go
// ❌ BAD: No context
log.Debugf("Confidence check: %.2f", signal.Confidence)

// ✅ GOOD: Full context with source
log.Infof("[%s] Confidence passed: signal=%.2f%%, threshold=%.2f%% (from DB), symbol=%s",
    modeName, signal.Confidence, threshold, symbol)
```

### Performance Considerations

- Database queries for mode config are fast (< 10ms)
- Consider caching mode config per autopilot loop iteration
- Don't query database 11 times per decision - cache per symbol/mode

**Optimization Pattern:**
```go
// Cache mode config at start of decision loop
modeConfigCache := make(map[string]ModeFullConfig)

// Reuse throughout decision process
func getModeCached(modeName string) (ModeFullConfig, error) {
    if config, exists := modeConfigCache[modeName]; exists {
        return config, nil
    }
    config, err := settings.GetModeConfig(userID, modeName)
    if err != nil {
        return ModeFullConfig{}, err
    }
    modeConfigCache[modeName] = config
    return config, nil
}
```

---

## Related Stories

- **Story 4.4:** Remove mergeWithDefaultConfigs (prerequisite)
- **Story 4.6:** Remove hardcoded confidence (natural continuation)
- **Story 4.7:** Frontend mode display (independent, can be parallel)

---

## Approval Sign-Off

- **Scrum Master**: ✅ Story Ready for Development
- **Developer**: _Pending Assignment_
- **Test Architect**: _Pending Test Review_
- **Product Manager**: _Pending Acceptance_
