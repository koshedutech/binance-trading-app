# Epic 4: Database-First Mode Configuration System

## Epic Overview

**Goal:** Establish database as the single source of truth for all mode configurations, eliminating JSON file dependencies and hardcoded values that cause settings to be overwritten on restart.

**Business Value:** User-configured settings (confidence thresholds, mode preferences, etc.) persist correctly across server restarts, enabling true per-user customization.

**Priority:** HIGH - Core configuration reliability

**Estimated Complexity:** HIGH

**Status:** COMPLETE

---

## Problem Statement

### Root Cause (Fixed)

The system had **THREE conflicting sources of truth**:

| Source | Location | Issue |
|--------|----------|-------|
| 1. JSON File | `autopilot_settings.json` | Overwrites database on startup |
| 2. Hardcoded Defaults | `settings.go` | Scattered across 20+ functions |
| 3. Database | `user_mode_configs` table | Gets overwritten by JSON sync |

### Impact (Resolved)

- User disables mode → Server restarts → Mode re-enabled from JSON
- User sets 45% confidence → Trade executes at hardcoded 50%
- Admin changes defaults → No effect on existing users

---

## Target State (Achieved)

```
IMPLEMENTED STATE:
┌─────────────────────────────────────────────────────────────┐
│ SINGLE SOURCE OF TRUTH: PostgreSQL Database                 │
├─────────────────────────────────────────────────────────────┤
│ ✅ Mode configs from user_mode_configs table                │
│ ✅ Confidence thresholds from database (not hardcoded)      │
│ ✅ JSON file serves ONLY as default template                │
│ ✅ New users get defaults loaded from JSON → DB             │
│ ✅ Admin can sync defaults without overwriting user configs │
│ ✅ Settings comparison view shows user vs default           │
│ ✅ Comprehensive reset functionality per section            │
└─────────────────────────────────────────────────────────────┘
```

---

## Stories

| Story | Title | Status |
|-------|-------|--------|
| 4.5 | Apply Per-Mode Confidence to Trade Decisions | Done |
| 4.6 | Remove Hardcoded Confidence Values | Done |
| 4.11 | DB-First Mode Enabled Status | Done |
| 4.12 | Position Optimizer DB Migration | Done |
| 4.13 | Default Settings JSON Foundation | Done |
| 4.14 | New User Load Defaults | Done |
| 4.15 | Admin Settings Sync | Done |
| 4.16 | Settings Comparison & Risk Display | Done |
| 4.17 | Comprehensive Reset Functionality | Done |
| 4.x | Frontend UI Updates | Done |

---

## Key Deliverables (Completed)

1. **Database-First Architecture** - All mode configs read from PostgreSQL
2. **JSON as Template Only** - `default_settings.json` for new user initialization
3. **Per-Mode Confidence** - Trade decisions use user-configured thresholds
4. **Settings Comparison UI** - Users see their settings vs admin defaults
5. **Reset Functionality** - Individual and bulk reset to defaults
6. **Admin Sync** - Admins can update defaults without affecting existing users

---

## Correlates With

- **Epic 5:** Continues DB wiring for remaining configs (scalp reentry, circuit breakers)
- **Epic 6:** Redis caching layer for DB-sourced settings
