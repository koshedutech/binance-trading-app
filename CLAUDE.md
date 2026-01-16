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

## Summary

> **After ANY code change, run `./scripts/docker-dev.sh`**
