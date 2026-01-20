# Story 9.11: Migrate Symbol Blocking to Database

## Story Information
- **Epic**: Epic 9 - Remove FuturesController and Consolidate to GinieAutopilot
- **Story ID**: 9.11
- **Priority**: High
- **Estimated Effort**: Medium (6-8 hours)
- **Created**: 2026-01-20
- **Status**: Done

---

## Problem Statement

### Current State
Symbol blocking (temporary ban of symbols from trading) is stored in the file-based `autopilot_settings.json`:

```go
sm := autopilot.GetSettingsManager()
sm.BlockSymbolForDay(symbol, reason)
sm.UnblockSymbol(symbol)
sm.GetAllBlockedSymbols()
sm.IsSymbolBlocked(symbol)
```

### Issues
1. **No Multi-User Support**: All users share the same blocked symbols
2. **No TTL Management**: Blocks with expiration stored in file
3. **No Persistence on Restart**: Redis would be better for TTL-based blocks
4. **Inconsistent with Architecture**: Should use DB+Redis like other settings

### Files Using Symbol Blocking

| File | Lines | Operations |
|------|-------|------------|
| `handlers_ginie.go` | 3681, 3705, 3722, 3746, 3764, 3783 | Block, Unblock, GetAll, AutoBlock |
| `ginie_autopilot.go` | Multiple | IsSymbolBlocked checks during trading |

---

## Solution: Create User Symbol Blocks Table + Redis Cache

### Database Schema

```sql
CREATE TABLE user_blocked_symbols (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(36) NOT NULL REFERENCES users(id),
    symbol VARCHAR(20) NOT NULL,
    reason VARCHAR(255),
    blocked_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,  -- NULL = permanent
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, symbol)
);

CREATE INDEX idx_blocked_symbols_user ON user_blocked_symbols(user_id);
CREATE INDEX idx_blocked_symbols_expires ON user_blocked_symbols(expires_at) WHERE expires_at IS NOT NULL;
```

### Redis Cache Pattern

```
blocked:{userID}:{symbol} -> {reason, expires_at}  (TTL auto-expires)
blocked_list:{userID} -> SET of symbols (for fast lookup)
```

---

## Technical Specification

### Task 1: Create Database Migration

Add migration file for `user_blocked_symbols` table.

### Task 2: Create Repository Methods

**File**: `internal/database/repository_user_blocked_symbols.go`

```go
type UserBlockedSymbol struct {
    ID        string
    UserID    string
    Symbol    string
    Reason    string
    BlockedAt time.Time
    ExpiresAt *time.Time  // nil = permanent
}

func (r *Repository) BlockSymbol(ctx, userID, symbol, reason string, duration *time.Duration) error
func (r *Repository) UnblockSymbol(ctx, userID, symbol string) error
func (r *Repository) GetBlockedSymbols(ctx, userID string) ([]UserBlockedSymbol, error)
func (r *Repository) IsSymbolBlocked(ctx, userID, symbol string) (bool, string, error)
func (r *Repository) CleanExpiredBlocks(ctx context.Context) error
```

### Task 3: Update API Handlers

**File**: `internal/api/handlers_ginie.go`

Replace:
```go
sm := autopilot.GetSettingsManager()
sm.BlockSymbolForDay(symbol, reason)
```

With:
```go
duration := 24 * time.Hour
s.repo.BlockSymbol(ctx, userID, symbol, reason, &duration)
```

### Task 4: Update GinieAutopilot Symbol Checks

**File**: `internal/autopilot/ginie_autopilot.go`

Replace `IsSymbolBlocked` calls with repository method.

---

## Acceptance Criteria

### AC9.11.1: Database Table Created
- [x] Migration creates `user_blocked_symbols` table
- [x] Proper indexes for user_id and expires_at

### AC9.11.2: Repository Methods Implemented
- [x] BlockSymbol with optional duration
- [x] UnblockSymbol removes from DB
- [x] GetBlockedSymbols returns user's blocks
- [x] IsSymbolBlocked checks expiration
- [x] CleanExpiredBlocks removes stale entries

### AC9.11.3: API Handlers Migrated
- [x] handleBlockSymbol uses DB
- [x] handleUnblockSymbol uses DB
- [x] handleGetBlockedSymbols uses DB
- [x] handleAutoBlockWorstPerformers uses DB

### AC9.11.4: Multi-User Isolation
- [x] Each user has separate blocked symbols
- [x] Block/unblock operations isolated by userID

### AC9.11.5: Expiration Works
- [x] Time-based blocks auto-expire
- [x] Expired blocks not returned in queries

---

## Implementation Plan

1. Create database migration
2. Create repository with CRUD methods
3. Update handlers_ginie.go (6 handlers)
4. Update ginie_autopilot.go symbol checks
5. Test multi-user isolation

---

## Dependencies

- Story 9.10 (Settings Migration Infrastructure) - COMPLETE

---

## Notes

This story focuses specifically on symbol blocking. Other SettingsManager usages (mode configs, confluence settings) will be addressed in subsequent stories.
