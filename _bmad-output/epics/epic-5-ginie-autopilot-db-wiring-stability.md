# Epic 5: Ginie Autopilot Database Wiring & UI Stability

## Epic Overview

**Goal:** Complete database-first architecture for Ginie Autopilot by wiring all remaining configurations to database, and fix critical UI stability issues that cause the Ginie panel to enter infinite loading state.

**Business Value:** Enable per-user customization of ALL Ginie Autopilot settings, eliminate hardcoded values that prevent multi-tenant flexibility, and ensure stable user experience with proper error handling.

**Priority:** CRITICAL - Trading system stability and multi-tenant configuration

**Estimated Complexity:** HIGH

---

## Problem Statement

### Current Issues Discovered

| Issue | Severity | Impact |
|-------|----------|--------|
| **Ginie Panel infinite loading** | CRITICAL | UI becomes unusable, users cannot access trading |
| **Silent 401/403 error handling** | CRITICAL | Token expiry causes silent failure, no recovery |
| **Scalp Reentry uses JSON file** | HIGH | No per-user customization, shared global config |
| **Global Circuit Breaker hardcoded** | HIGH | Cannot customize per-user safety limits |
| **503 errors cascade to UI** | HIGH | Backend autopilot init failure breaks entire panel |
| **No error retry in frontend** | MEDIUM | Users must manually refresh on transient errors |

### Architecture Gap Analysis

```
CURRENT STATE (Broken):
┌─────────────────────────────────────────────────────────────┐
│ MODE CONFIGS (scalp, swing, position)                       │
│ ├── user_mode_configs table (PostgreSQL)        ✅ DATABASE │
│ ├── Hedge settings                              ✅ DATABASE │
│ ├── Mode circuit breakers                       ✅ DATABASE │
│ ├── SLTP settings                               ✅ DATABASE │
│ └── Confidence thresholds                       ✅ DATABASE │
├─────────────────────────────────────────────────────────────┤
│ SCALP REENTRY CONFIG                                        │
│ ├── autopilot_settings.json (file)              ❌ JSON FILE│
│ ├── TP1/2/3 percentages                         ❌ JSON FILE│
│ ├── Hedge mode settings                         ❌ JSON FILE│
│ ├── DCA settings                                ❌ JSON FILE│
│ └── 36+ other settings                          ❌ JSON FILE│
├─────────────────────────────────────────────────────────────┤
│ GLOBAL CIRCUIT BREAKER                                      │
│ ├── MaxLossPerHour = $100                       ❌ HARDCODED│
│ ├── MaxDailyLoss = $300                         ❌ HARDCODED│
│ ├── MaxConsecutiveLosses = 3                    ❌ HARDCODED│
│ └── CooldownMinutes = 30                        ❌ HARDCODED│
├─────────────────────────────────────────────────────────────┤
│ FRONTEND ERROR HANDLING                                     │
│ ├── 401/403 errors silently ignored             ❌ BUG      │
│ ├── 503 errors not displayed                    ❌ BUG      │
│ ├── No retry buttons                            ❌ MISSING  │
│ └── Status stuck in "Loading..."                ❌ BUG      │
└─────────────────────────────────────────────────────────────┘
```

---

## Target State

### Complete Database-First Architecture

```
TARGET STATE:
┌─────────────────────────────────────────────────────────────┐
│ ALL CONFIGS → user_mode_configs TABLE (PostgreSQL)          │
├─────────────────────────────────────────────────────────────┤
│ MODE CONFIGS                                    ✅ DATABASE │
│ SCALP REENTRY CONFIG                            ✅ DATABASE │
│ GLOBAL CIRCUIT BREAKER                          ✅ DATABASE │
│ HEDGE MODE                                      ✅ DATABASE │
├─────────────────────────────────────────────────────────────┤
│ FRONTEND                                                    │
│ ├── Proper error states per section             ✅ FIXED   │
│ ├── Retry buttons on failure                    ✅ FIXED   │
│ ├── Auth expiry detection                       ✅ FIXED   │
│ └── Graceful degradation                        ✅ FIXED   │
└─────────────────────────────────────────────────────────────┘
```

