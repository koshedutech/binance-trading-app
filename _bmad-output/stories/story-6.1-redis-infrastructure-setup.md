# Story 6.1: Redis Infrastructure Setup
**Epic:** Epic 6: Redis Caching Infrastructure
**Sprint:** Sprint 6
**Story Points:** 3
**Priority:** P0

## User Story
As a system administrator, I want Redis infrastructure deployed with persistence so that the trading bot can cache settings and maintain state across container restarts.

## Acceptance Criteria
- [ ] Redis 7 Alpine container added to docker-compose.yml
- [ ] AOF (Append Only File) persistence enabled
- [ ] Named volume for Redis data (redis_data)
- [ ] Health check endpoint configured
- [ ] Go Redis client (go-redis/redis) integrated
- [ ] Connection pool with configurable size
- [ ] Graceful reconnection on connection loss
- [ ] Environment variables for Redis config (host, port, password)

## Technical Approach

### Docker Infrastructure
Add Redis service to both docker-compose.yml and docker-compose.prod.yml with:
- Image: redis:7-alpine
- Container name: binance-bot-redis
- Port: 6379 exposed
- Volume: redis_data mounted to /data
- Command: `redis-server --appendonly yes --appendfsync everysec --maxmemory 512mb --maxmemory-policy noeviction`
- Health check: `redis-cli ping` every 10s
- Network: trading-network
- Restart policy: unless-stopped

### Go Redis Client Integration
- Install go-redis/redis v9 library
- Create internal/cache/redis.go with:
  - RedisClient struct with connection pool
  - Connection configuration from environment variables
  - Graceful reconnection logic with exponential backoff
  - Health check method
  - Ping/Pong validation
- Environment variables:
  - REDIS_HOST (default: redis)
  - REDIS_PORT (default: 6379)
  - REDIS_PASSWORD (optional)
  - REDIS_DB (default: 0)
  - REDIS_POOL_SIZE (default: 10)

### Persistence Configuration
- AOF enabled for durability
- appendfsync everysec balances performance and durability
- maxmemory 512mb with noeviction policy prevents data loss
- Named volume ensures data survives container recreation

## Dependencies
- **Blocked By:** None (infrastructure foundation)
- **Blocks:** Stories 6.2-6.9 (all require Redis infrastructure)

## Files to Create/Modify
- `docker-compose.yml` - Add Redis service and volume
- `docker-compose.prod.yml` - Add Redis service and volume
- `internal/cache/redis.go` - Redis client initialization and connection pool
- `internal/cache/health.go` - Redis health check implementation
- `go.mod` - Add go-redis/redis v9 dependency
- `main.go` - Initialize Redis client on startup

## Testing Requirements

### Unit Tests
- Test Redis client initialization with valid config
- Test Redis client initialization with invalid config (expect error)
- Test connection pool creation
- Test health check ping/pong

### Integration Tests
- Test Redis container starts successfully
- Test health check endpoint returns healthy status
- Test connection from Go app to Redis container
- Test AOF persistence (restart container, verify data retained)
- Test connection reconnection after Redis restart
- Test graceful handling when Redis is unavailable

### Performance Tests
- Verify connection pool handles 100 concurrent connections
- Test Redis response time <1ms for basic operations

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Redis container running in dev and prod environments
- [ ] Health check endpoint verified
- [ ] Connection pool tested under load
- [ ] Documentation updated (README with Redis setup instructions)
- [ ] PO acceptance received
