# Epic 9: Remove FuturesController and Consolidate to GinieAutopilot

## Overview

**Epic ID**: epic-9
**Priority**: Medium (gradual migration)
**Estimated Effort**: 52-79 hours (1-2 weeks)
**Risk Level**: High - requires careful incremental approach

## Problem Statement

The codebase has TWO autopilot systems that overlap significantly:
- `FuturesController` (5,291 lines) - Legacy wrapper with infrastructure
- `GinieAutopilot` (17,000+ lines) - Main trading engine

FuturesController was originally a wrapper around GinieAutopilot, but over time:
- GinieAutopilot grew to handle most trading logic directly
- FuturesController became a pass-through layer with some unique infrastructure
- This creates confusion, maintenance burden, and potential bugs

## Goal

Consolidate all futures trading functionality into `GinieAutopilot` and remove `FuturesController` completely.

## Current Architecture

```
┌─────────────────────────────────────────────────────────┐
│ UserAutopilotManager                                     │
│   Creates both FuturesController AND GinieAutopilot     │
└───────────────────┬─────────────────────────────────────┘
                    │
        ┌───────────┴───────────┐
        ▼                       ▼
┌───────────────────┐   ┌───────────────────┐
│ FuturesController │   │ GinieAutopilot    │
│ (5,291 lines)     │   │ (17,000+ lines)   │
│                   │   │                   │
│ - Position Track  │   │ - Signal Analysis │
│ - Profit/Risk     │   │ - Trade Execution │
│ - WebSocket       │   │ - Mode Management │
│ - TP/SL Mgmt     │   │ - LLM Integration │
│ - Wrapper calls   │──▶│ - Pattern Trading │
└───────────────────┘   └───────────────────┘
```

## Target Architecture

```
┌─────────────────────────────────────────────────────────┐
│ UserAutopilotManager                                     │
│   Creates ONLY GinieAutopilot                           │
└───────────────────┬─────────────────────────────────────┘
                    │
                    ▼
        ┌───────────────────────────────────────┐
        │ GinieAutopilot (Consolidated)         │
        │ - All trading logic                   │
        │ - All position management             │
        │ - All infrastructure                  │
        └───────────────────────────────────────┘
```

## FuturesController Unique Functionality to Migrate

| Category | Lines | Complexity | Story |
|----------|-------|------------|-------|
| Position Tracking | ~400 | Medium | STORY-003 |
| Profit/Risk Management | ~300 | Medium | STORY-004 |
| WebSocket Handlers | ~400 | Medium | STORY-005 |
| TP/SL Management | ~1,800 | HIGH | STORY-006 |
| Helper/Utility Methods | ~500 | Low | STORY-002 |
| Constructor/Setup | ~200 | Low | STORY-007 |

---

## Stories (Ordered by Criticality - Safest First)

### STORY-001: Audit and Document All FuturesController Dependencies [SAFE]
**Priority**: 1 (Do First)
**Risk**: None
**Effort**: 4 hours

**Acceptance Criteria**:
- [ ] List all API handlers that call FuturesController methods
- [ ] List all methods in FuturesController that are NOT in GinieAutopilot
- [ ] Document which GinieAutopilot methods already exist as equivalents
- [ ] Create mapping table: FC method → GA equivalent (or "needs migration")

**Why Safe**: Pure analysis, no code changes.

---

### STORY-002: Migrate Helper/Utility Methods [LOW RISK]
**Priority**: 2
**Risk**: Low
**Effort**: 4 hours