### Key Principles

1. **DATABASE-FIRST** - All settings come from PostgreSQL, JSON file only for bootstrap
2. **PER-USER ISOLATION** - Each user has independent configuration
3. **EXPLICIT ERROR HANDLING** - Every API failure shown to user with retry option
4. **GRACEFUL DEGRADATION** - Partial failures don't break entire panel
5. **NO HARDCODED VALUES** - All limits configurable per-user

---

## Requirements Traceability

### Functional Requirements

| ID | Requirement | Stories |
|----|-------------|---------|
| FR-1 | Ginie Panel must show error messages instead of infinite loading | 5.1 |
| FR-2 | 401/403 errors must show "Session expired" with re-login option | 5.1 |
| FR-3 | 503 errors must show specific error message with retry button | 5.1 |
| FR-4 | Scalp Reentry config must load from database | 5.2 |
| FR-5 | Scalp Reentry config must be per-user customizable | 5.2 |
| FR-6 | Global Circuit Breaker must load from database | 5.3 |
| FR-7 | Global Circuit Breaker must be per-user customizable | 5.3 |
| FR-8 | Each panel section must have independent error state | 5.1 |

### Non-Functional Requirements

| ID | Requirement | Stories |
|----|-------------|---------|
| NFR-1 | Panel must recover from errors within 10 seconds with retry | 5.1 |
| NFR-2 | Database config load must complete within 500ms | 5.2, 5.3 |
| NFR-3 | Fallback to defaults if database config missing | 5.2, 5.3 |

---

## Stories

### Story 5.1: Fix Ginie Panel Loading & Error Handling (CRITICAL)
- Fix infinite loading state when API calls fail
- Add proper error states for each panel section
- Add retry buttons for failed API calls
- Handle 401/403 with session expiry message
- Handle 503 with specific error + retry

### Story 5.2: Wire Scalp Reentry Config to Database
- Add scalp_reentry to user_mode_configs schema
- Migrate existing JSON config to database
- Update SettingsManager to load from database
- Add API handlers for scalp_reentry config CRUD
- Update frontend to use new API endpoints

### Story 5.3: Wire Global Circuit Breaker to Database
- Add global_circuit_breaker to user settings
- Remove hardcoded values in DefaultGinieAutopilotConfig()
- Load from database with default fallback
- Add API handlers for global CB config
- Update frontend to allow customization

---

## Dependencies

| Dependency | Type | Status |
|------------|------|--------|
| Epic 4 - Database-First Mode Config | Prerequisite | Complete |
| user_mode_configs table | Database | Exists |
| Authentication system | System | Working |

---

## Success Criteria

1. **Loading Issue Fixed**: Ginie Panel never shows infinite loading - always shows error or data
2. **Error Recovery**: Users can retry failed API calls without page refresh
3. **Auth Handling**: Token expiry shows clear message and redirect to login
4. **Scalp Reentry**: Per-user customization works, changes persist across sessions
5. **Global CB**: Per-user limits work, users can adjust their own safety limits
6. **Multi-User**: Two users can have completely different Scalp Reentry and CB settings

---

## Technical Considerations

### Database Changes
- Extend user_mode_configs for scalp_reentry mode
- Add user_global_settings table for global CB
- Migration scripts for existing data

### API Changes
- GET/PUT /api/futures/ginie/scalp-reentry-config
- GET/PUT /api/user/global-circuit-breaker
- Error response standardization

### Frontend Changes
- Per-section error states in GiniePanel
- ErrorBoundary for section isolation
- Retry logic with exponential backoff

---

## Affected Files

### Backend
- internal/autopilot/settings.go
- internal/autopilot/scalp_reentry_types.go
- internal/autopilot/ginie_autopilot.go
- internal/api/handlers_ginie.go
- internal/database/repository_user_mode_config.go

### Frontend
- web/src/components/GiniePanel.tsx
- web/src/services/futuresApi.ts

### Database
- New migration for scalp_reentry support
- New migration for global_circuit_breaker

---

## Author

**Created By:** BMAD Agent (Mary - Business Analyst)
**Date:** 2026-01-06
**Version:** 1.0
