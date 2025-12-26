# Claude Code Development Instructions

This file contains **MANDATORY** instructions for Claude Code when working on this project.

## Project Overview

- **Project**: Binance Trading Bot with Ginie Autopilot
- **Language**: Go (backend), React/TypeScript (frontend)
- **Database**: PostgreSQL
- **Deployment**: **Docker ONLY** (native mode is NOT supported)

---

## Multi-Agent Development Approach (MANDATORY)

**CRITICAL**: For ANY development task, you MUST use the **multi-agent and sub-agent approach** wherever possible:

### When to Use Multi-Agents
- **Code Analysis**: Use `Explore` agent for codebase investigation
- **Implementation Planning**: Use `Plan` agent for architectural decisions
- **Complex Features**: Launch parallel agents for independent components
- **Research Tasks**: Use `general-purpose` agent for deep research
- **Story/Epic Creation**: Use BMAD agents (analyst, architect, dev, pm, sm)

### How to Apply
1. **Break down tasks** into parallelizable sub-tasks
2. **Launch multiple agents simultaneously** when tasks are independent
3. **Use specialized agents** for their domain expertise
4. **Coordinate results** from multiple agents for comprehensive solutions

### Examples
```
User: "Implement feature X"
→ Launch Plan agent for architecture
→ Launch Explore agent for existing code patterns
→ Synthesize findings and implement

User: "Fix bug in module Y"
→ Launch Explore agent to find related code
→ Analyze root cause with multiple file reads in parallel
→ Implement fix with proper testing
```

### Benefits
- Faster analysis through parallelization
- More thorough investigation
- Reduced context usage per agent
- Specialized expertise per task

---

## Port Configuration

| Environment | Port | Docker Compose File | Container Name |
|-------------|------|---------------------|----------------|
| **Development** | **8094** | `docker-compose.yml` | `binance-trading-bot-dev` |
| **Production** | **8095** | `docker-compose.prod.yml` | `binance-trading-bot-prod` |

Both environments can run simultaneously on different ports.

---

## Development Workflow

### Key Principle: Docker Image Built ONCE

The development Docker image is built **once** and reused. It contains:
- Go compiler
- Node.js and npm
- All build tools

Source code is **mounted via volumes**. The container builds the app on startup.

### After Making Code Changes

Simply restart the container - the app rebuilds automatically inside:

```bash
./scripts/docker-dev.sh
```

Or use Make:

```bash
make dev
```

### Development Commands

| Command | Description |
|---------|-------------|
| `./scripts/docker-dev.sh` | Restart container (app rebuilds inside) |
| `./scripts/docker-dev.sh --logs` | View logs only |
| `./scripts/docker-dev.sh --down` | Stop containers |
| `./scripts/docker-dev.sh -d` | Start in detached mode |
| `./scripts/docker-dev.sh --build-image` | Rebuild Docker image (rare) |

### What Happens on Container Start

1. Frontend builds (`npm install && npm run build`)
2. Go app builds (`go build -o trading-bot main.go`)
3. App starts

---

## Production Commands

| Command | Description |
|---------|-------------|
| `make prod` | Start production (port 8095) |
| `make prod-down` | Stop production containers |
| `make prod-logs` | View production logs |

---

## Claude Code Workflow Checklist

### After Completing ANY Code Change
- [ ] Run: `./scripts/docker-dev.sh`
- [ ] Wait for container to build app (~30-60 seconds)
- [ ] Verify: `curl http://localhost:8094/health`

### Never Do These
- ❌ Do NOT run `go run main.go` directly (Go not on host)
- ❌ Do NOT rebuild Docker image for code changes
- ❌ Use `--build-image` only when Dockerfile changes

---

## Environment Variables

Environment variables are configured in:
- **Development**: `docker-compose.yml` → `environment:` section
- **Production**: `docker-compose.prod.yml` → `environment:` section

---

## File Structure

### Backend (Go)
- `main.go` - Application entry point
- `internal/api/` - HTTP handlers and routes
- `internal/autopilot/` - Ginie autopilot logic
- `internal/binance/` - Binance API clients
- `internal/database/` - PostgreSQL repositories
- `internal/auth/` - Authentication service

### Frontend (React/TypeScript)
- `web/src/components/` - React components
- `web/src/pages/` - Page components
- `web/src/services/` - API client services
- `web/src/contexts/` - React contexts

### Docker
- `docker-compose.yml` - Development (port 8094)
- `docker-compose.prod.yml` - Production (port 8095)
- `Dockerfile` - Production multi-stage build
- `Dockerfile.dev` - Development image with build tools
- `scripts/docker-dev.sh` - Dev workflow script

---

## Authentication

Default admin credentials:
- **Email**: `admin@binance-bot.local`
- **Password**: `Weber@#2025`

Admin panel: Navigate to `/admin` when logged in as admin.

---

## User Accounts for Testing

**IMPORTANT**: Do NOT use the admin account for trading or API testing. The admin account is for administrative tasks only.

For testing trading features, API endpoints, or Ginie autopilot:
1. **Development** (port 8094): Use `jejeram@gmail.com` with password `password123`
2. **Production** (port 8095): Same credentials

**Why separate?** Dev and Prod use **different PostgreSQL databases**:
- Dev: `binance-bot-postgres` (port 5433)
- Prod: `binance-bot-postgres-prod`

Users registered in one environment do NOT exist in the other.

---

## Debugging Rules

**CRITICAL**: For ALL debugging and testing, ALWAYS use the **Development environment** (port 8094):
- API testing: `http://localhost:8094/api/...`
- Health checks: `http://localhost:8094/health`
- Login testing: Use development database

**NEVER** debug or test against the Production environment (port 8095) unless explicitly requested by the user.

---

## Testing Changes

```bash
# Health check
curl http://localhost:8094/health

# Admin login test
curl -X POST http://localhost:8094/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@binance-bot.local","password":"Weber@#2025"}'
```

---

## Summary: The Golden Rule

> **After ANY code change, run `./scripts/docker-dev.sh`**

This restarts the container, which builds and runs the app inside.
**No Docker image rebuild needed for code changes!**