**Acceptance Criteria**:
- [ ] Identify helper methods in FC that GA doesn't have
- [ ] Copy helpers to GinieAutopilot (don't delete from FC yet)
- [ ] Update callers to use GA helpers
- [ ] Test that Ginie still works

**Methods to migrate**:
- `formatCurrency()`
- `calculatePercentage()`
- `validateSymbol()`
- Other utility functions

**Why Low Risk**: Adding methods, not removing. FC still works.

---

### STORY-003: Migrate Position Tracking Infrastructure [MEDIUM RISK]
**Priority**: 3
**Risk**: Medium
**Effort**: 8 hours

**Acceptance Criteria**:
- [ ] Copy position tracking structs to GA
- [ ] Migrate `trackPosition()`, `getTrackedPositions()`
- [ ] Migrate position state management
- [ ] Update API handlers one-by-one
- [ ] Test position display still works

**Why Medium Risk**: Position tracking affects UI display. Test thoroughly.

---

### STORY-004: Migrate Profit/Risk Management [MEDIUM RISK]
**Priority**: 4
**Risk**: Medium
**Effort**: 8 hours

**Acceptance Criteria**:
- [ ] Migrate profit calculation methods
- [ ] Migrate risk assessment logic
- [ ] Migrate daily P&L tracking
- [ ] Update API handlers
- [ ] Test profit displays correctly

**Why Medium Risk**: Affects P&L calculations. Verify numbers match.

---

### STORY-005: Migrate WebSocket Handlers [MEDIUM RISK]
**Priority**: 5
**Risk**: Medium
**Effort**: 8 hours

**Acceptance Criteria**:
- [ ] Migrate WebSocket connection management
- [ ] Migrate real-time update handlers
- [ ] Migrate reconnection logic
- [ ] Test real-time updates work

**Why Medium Risk**: WebSocket issues cause silent failures. Test live updates.

---

### STORY-006: Migrate TP/SL Management [HIGH RISK - CRITICAL]
**Priority**: 6
**Risk**: HIGH
**Effort**: 16 hours

**Acceptance Criteria**:
- [ ] Migrate all TP/SL calculation logic (~1,800 lines)
- [ ] Migrate algo order management
- [ ] Migrate position closing logic
- [ ] Migrate trailing stop logic
- [ ] EXTENSIVE testing before deploying

**Why High Risk**: This is the LARGEST component. Incorrect TP/SL = lost money. Do this LAST and test extensively.

**Sub-tasks**:
1. Migrate TP level calculation (TP1, TP2, TP3, TP4)
2. Migrate SL calculation
3. Migrate trailing stop activation
4. Migrate partial position closing
5. Migrate recalc-sltp endpoint
6. Test all scenarios in paper mode first

---

### STORY-007: Update API Handlers to Use GinieAutopilot [MEDIUM RISK]
**Priority**: 7
**Risk**: Medium
**Effort**: 12 hours

**Acceptance Criteria**:
- [ ] Update each handler in `handlers_ginie.go` (60+ handlers)
- [ ] Change `fc.Method()` to `ga.Method()`
- [ ] Test each endpoint after update
- [ ] Ensure backward compatibility

**Handler files to update**:
- `internal/api/handlers_ginie.go`
- `internal/api/handlers_futures.go`
- `internal/api/handlers_settings.go`

---

### STORY-008: Update UserAutopilotManager [HIGH RISK]
**Priority**: 8
**Risk**: High
**Effort**: 8 hours

**Acceptance Criteria**:
- [ ] Remove FuturesController creation from manager
- [ ] Update all references to use GinieAutopilot
- [ ] Update multi-user support
- [ ] Test with multiple concurrent users

**Why High Risk**: Affects all user sessions. Must test multi-user scenarios.

---

### STORY-009: Remove FuturesController File [FINAL]
**Priority**: 9 (Do Last)
**Risk**: Low (if previous stories complete)
**Effort**: 2 hours

**Acceptance Criteria**:
- [ ] Delete `futures_controller.go`
- [ ] Remove all imports
- [ ] Clean up any remaining references
- [ ] Full system test
- [ ] Deploy to production

**Why Last**: Only delete when everything else is migrated and tested.

---

## Testing Strategy

### Per-Story Testing
1. Run unit tests after each migration
2. Test in paper mode before live
3. Verify API responses match before/after
4. Check UI displays correctly

### Full System Testing (Before STORY-009)
1. Create test positions in paper mode
2. Verify TP/SL orders placed correctly
3. Verify position tracking works
4. Verify profit calculations match
5. Verify WebSocket updates work
6. Test with multiple users simultaneously

---

## Rollback Plan

Each story should be:
1. Committed separately
2. Deployable independently
3. Reversible by git revert

If any story breaks Ginie:
1. `git revert <commit>`
2. Redeploy
3. Investigate root cause
4. Fix and retry

---

## Dependencies

- **Prerequisite**: Fix database-first issue (Option A) BEFORE starting this epic
- **Prerequisite**: Safety commit created (`7ad58953`)

---

## Timeline (Suggested)

| Week | Stories | Risk |
|------|---------|------|
| Week 1 | STORY-001, STORY-002, STORY-003 | Low-Medium |
| Week 2 | STORY-004, STORY-005 | Medium |
| Week 3 | STORY-006 (TP/SL - most complex) | High |
| Week 4 | STORY-007, STORY-008, STORY-009 | Medium-High |

---

## Success Metrics

- [ ] All API endpoints work identically before/after
- [ ] No regression in trading functionality
- [ ] Position sizing uses correct database values
- [ ] TP/SL orders placed correctly
- [ ] Real-time updates work
- [ ] Multi-user support maintained
- [ ] Codebase reduced by ~5,000 lines (FuturesController removed)

---

## Notes

- **Created**: 2025-01-10
- **Author**: Claude Code
- **Safety Commit**: `7ad58953` (checkpoint before this work)
- **Status**: PLANNED - Waiting for Option A completion first
