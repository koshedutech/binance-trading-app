# Claude Code Development Instructions

## Project Overview

- **Project**: Binance Trading Bot with Ginie Autopilot
- **Language**: Go (backend), React/TypeScript (frontend)
- **Database**: PostgreSQL
- **Deployment**: **Docker ONLY** (native mode NOT supported)

---

## Port Configuration

| Environment | Port | Docker Compose File |
|-------------|------|---------------------|
| **Development** | **8094** | `docker-compose.yml` |
| **Production** | **8095** | `docker-compose.prod.yml` |

Both environments can run simultaneously.

---

## Development Workflow

### After Making Code Changes

Simply restart the container - app rebuilds automatically inside:

```bash
./scripts/docker-dev.sh
```

### Development Commands

| Command | Description |
|---------|-------------|
| `./scripts/docker-dev.sh` | Restart container (app rebuilds inside) |
| `./scripts/docker-dev.sh --logs` | View logs only |
| `./scripts/docker-dev.sh --down` | Stop containers |
| `./scripts/docker-dev.sh --build-image` | Rebuild Docker image (rare) |

### Production Commands

| Command | Description |
|---------|-------------|
| `make prod` | Start production (port 8095) |
| `make prod-down` | Stop production |
| `make prod-logs` | View production logs |

### Production Release Management

Versioned releases with rollback capability. Keeps last 3 releases.

| Command | Description |
|---------|-------------|
| `./scripts/prod-release.sh --new "Description"` | Create new release |
| `./scripts/prod-release.sh --list` | List all releases |
| `./scripts/prod-release.sh --info prod-XXX` | Show release details |
| `./scripts/prod-release.sh --rollback prod-XXX` | Rollback (keep current Redis) |
| `./scripts/prod-release.sh --rollback prod-XXX --restore-redis` | Rollback with Redis restore |

**Emergency Recovery:**
```bash
./scripts/prod-release.sh --list            # See available releases
./scripts/prod-release.sh --rollback prod-002  # Restore working version (~60s)
```

See `docs/production-releases.md` for full documentation.

---

## Critical Rules

### After ANY Code Change
1. Run: `./scripts/docker-dev.sh`
2. Wait ~30-60 seconds for build
3. Verify: `curl http://localhost:8094/health`

### Never Do These
- Do NOT run `go run main.go` directly (Go not on host)
- Do NOT rebuild Docker image for code changes
- Use `--build-image` only when Dockerfile changes

### Debugging Rules
**ALWAYS** use Development environment (port 8094) for debugging/testing.
**NEVER** test against Production (port 8095) unless explicitly requested.

---

## CRITICAL: Docker Volume Protection (MANDATORY)

**Docker volumes contain REAL USER DATA (accounts, settings, API keys, trade history). NEVER remove volumes without explicit written permission.**

### Rules - NO EXCEPTIONS

1. **NEVER remove Docker volumes** - even with `dangerouslySkipPermissions: true`
2. **This applies to ALL environments** - Development AND Production
3. **Volume removal destroys:**
   - User accounts and authentication data
   - All configured settings (trading modes, limits, etc.)
   - API keys (Binance, AI providers)
   - Trade history and P&L records
   - All database data

### Before ANY Volume Operation

If you believe volume removal is necessary, you MUST:

1. **STOP and ASK the user first** - Never proceed automatically
2. **STATE the specific reason** - Explain exactly why removal seems necessary
3. **PROPOSE alternatives first** - Database fixes, migrations, manual corrections
4. **WAIT for WRITTEN permission** - User must TYPE explicit confirmation like:
   - "I confirm volume removal for [reason]"
   - A simple "yes" or clicking confirm is NOT sufficient

### Preferred Solutions (Instead of Volume Removal)

| Problem | Solution |
|---------|----------|
| Database schema issues | Write migration scripts |
| Corrupted data | Fix with SQL UPDATE/DELETE |
| Settings conflicts | Reset specific settings via API/SQL |
| Container won't start | Check logs, fix code, NOT volume |
| Migration failures | Debug migration, add missing columns |

### Example Dialogue

```
Claude: I've identified the issue - there's a missing column in the users table.

WRONG: "Let me remove the volume to fix this."
RIGHT: "I'll create a migration to add the missing column. This preserves all your data."
```

**WARNING**: Violating this rule causes irreversible data loss. The user will need to recreate accounts, re-enter all API keys, and lose all historical data.

---

## Authentication

Default admin credentials:
- **Email**: `admin@binance-bot.local`
- **Password**: `Weber@#2025`

**Note**: Dev and Prod use different databases. Users registered in one do NOT exist in the other.

---

## Settings Lifecycle Rule (MANDATORY)

**Any new user-configurable setting MUST follow the complete lifecycle:**

```
default-settings.json → Database → Redis Cache → API → Frontend
```

**Quick Reference:**
1. Add to `default-settings.json`
2. Add to Go struct (`settings.go` or `models_user.go`)
3. Database migration (if new column)
4. Update cache extract/merge in `settings_cache_service.go`
5. Update admin defaults cache
6. API handler with write-through pattern
7. Frontend component with settings comparison

**Full documentation:** `_bmad/bmm/data/settings-lifecycle-rule.md`

**Key Files:**
| Layer | File |
|-------|------|
| Source | `default-settings.json` |
| Cache | `internal/cache/settings_cache_service.go` |
| Cache | `internal/cache/admin_defaults_cache.go` |
| DB | `internal/database/repository_user_mode_config.go` |
| Init | `internal/database/user_initialization.go` |
| API | `internal/api/handlers_settings.go` |

**Redis Keys (per user):** 88 keys (80 mode + 4 global + 4 safety)

---

## Screenshot Reference

When the user says "look at the image" or "check the screenshot", read the image file from:
- **Path**: `/mnt/c/Users/Administrator/Downloads/binance.png`

This is the standard screenshot location for this project.

---

## Summary

> **After ANY code change, run `./scripts/docker-dev.sh`**
