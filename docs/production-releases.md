# Production Release Management

Quick reference for managing production releases with rollback capability.

## Quick Commands

| Command | Description |
|---------|-------------|
| `./scripts/prod-release.sh --new "Description"` | Create new release |
| `./scripts/prod-release.sh --list` | List all releases |
| `./scripts/prod-release.sh --info prod-XXX` | Show release details |
| `./scripts/prod-release.sh --rollback prod-XXX` | Rollback (keep Redis) |
| `./scripts/prod-release.sh --rollback prod-XXX --restore-redis` | Rollback with Redis |

## Emergency Recovery

If production breaks:

```bash
# 1. See available releases
./scripts/prod-release.sh --list

# 2. Pick a working release and rollback
./scripts/prod-release.sh --rollback prod-002

# 3. Trading resumes in ~60 seconds
```

## Creating a New Release

Before deploying new code to production:

```bash
# 1. Test changes in development first
./scripts/docker-dev.sh

# 2. Verify dev is working
curl http://localhost:8094/health

# 3. Create production release with description
./scripts/prod-release.sh --new "Story 9.5 - Trend Filters"

# 4. Verify production
curl http://localhost:8095/health
```

## What Gets Preserved

Each release includes:

| Component | Description |
|-----------|-------------|
| Docker Image | `binance-trading-bot:prod-XXX` |
| PostgreSQL | Full database dump |
| Redis | Shared cache snapshot |
| Configs | `default-settings.json`, `autopilot_settings.json` |

## When to Restore Redis

| Scenario | Restore Redis? | Command |
|----------|----------------|---------|
| Code bug, cache unchanged | NO | `--rollback prod-XXX` |
| Cache key structure changed | YES | `--rollback prod-XXX --restore-redis` |
| Unknown issue | Try NO first | `--rollback prod-XXX` |

**Note:** Redis is shared between dev and prod. Restoring Redis will affect both environments!

## Release Retention

- **Maximum 3 releases** are kept
- Oldest releases auto-deleted when creating new ones
- Both Docker images and backup folders are cleaned up

## Example Workflow

```bash
# Week 1: Initial stable release
./scripts/prod-release.sh --new "Initial stable release"

# Week 2: Add trend filters
./scripts/prod-release.sh --new "Story 9.5 - Trend Filters"

# Week 3: Add position optimizer
./scripts/prod-release.sh --new "Story 9.6 - Position Optimizer"

# Week 3: Something broke!
./scripts/prod-release.sh --list
# Shows: prod-003 (current), prod-002, prod-001

./scripts/prod-release.sh --rollback prod-002
# Back to trend filters version in ~60 seconds
```

## File Locations

| Path | Description |
|------|-------------|
| `releases/` | All release backups |
| `releases/manifest.json` | Release index |
| `releases/prod-XXX/` | Individual release folder |
| `releases/prod-XXX/release-info.json` | Release metadata |
| `releases/prod-XXX/volumes/` | Volume backups |
| `releases/prod-XXX/configs/` | Config backups |

## Troubleshooting

### "Release not found"
Check available releases with `--list`

### "PostgreSQL backup not found"
The release may not have volume backups (first release before volumes existed)

### "Health check not confirmed"
Wait a bit longer and check manually: `curl http://localhost:8095/health`

### Rollback failed
Check Docker logs: `docker-compose -f docker-compose.prod.yml logs`
