# Sprint Plan: Epics 6, 7, 8
## Redis Caching, Client Order ID, and Daily Settlement

**Created By:** Bob the Scrum Master (BMAD Party Mode)
**Date:** 2026-01-06
**Version:** 1.0
**Sprint Duration:** 2-week sprints
**Epics Covered:**
- Epic 6: Redis Caching Infrastructure
- Epic 7: Client Order ID & Trade Lifecycle Tracking
- Epic 8: Daily Settlement & Mode Analytics

---

## Table of Contents
1. [Sprint Overview](#sprint-overview)
2. [Story Point Estimates](#story-point-estimates)
3. [Sprint Assignments](#sprint-assignments)
4. [Dependency Map](#dependency-map)
5. [Priority Matrix](#priority-matrix)
6. [Risk Register](#risk-register)
7. [Definition of Done](#definition-of-done)
8. [Velocity Assumptions](#velocity-assumptions)

---

## Sprint Overview

### Assumptions

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| **Sprint Duration** | 2 weeks (10 working days) | Standard Agile sprint |
| **Team Size** | 2-3 developers | Small, focused team |
| **Team Velocity** | 20-25 points/sprint | Conservative estimate for new infrastructure work |
| **Total Sprints** | 4 sprints (8 weeks) | Foundation → Core → Analytics → Polish |
| **Buffer** | 20% (built into estimates) | Account for unknowns, infrastructure complexity |

### Total Story Points by Epic

| Epic | Stories | Story Points | % of Total |
|------|---------|--------------|------------|
| **Epic 6: Redis Caching** | 9 stories | 34 points | 34% |
| **Epic 7: Client Order ID** | 11 stories | 43 points | 43% |
| **Epic 8: Daily Settlement** | 11 stories | 23 points | 23% |
| **TOTAL** | 31 stories | **100 points** | 100% |

### Sprint Distribution

| Sprint | Focus Area | Stories | Points | Cumulative |
|--------|------------|---------|--------|------------|
| **Sprint 1** | Redis Foundation | 5 stories | 21 points | 21% |
| **Sprint 2** | Order ID System | 6 stories | 25 points | 46% |
| **Sprint 3** | Settlement Core | 5 stories | 24 points | 70% |
| **Sprint 4** | Analytics & Polish | 15 stories | 30 points | 100% |

---

## Story Point Estimates

### Fibonacci Scale Reference

| Points | Complexity | Effort | Risk | Example |
|--------|------------|--------|------|---------|
| **1** | Trivial | 1-2 hours | None | Add config parameter |
| **2** | Simple | Half day | Low | Simple API endpoint |
| **3** | Moderate | 1 day | Low | Basic CRUD operation |
| **5** | Complex | 2-3 days | Medium | Multi-component feature |
| **8** | Very Complex | 4-5 days | Medium-High | Core infrastructure change |
| **13** | Highly Complex | 1-2 weeks | High | Large architectural shift |

---

### Epic 6: Redis Caching Infrastructure (34 points)

| Story | Title | Points | Justification |
|-------|-------|--------|---------------|
| **6.1** | Redis Infrastructure Setup | 5 | Docker service setup, health checks, connection pool, requires infrastructure knowledge |
| **6.2** | User Settings Cache (Write-Through) | 5 | Complex read/write pattern with DB sync, critical correctness requirement |
| **6.3** | Mode Configuration Cache (4 modes) | 3 | Similar to 6.2 but pattern established, 4 mode types to implement |
| **6.4** | Admin Defaults Cache with Sync | 3 | Hash-based change detection, invalidation on sync (Epic 4 correlation) |
| **6.5** | Scalp Reentry & Circuit Breaker Cache | 3 | Two distinct config types, Epic 5 correlation |
| **6.6** | Cache-First Read Pattern (All APIs) | 5 | Multiple API endpoints to refactor (~6 handlers), testing across endpoints |
| **6.7** | Cache Invalidation on Update | 5 | Critical correctness: synchronous invalidation, DELETE on failure, multiple triggers |
| **6.8** | Ginie Engine Cache Integration | 3 | Modify 4 files in Ginie, replace DB calls with cache calls |
| **6.9** | Cache Fallback & Graceful Degradation | 2 | Circuit breaker pattern, health checks, fallback logic (reuses established patterns) |

**Epic 6 Total: 34 points**

**Epic 6 Risk Factors:**
- Infrastructure complexity (Redis deployment)
- Cache-DB consistency critical (6.7)
- Performance requirements (<1ms reads)
- Ginie engine integration risk (6.8)

---

### Epic 7: Client Order ID & Trade Lifecycle (43 points)

| Story | Title | Points | Justification |
|-------|-------|--------|---------------|
| **7.1** | Client Order ID Generation | 5 | Core generator logic, 5 modes, 12+ order types, validation, integration with order placement |
| **7.2** | Daily Sequence Storage in Redis | 3 | Redis INCR atomic operation, TTL handling, timezone-aware key generation |
| **7.3** | Order Chain Tracking | 2 | Leverage 7.1 base ID, reuse pattern for related orders |
| **7.4** | Parse Client Order ID | 3 | Regex parsing, error handling for malformed IDs, legacy ID support |
| **7.5** | Trade Lifecycle Tab UI | 8 | New React tab, chain card components, timeline visualization, filters, search, P&L calculation |
| **7.6** | User Timezone Settings | 5 | Database migration, timezone preset table, UI for settings, integration with date generation |
| **7.7** | Hedge Order Suffixes | 2 | Extend 7.1 with hedge types (-H, -HSL, -HTP), chain display logic |
| **7.8** | Redis Fallback (UUID-based) | 3 | Fallback ID generation (UUID), graceful degradation, warning logging, UI indication |
| **7.9** | Backend API for Lifecycle Tab | 5 | List & detail endpoints, query filtering (5 params), pagination, P&L aggregation, indexing |
| **7.10** | Edge Case Test Suite | 5 | 10 test categories, Binance acceptance testing, concurrent sequence testing, DST edge cases |
| **7.0** | Epic 6 Dependency Gate Validation | 2 | Verify Redis running, CacheService implemented, test sequence increment |

**Epic 7 Total: 43 points**

**Epic 7 Risk Factors:**
- Epic 6 dependency gate (7.0 critical)
- UI complexity (7.5 largest story)
- Binance API integration testing (7.10)
- Timezone edge cases (7.6, 7.10)

---

### Epic 8: Daily Settlement & Mode Analytics (23 points)

| Story | Title | Points | Justification |
|-------|-------|--------|---------------|
| **8.0** | User Timezone Database Migration | 2 | SQL migration (2 columns), idempotency checks, rollback script, straightforward |
| **8.1** | EOD Snapshot of Open Positions | 3 | Binance API queries, mark-to-market calculation, mode extraction from clientOrderId |
| **8.2** | Daily P&L Aggregation by Mode | 3 | Fetch trades, parse clientOrderId, aggregate by mode, win rate calculation |
| **8.3** | Daily Summary Storage | 2 | Database table creation, upsert logic, straightforward persistence |
| **8.4** | Handle Open Positions in Daily P&L | 2 | Delta calculation (today - yesterday unrealized), match Binance's method |
| **8.5** | Admin Dashboard for Daily Summaries | 3 | Admin endpoint, filters, CSV export, aggregation across users |
| **8.6** | Historical Reports (Date Range) | 2 | Query with date range, rollup logic (weekly/monthly/yearly), indexing |
| **8.7** | Capital Utilization Tracking | 2 | Periodic sampling (5 min), max/avg calculation, simple metrics |
| **8.8** | Settlement Failure Recovery | 2 | Retry with exponential backoff (5s, 15s, 45s), status tracking, admin retry endpoint |
| **8.9** | Settlement Monitoring & Alerts | 1 | Admin status endpoint, email alerts (reuse email service), straightforward |
| **8.10** | Data Quality Validation | 1 | Validation rules (win rate, P&L bounds), flagging logic, admin review endpoint |

**Epic 8 Total: 23 points**

**Epic 8 Risk Factors:**
- Epic 6 & 7 dependencies (Redis, ParseClientOrderId)
- Binance API rate limits during settlement
- DST handling complexity (scheduler)
- Data integrity validation critical

---

## Sprint Assignments

### Sprint 1: Redis Foundation (2 weeks) - 21 points

**Goal:** Establish Redis infrastructure and core caching patterns

**Epics:** Epic 6 only

| Story | Title | Points | Priority | Dependencies |
|-------|-------|--------|----------|--------------|
| 6.1 | Redis Infrastructure Setup | 5 | P0 | None |
| 6.2 | User Settings Cache (Write-Through) | 5 | P0 | 6.1 |
| 6.3 | Mode Configuration Cache | 3 | P0 | 6.1, 6.2 |
| 6.4 | Admin Defaults Cache | 3 | P1 | 6.1, 6.2 |
| 6.5 | Scalp Reentry & Circuit Breaker Cache | 3 | P1 | 6.1, 6.2 |
| 6.9 | Cache Fallback & Graceful Degradation | 2 | P1 | 6.1 |

**Sprint 1 Total: 21 points**

**Risks:**
- Redis container deployment issues (Docker networking)
- Cache-DB consistency bugs in write-through pattern
- Performance not meeting <1ms target

**Mitigation:**
- Early smoke test of Redis container (Day 1)
- Comprehensive integration tests for 6.2
- Load testing after 6.2 completion

**Success Criteria:**
- Redis container healthy and persistent
- User settings cached with write-through working
- All 4 mode configs cached
- Fallback to DB working when Redis unavailable
- Performance: Settings reads <1ms (measured)

---

### Sprint 2: Order ID System (2 weeks) - 25 points

**Goal:** Implement structured Client Order ID generation and parsing

**Epics:** Epic 6 (completion), Epic 7 (foundation)

| Story | Title | Points | Priority | Dependencies |
|-------|-------|--------|----------|--------------|
| 6.6 | Cache-First Read Pattern (All APIs) | 5 | P0 | 6.2, 6.3, 6.4, 6.5 |
| 6.7 | Cache Invalidation on Update | 5 | P0 | 6.6 |
| 6.8 | Ginie Engine Cache Integration | 3 | P0 | 6.6, 6.7 |
| 7.0 | Epic 6 Dependency Gate Validation | 2 | P0 | 6.1-6.9 complete |
| 7.1 | Client Order ID Generation | 5 | P0 | 7.0 (Epic 6 complete) |
| 7.2 | Daily Sequence Storage in Redis | 3 | P0 | 7.1 |
| 7.4 | Parse Client Order ID | 3 | P0 | 7.1 |

**Sprint 2 Total: 26 points** (over by 1 point - acceptable with buffer)

**Risks:**
- Cache invalidation bugs causing stale data
- Ginie engine integration breaking existing functionality
- Epic 6 not fully stable at start of 7.x stories

**Mitigation:**
- Thorough testing of 6.7 before proceeding
- Feature flag for Ginie cache integration (rollback option)
- Day 1: Validate Epic 6 stability before starting 7.1

**Success Criteria:**
- All API endpoints use cache-first reads
- Cache invalidates immediately on any update
- Ginie engine reads settings from cache only (0 DB queries during trading)
- Client Order ID generates correctly for all 5 modes
- Sequence storage atomic in Redis
- Parser handles all valid and malformed IDs gracefully

---

### Sprint 3: Settlement Core (2 weeks) - 24 points

**Goal:** Implement daily settlement processing and Trade Lifecycle UI

**Epics:** Epic 7 (core features), Epic 8 (foundation)

| Story | Title | Points | Priority | Dependencies |
|-------|-------|--------|----------|--------------|
| 7.3 | Order Chain Tracking | 2 | P0 | 7.1, 7.2 |
| 7.6 | User Timezone Settings | 5 | P0 | None (parallel to other work) |
| 7.7 | Hedge Order Suffixes | 2 | P1 | 7.1, 7.3 |
| 7.8 | Redis Fallback for Sequence | 3 | P1 | 7.1, 7.2 |
| 8.0 | User Timezone Database Migration | 2 | P0 | None |
| 8.1 | EOD Snapshot of Open Positions | 3 | P0 | 8.0, 7.4 (parser) |
| 8.2 | Daily P&L Aggregation by Mode | 3 | P0 | 8.0, 7.4 (parser) |
| 8.3 | Daily Summary Storage | 2 | P0 | 8.1, 8.2 |
| 8.4 | Handle Open Positions in P&L | 2 | P0 | 8.1, 8.2, 8.3 |

**Sprint 3 Total: 24 points**

**Risks:**
- Timezone complexity (DST transitions)
- Mode extraction from clientOrderId fails for legacy orders
- Settlement calculation bugs (P&L accuracy critical)

**Mitigation:**
- Early testing of timezone migrations (8.0 Day 1)
- Default "UNKNOWN" mode for unparseable IDs
- Cross-validation with Binance API totals for P&L

**Success Criteria:**
- User timezone settings functional
- Order chains correctly linked (entry/SL/TP/hedge)
- Hedge orders identified with -H/-HSL/-HTP suffixes
- Redis fallback generates valid UUID-based IDs
- EOD snapshots capture mark-to-market correctly
- Daily summaries stored per mode per user
- Open position handling matches Binance's daily P&L method

---

### Sprint 4: Analytics & Polish (2 weeks) - 30 points

**Goal:** Complete Trade Lifecycle UI, analytics, and operational excellence

**Epics:** Epic 7 (completion), Epic 8 (completion)

| Story | Title | Points | Priority | Dependencies |
|-------|-------|--------|----------|--------------|
| 7.5 | Trade Lifecycle Tab UI | 8 | P0 | 7.4, 7.9 |
| 7.9 | Backend API for Lifecycle Tab | 5 | P0 | 7.3, 7.4 |
| 7.10 | Edge Case Test Suite | 5 | P1 | 7.1-7.9 complete |
| 8.5 | Admin Dashboard for Summaries | 3 | P0 | 8.3 |
| 8.6 | Historical Reports (Date Range) | 2 | P1 | 8.3 |
| 8.7 | Capital Utilization Tracking | 2 | P1 | 8.1-8.4 |
| 8.8 | Settlement Failure Recovery | 2 | P1 | 8.1-8.4 |
| 8.9 | Settlement Monitoring & Alerts | 1 | P1 | 8.8 |
| 8.10 | Data Quality Validation | 1 | P2 | 8.3 |
| - | **Integration Testing** | 1 | P0 | All stories |

**Sprint 4 Total: 30 points**

**Risks:**
- UI complexity causing delays (7.5 is 8 points)
- Edge case testing uncovers critical bugs requiring fixes
- Binance API rate limits during settlement load testing

**Mitigation:**
- Start 7.5 (UI) in parallel with backend work (7.9)
- Reserve last 2 days for bug fixes from 7.10
- Stagger settlement testing across multiple test accounts

**Success Criteria:**
- Trade Lifecycle Tab fully functional with filters and search
- All edge cases passing (midnight rollover, DST, concurrent sequences, Binance acceptance)
- Admin can view historical data and export for billing
- Historical reports with weekly/monthly/yearly rollups
- Settlement failure recovery working (retry with backoff)
- Admin alerts for persistent failures
- Data quality validation catches anomalies
- All 31 stories complete with acceptance criteria met

---

## Dependency Map

### Epic-Level Dependencies (ASCII Diagram)

```
┌──────────────────────────────────────────────────────────────────┐
│                        EPIC DEPENDENCIES                         │
└──────────────────────────────────────────────────────────────────┘

EPIC 6: REDIS CACHING INFRASTRUCTURE
┌─────────────────────────────────────┐
│ Stories 6.1 → 6.9                   │  Foundation for everything
│ (Redis container, caching patterns) │
└─────────────────────────────────────┘
                │
                ├──────────────────────────────────────┐
                │                                      │
                ▼                                      ▼
┌─────────────────────────────────┐    ┌─────────────────────────────────┐
│ EPIC 7: CLIENT ORDER ID         │    │ EPIC 8: DAILY SETTLEMENT        │
│ Stories 7.1 → 7.10              │───>│ Stories 8.0 → 8.10              │
│ (Sequence in Redis: 7.2)        │    │ (Needs ParseClientOrderId: 7.4) │
└─────────────────────────────────┘    └─────────────────────────────────┘
    │ 7.4 (ParseClientOrderId)                       │
    └────────────────────────────────────────────────┘
                     Mode extraction for 8.1, 8.2


CRITICAL PATH:
6.1 → 6.2 → 7.0 (gate) → 7.1 → 7.2 → 7.4 → 8.1 → 8.2 → 8.3

KEY:
→  Blocking dependency (must complete before next)
├─ Parallel work possible
```

---

### Story-Level Dependencies (Detailed)

#### Sprint 1 Dependencies

```
6.1 Redis Infrastructure
 ├─→ 6.2 User Settings Cache
 ├─→ 6.3 Mode Config Cache
 ├─→ 6.4 Admin Defaults Cache
 ├─→ 6.5 Scalp Reentry & CB Cache
 └─→ 6.9 Cache Fallback

6.2 User Settings Cache
 └─→ 6.3 Mode Config Cache (pattern established)
```

**Parallel Work:**
- 6.4, 6.5, 6.9 can proceed in parallel once 6.1, 6.2 complete

---

#### Sprint 2 Dependencies

```
Epic 6 Completion (6.1-6.9)
 └─→ 7.0 Dependency Gate Validation
      └─→ 7.1 Client Order ID Generation
           ├─→ 7.2 Sequence Storage (uses 7.1 base)
           └─→ 7.4 Parser (parses 7.1 format)

6.2, 6.3, 6.4, 6.5
 └─→ 6.6 Cache-First Reads (refactors APIs)
      └─→ 6.7 Cache Invalidation (sync with 6.6)
           └─→ 6.8 Ginie Integration (uses 6.6, 6.7)
```

**Parallel Work:**
- 7.1, 7.2, 7.4 can start once 7.0 validated
- 6.6, 6.7, 6.8 form a sequential chain

---

#### Sprint 3 Dependencies

```
7.1 Client Order ID Generation
 ├─→ 7.3 Order Chain Tracking (uses base ID)
 ├─→ 7.7 Hedge Suffixes (extends 7.1)
 └─→ 7.8 Redis Fallback (fallback for 7.1)

7.4 Parser
 ├─→ 8.1 Position Snapshots (parses mode)
 └─→ 8.2 P&L Aggregation (parses mode)

8.0 Timezone Migration (BLOCKER)
 ├─→ 8.1 Position Snapshots
 └─→ 8.2 P&L Aggregation

8.1, 8.2
 └─→ 8.3 Summary Storage
      └─→ 8.4 Open Position Handling
```

**Parallel Work:**
- 7.6 Timezone Settings (independent, can start early)
- 7.3, 7.7, 7.8 can proceed in parallel once 7.1 complete
- 8.1, 8.2 can proceed in parallel once 8.0, 7.4 complete

---

#### Sprint 4 Dependencies

```
7.4 Parser + 8.3 Summary Storage
 └─→ 7.9 Backend API for Lifecycle
      └─→ 7.5 Trade Lifecycle Tab UI

7.1-7.9
 └─→ 7.10 Edge Case Test Suite

8.3 Summary Storage
 ├─→ 8.5 Admin Dashboard
 ├─→ 8.6 Historical Reports
 ├─→ 8.7 Capital Tracking
 ├─→ 8.8 Failure Recovery
 │    └─→ 8.9 Monitoring & Alerts
 └─→ 8.10 Data Quality Validation
```

**Parallel Work:**
- 7.5 (UI) and 7.9 (API) can overlap (API first, then UI integration)
- 8.5, 8.6, 8.7, 8.8, 8.10 can proceed in parallel once 8.3 complete
- 8.9 waits for 8.8

---

### Dependency Gate Checklist

**Before Starting Sprint 2 (Epic 7):**
- [ ] Redis container healthy (`docker ps | grep redis`)
- [ ] CacheService implemented with `IncrementDailySequence()` method
- [ ] All Epic 6 stories (6.1-6.9) acceptance criteria met
- [ ] Performance: Settings reads <1ms validated
- [ ] Load test: 1000 reads/second passed

**Before Starting Sprint 3 (Epic 8):**
- [ ] Client Order ID generation working for all 5 modes
- [ ] Parser handles valid and malformed IDs gracefully
- [ ] Sequence storage atomic in Redis
- [ ] User timezone migration (8.0) applied to database

**Before Starting Sprint 4 (Polish):**
- [ ] Daily settlement running successfully for test users
- [ ] Mode breakdown accurate (cross-validated with Binance)
- [ ] Backend API endpoints returning correct data

---

## Priority Matrix

### Priority Definitions

| Priority | Definition | Action |
|----------|------------|--------|
| **P0** | MUST HAVE - System cannot function without this | Do first, no compromise |
| **P1** | CRITICAL - Essential for user value and correctness | Do in sprint, defer only if blocked |
| **P2** | IMPORTANT - Valuable but system functional without | Can defer to next sprint if needed |
| **P3** | NICE TO HAVE - Polish and enhancements | Defer if sprint at risk |

---

### Priority Assignments

#### Epic 6: Redis Caching (P0 Epic - Foundation)

| Story | Priority | Rationale |
|-------|----------|-----------|
| 6.1 | **P0** | Blocks all other Epic 6 and Epic 7 stories |
| 6.2 | **P0** | Core caching pattern, required for 6.6, 6.7, 6.8 |
| 6.3 | **P0** | Ginie needs mode configs cached |
| 6.4 | **P1** | Admin defaults needed for Epic 4 correlation |
| 6.5 | **P1** | Scalp reentry and CB needed for Epic 5 correlation |
| 6.6 | **P0** | All APIs must use cache-first pattern |
| 6.7 | **P0** | Cache-DB consistency critical |
| 6.8 | **P0** | Ginie must read from cache for performance |
| 6.9 | **P1** | Graceful degradation important but not blocker |

---

#### Epic 7: Client Order ID (P0 Epic - Traceability)

| Story | Priority | Rationale |
|-------|----------|-----------|
| 7.0 | **P0** | Gate validation prevents broken dependencies |
| 7.1 | **P0** | Core ID generation blocks all other stories |
| 7.2 | **P0** | Sequence storage required for 7.1 to work |
| 7.3 | **P0** | Chain tracking essential for order grouping |
| 7.4 | **P0** | Parser required for Epic 8 (mode extraction) |
| 7.5 | **P0** | Trade Lifecycle UI is primary user-facing feature |
| 7.6 | **P0** | Timezone required for date component accuracy |
| 7.7 | **P1** | Hedge orders important but system works without |
| 7.8 | **P1** | Fallback prevents system failure but not critical path |
| 7.9 | **P0** | Backend API required for 7.5 UI |
| 7.10 | **P1** | Edge case testing critical but can defer some cases |

---

#### Epic 8: Daily Settlement (P1 Epic - Analytics)

| Story | Priority | Rationale |
|-------|----------|-----------|
| 8.0 | **P0** | Timezone migration blocks all settlement work |
| 8.1 | **P0** | Position snapshots required for accurate P&L |
| 8.2 | **P0** | P&L aggregation core to settlement |
| 8.3 | **P0** | Storage required for all analytics |
| 8.4 | **P0** | Open position handling ensures P&L accuracy |
| 8.5 | **P1** | Admin dashboard important for billing |
| 8.6 | **P1** | Historical reports valuable but not critical path |
| 8.7 | **P1** | Capital tracking useful for risk management |
| 8.8 | **P1** | Failure recovery important for production reliability |
| 8.9 | **P1** | Monitoring improves operational awareness |
| 8.10 | **P2** | Data quality validation adds robustness |

---

### Priority-Based Sprint Planning

#### Sprint 1 Focus: P0 Foundation Stories
- All stories are P0 or P1 (Redis foundation critical)
- No P2/P3 stories in Sprint 1

#### Sprint 2 Focus: P0 Order ID Core
- All stories are P0 (complete Epic 6, start Epic 7 core)
- No deferrable stories in Sprint 2

#### Sprint 3 Focus: P0 Settlement Core + P1 Order ID Features
- Mix of P0 (settlement core) and P1 (hedge, fallback)
- Can defer 7.7, 7.8 if sprint at risk

#### Sprint 4 Focus: P0 UI + P1/P2 Polish
- Must complete: 7.5 (UI), 7.9 (API), 7.10 (tests) - P0
- Can defer: 8.7, 8.8, 8.9, 8.10 if needed - P1/P2

---

## Risk Register

### Risk Categories

1. **Technical Risks** - Implementation complexity, unknowns
2. **Dependency Risks** - Blocking dependencies, external systems
3. **Integration Risks** - Multi-component interactions
4. **Performance Risks** - Latency, throughput, scale
5. **Operational Risks** - Deployment, monitoring, recovery

---

### Epic 6 Risks

| Risk ID | Risk | Category | Probability | Impact | Severity | Mitigation | Owner |
|---------|------|----------|-------------|--------|----------|------------|-------|
| **R6.1** | Redis container networking issues in Docker | Technical | MEDIUM | HIGH | **HIGH** | Early smoke test Day 1, test with docker-compose up, validate health check | DevOps + Dev |
| **R6.2** | Cache-DB consistency bugs (write-through pattern) | Technical | MEDIUM | CRITICAL | **CRITICAL** | Comprehensive integration tests, transactional logic, DELETE on cache update failure | Dev + QA |
| **R6.3** | Performance not meeting <1ms target | Performance | LOW | HIGH | **MEDIUM** | Early load testing after 6.2, Redis on same network, use pipelining if needed | Dev |
| **R6.4** | Ginie engine integration breaks existing functionality | Integration | MEDIUM | HIGH | **HIGH** | Feature flag for cache integration, rollback plan, thorough testing before merge | Dev + QA |
| **R6.5** | Redis memory pressure (OOM) | Operational | LOW | MEDIUM | **LOW** | maxmemory 512mb + noeviction policy, monitoring, alerts | DevOps |
| **R6.6** | Cache invalidation misses some code paths | Technical | MEDIUM | HIGH | **HIGH** | Code review for all PUT/POST endpoints, grep for DB update calls | Dev + QA |

**Epic 6 High-Severity Risks: 3** (R6.1, R6.2, R6.4)

---

### Epic 7 Risks

| Risk ID | Risk | Category | Probability | Impact | Severity | Mitigation | Owner |
|---------|------|----------|-------------|--------|----------|------------|-------|
| **R7.1** | Epic 6 not stable at start of Epic 7 | Dependency | LOW | CRITICAL | **HIGH** | Story 7.0 gate validation, 2-day buffer at end of Sprint 1 | SM + Dev |
| **R7.2** | UI complexity causes 7.5 delays (8 points) | Technical | MEDIUM | HIGH | **HIGH** | Start 7.5 early in Sprint 4, parallel backend/frontend work, consider pair programming | Dev + UI Dev |
| **R7.3** | Binance rejects clientOrderId format | Integration | LOW | CRITICAL | **MEDIUM** | Early test with Binance testnet (7.10), validate character limits, test all order types | Dev |
| **R7.4** | Timezone edge cases fail (DST transitions) | Technical | MEDIUM | MEDIUM | **MEDIUM** | Use Go time.Location (auto-handles DST), comprehensive tests in 7.10, store dates as strings | Dev + QA |
| **R7.5** | Sequence race condition causes duplicate IDs | Technical | LOW | CRITICAL | **HIGH** | Redis INCR is atomic, concurrent test in 7.10, load test 100+ concurrent | Dev + QA |
| **R7.6** | Parser fails on unexpected Binance ID format | Integration | LOW | MEDIUM | **LOW** | Graceful handling (return nil), log warnings, default to "UNKNOWN" mode | Dev |
| **R7.7** | Trade Lifecycle API too slow (>2s load time) | Performance | MEDIUM | MEDIUM | **MEDIUM** | Database indexing on clientOrderId prefix, pagination (limit 50), cache frequently accessed chains | Dev |

**Epic 7 High-Severity Risks: 3** (R7.1, R7.2, R7.5)

---

### Epic 8 Risks

| Risk ID | Risk | Category | Probability | Impact | Severity | Mitigation | Owner |
|---------|------|----------|-------------|--------|----------|------------|-------|
| **R8.1** | Binance API rate limits during settlement | Integration | HIGH | MEDIUM | **HIGH** | Stagger user settlements (1 per minute), exponential backoff (5s, 15s, 45s), cache where possible | Dev |
| **R8.2** | Binance API timeout/connection failure | Integration | MEDIUM | HIGH | **HIGH** | 3-retry strategy, mark as 'failed' status, admin retry endpoint, alert after 1 hour | Dev |
| **R8.3** | P&L calculation inaccurate (mismatches Binance) | Technical | MEDIUM | CRITICAL | **CRITICAL** | Cross-validate with Binance daily totals, data quality validation (8.10), flag >$100 differences | Dev + QA |
| **R8.4** | Settlement job crashes/doesn't restart | Operational | LOW | HIGH | **MEDIUM** | Status tracking in DB, auto-retry on restart, manual trigger endpoint, monitoring (8.9) | DevOps |
| **R8.5** | Mode extraction fails for legacy orders (no clientOrderId) | Technical | HIGH | LOW | **LOW** | Default to "UNKNOWN" mode, still include in "ALL" totals, document limitation | Dev |
| **R8.6** | DST transitions cause duplicate/missed settlements | Technical | LOW | HIGH | **MEDIUM** | Go time.Location auto-handles DST, store dates as strings, test Spring/Fall transitions | Dev + QA |
| **R8.7** | Database transaction failure during settlement | Technical | LOW | MEDIUM | **LOW** | Rollback + single retry after 10s, mark as failed if unsuccessful, transaction isolation | Dev |
| **R8.8** | Admin not notified of settlement failures | Operational | MEDIUM | MEDIUM | **MEDIUM** | Email alerts after 1 hour, monitoring dashboard, periodic status checks (8.9) | DevOps |

**Epic 8 High-Severity Risks: 3** (R8.1, R8.2, R8.3)

---

### Cross-Epic Risks

| Risk ID | Risk | Category | Probability | Impact | Severity | Mitigation | Owner |
|---------|------|----------|-------------|--------|----------|------------|-------|
| **RX.1** | Team velocity lower than estimated (20-25 points/sprint) | Velocity | MEDIUM | HIGH | **HIGH** | Conservative estimates (20% buffer built in), adjust Sprint 2-4 scope based on Sprint 1 actual | SM |
| **RX.2** | Key developer unavailable mid-sprint | Team | LOW | HIGH | **MEDIUM** | Knowledge sharing, pair programming on critical stories, document as you go | SM + Team |
| **RX.3** | Scope creep from user requests during sprints | Scope | MEDIUM | MEDIUM | **MEDIUM** | Strict sprint commitment, defer new requests to backlog, Product Owner approval required | SM + PO |
| **RX.4** | Integration testing uncovers major bugs late | Quality | MEDIUM | HIGH | **HIGH** | Continuous integration, test each story on completion, reserve last 2 days of Sprint 4 for fixes | QA + Dev |

---

### Risk Mitigation Timeline

```
Sprint 1:
  Week 1: R6.1 mitigation (smoke test Redis), R6.2 setup (test harness)
  Week 2: R6.3 load testing, R6.4 feature flag implementation

Sprint 2:
  Week 1: R7.1 gate validation (Day 1), R7.5 concurrent testing
  Week 2: R7.3 Binance testnet validation

Sprint 3:
  Week 1: R8.6 DST test setup, R7.4 timezone testing
  Week 2: R8.3 P&L cross-validation framework

Sprint 4:
  Week 1: R7.2 UI complexity (pair programming if needed)
  Week 2: RX.4 integration testing, bug fix buffer
```

---

## Definition of Done

### Story-Level Definition of Done

A story is considered DONE when:

#### Code Complete
- [ ] All acceptance criteria met and verified
- [ ] Code written following Go best practices and project standards
- [ ] No hardcoded values (use environment variables or config)
- [ ] Error handling comprehensive (no bare `err != nil` without context)
- [ ] Logging added for key operations (Info, Warn, Error levels)
- [ ] Code reviewed by at least one other developer
- [ ] No unresolved review comments

#### Testing Complete
- [ ] Unit tests written with >80% code coverage
- [ ] Integration tests written for multi-component interactions
- [ ] All tests passing locally
- [ ] All tests passing in CI/CD pipeline
- [ ] Edge cases tested (nulls, empty strings, boundary values)
- [ ] Performance tested (if performance requirements exist)

#### Documentation Complete
- [ ] Code comments for complex logic
- [ ] API endpoint documented (if applicable) with examples
- [ ] README updated (if infrastructure change)
- [ ] Database migration documented (if schema change)

#### Deployment Ready
- [ ] Feature merged to `main` branch
- [ ] Docker container rebuilds successfully
- [ ] Health check passing in development environment
- [ ] No regressions in existing functionality

#### Product Owner Acceptance
- [ ] Demo completed to Product Owner
- [ ] Product Owner sign-off received

---

### Sprint-Level Definition of Done

A sprint is considered DONE when:

#### Sprint Goal Achieved
- [ ] All committed stories completed (meet Story-Level DoD)
- [ ] Sprint goal demonstrated and accepted
- [ ] Any incomplete stories moved to next sprint with rationale

#### Quality Gates Passed
- [ ] All automated tests passing
- [ ] No critical or high-severity bugs open
- [ ] Code coverage maintained or improved
- [ ] Performance benchmarks met (if applicable)

#### Documentation Updated
- [ ] Sprint retrospective completed
- [ ] Lessons learned documented
- [ ] Known issues logged in backlog
- [ ] Deployment notes updated

#### Production Ready
- [ ] Development environment stable
- [ ] No blocking issues for next sprint
- [ ] Demo completed to stakeholders

---

### Epic-Level Definition of Done

An epic is considered DONE when:

#### All Stories Complete
- [ ] All stories meet Story-Level DoD
- [ ] All acceptance criteria across epic verified

#### Integration Verified
- [ ] End-to-end testing completed
- [ ] Cross-component integration stable
- [ ] Performance targets met

#### Documentation Complete
- [ ] User documentation updated (if user-facing)
- [ ] Admin documentation updated (if admin features)
- [ ] API documentation complete
- [ ] Architecture diagrams updated

#### Production Deployment
- [ ] Feature deployed to production (if applicable)
- [ ] Monitoring and alerting configured
- [ ] Rollback plan tested

---

### Sprint-Specific DoD

#### Sprint 1 DoD: Redis Foundation
- [ ] Redis container healthy and persistent across restarts
- [ ] All 4 mode configs cached
- [ ] User settings cached with write-through pattern
- [ ] Performance: Settings reads <1ms (measured and documented)
- [ ] Load test: 1000 reads/second passed
- [ ] Fallback to DB working when Redis unavailable

#### Sprint 2 DoD: Order ID System
- [ ] All API endpoints use cache-first reads
- [ ] Cache invalidates synchronously on updates
- [ ] Ginie engine reads from cache (0 DB queries during trading verified)
- [ ] Client Order ID generates for all 5 modes
- [ ] Parser handles valid and malformed IDs
- [ ] Sequence storage atomic (concurrent test passed)

#### Sprint 3 DoD: Settlement Core
- [ ] User timezone migration applied
- [ ] Order chains correctly linked (entry/SL/TP/hedge)
- [ ] EOD snapshots capture mark-to-market accurately
- [ ] Daily summaries stored per mode per user
- [ ] P&L calculation matches Binance (cross-validated)
- [ ] Open position handling correct (delta calculation)

#### Sprint 4 DoD: Analytics & Polish
- [ ] Trade Lifecycle Tab functional with filters and search
- [ ] All edge case tests passing (7.10)
- [ ] Admin dashboard shows historical data
- [ ] Settlement failure recovery working (retry + alerts)
- [ ] Data quality validation catches anomalies
- [ ] All 31 stories complete and deployed

---

## Velocity Assumptions

### Team Velocity Model

#### Historical Velocity (Assumed)
- **Team Size:** 2-3 developers
- **Sprint Duration:** 2 weeks (10 working days)
- **Average Velocity:** 20-25 points/sprint
- **Velocity Range:** 18-28 points (±20% variance)

#### Velocity Factors

| Factor | Impact on Velocity | Rationale |
|--------|-------------------|-----------|
| **Infrastructure Work** | -10% | Epic 6 Redis setup (new infrastructure complexity) |
| **New Technology** | -5% | Team learning Redis patterns |
| **High Dependency Count** | -10% | Sequential dependencies reduce parallelism |
| **Complex UI Work** | -15% | Story 7.5 Trade Lifecycle UI (8 points, React complexity) |
| **Testing Overhead** | -10% | Edge cases (7.10), settlement accuracy critical |
| **Team Experience** | +10% | Assuming experienced Go/React developers |

**Adjusted Velocity: 20-25 points/sprint** (conservative)

---

### Sprint Velocity Targets

| Sprint | Planned Points | Adjusted Target | Confidence | Rationale |
|--------|----------------|-----------------|------------|-----------|
| **Sprint 1** | 21 points | 20-22 points | HIGH | Foundation work, infrastructure complexity, but clear scope |
| **Sprint 2** | 26 points | 23-26 points | MEDIUM | Above velocity target, Ginie integration risk, but Epic 6 pattern established |
| **Sprint 3** | 24 points | 22-25 points | MEDIUM | Settlement core critical, timezone complexity, but parser pattern reused |
| **Sprint 4** | 30 points | 28-32 points | LOW | UI complexity (7.5), many parallel stories possible, but edge case testing risk |

---

### Velocity Tracking

#### Metrics to Track

| Metric | Target | Purpose |
|--------|--------|---------|
| **Story Completion Rate** | 100% committed stories | Validate sprint planning accuracy |
| **Actual vs Planned Points** | ±10% variance | Adjust future sprint planning |
| **Story Cycle Time** | <3 days average | Identify bottlenecks |
| **Rework Rate** | <10% of points | Quality indicator |
| **Carryover Rate** | <10% of points | Sprint commitment accuracy |

#### Velocity Adjustment Strategy

**After Sprint 1:**
- If actual velocity 18-20 points → Reduce Sprint 2 to 22 points
- If actual velocity 22-25 points → Keep Sprint 2 at 26 points
- If actual velocity >25 points → Consider adding 2-3 points to Sprint 2

**After Sprint 2:**
- Recalculate based on Sprint 1 + Sprint 2 average
- Adjust Sprint 3 and Sprint 4 scope accordingly

**Contingency Plan:**
- If velocity consistently <20 points → Defer P2 stories (8.10, 8.6, 8.7)
- If velocity >25 points → Pull forward P2 stories from backlog

---

### Timeline Projection

#### Optimistic Scenario (Velocity = 25 points/sprint)
- **Sprint 1:** 21 points (85% capacity) → Complete on time
- **Sprint 2:** 26 points (104% capacity) → Complete on time or 1-2 days over
- **Sprint 3:** 24 points (96% capacity) → Complete on time
- **Sprint 4:** 30 points (120% capacity) → Complete with 2-3 days buffer

**Total Duration: 8 weeks (4 sprints)**

---

#### Realistic Scenario (Velocity = 22 points/sprint)
- **Sprint 1:** 21 points → Complete on time
- **Sprint 2:** 26 points → Defer 4 points (6.8, 6.9) to Sprint 3
- **Sprint 3:** 24 + 4 = 28 points → Defer 6 points (7.7, 7.8, 8.7) to Sprint 4
- **Sprint 4:** 30 + 6 = 36 points → Defer 14 points (8.6, 8.8, 8.9, 8.10) to Sprint 5

**Total Duration: 9-10 weeks (4-5 sprints)** with scope adjustment

---

#### Pessimistic Scenario (Velocity = 18 points/sprint)
- **Sprint 1:** 21 points → 3 points deferred (6.4, 6.5)
- **Sprint 2:** 26 + 3 = 29 points → 11 points deferred
- **Sprint 3:** 24 + 11 = 35 points → 17 points deferred
- **Sprint 4:** 30 + 17 = 47 points → Multiple sprints needed

**Total Duration: 11-12 weeks (5-6 sprints)** with significant scope adjustment

**Risk Mitigation:** Focus on P0 stories only, defer all P2 stories

---

### Velocity Adjustment Triggers

| Trigger | Action |
|---------|--------|
| **Sprint 1 velocity <20 points** | Emergency sprint retrospective, identify blockers, reduce Sprint 2-4 scope by 15% |
| **Story cycle time >4 days** | Pair programming, break down stories further, identify blockers |
| **Carryover >2 stories** | Re-evaluate story size estimates, improve sprint planning |
| **Rework rate >15%** | Improve code review process, increase testing |

---

## Appendix A: Story Prioritization Rationale

### Why Epic 6 is P0 Foundation
- **No caching = slow Ginie** (250ms+ DB queries per cycle)
- **Epic 7 depends on Redis** (sequence storage)
- **Performance critical** for real-time trading decisions

### Why Epic 7 is P0 Traceability
- **Mode analytics impossible** without structured IDs
- **Epic 8 depends on mode extraction** (parser)
- **User value high** (Trade Lifecycle UI primary feature)

### Why Epic 8 is P1 Analytics
- **Billing required** but not immediate blocker
- **Historical data valuable** but system functional without
- **Operational excellence** improves over time

---

## Appendix B: Burndown Chart Projections

### Sprint 1 Burndown (Ideal)

```
Points
21 │●
20 │ ●
18 │  ●
15 │   ●
12 │    ●
10 │     ●
 8 │      ●
 5 │       ●
 3 │        ●
 0 │_________●
   Day 1  3  5  7  9
```

**Milestones:**
- Day 2: 6.1 complete (Redis running)
- Day 5: 6.2 complete (write-through pattern)
- Day 7: 6.3, 6.4, 6.5 complete
- Day 9: 6.9 complete

---

### Sprint 2 Burndown (Ideal)

```
Points
26 │●
23 │ ●
20 │  ●
17 │   ●
14 │    ●
11 │     ●
 8 │      ●
 5 │       ●
 2 │        ●
 0 │_________●
   Day 1  3  5  7  9
```

**Milestones:**
- Day 2: 6.6 complete (cache-first reads)
- Day 4: 6.7 complete (invalidation)
- Day 6: 6.8, 7.0 complete (Ginie integration, gate validation)
- Day 8: 7.1, 7.2 complete (ID generation, sequence)
- Day 10: 7.4 complete (parser)

---

## Appendix C: Communication Plan

### Daily Standups (15 minutes)
- **When:** Every day at 9:00 AM
- **Format:**
  - What did I complete yesterday?
  - What will I work on today?
  - Any blockers?
- **Focus:** Story progress, dependency coordination

### Sprint Planning (2 hours)
- **When:** Day 1 of sprint
- **Attendees:** Team, Product Owner, Scrum Master
- **Agenda:**
  1. Review sprint goal
  2. Commit to stories
  3. Break down stories into tasks
  4. Estimate tasks (hours)
  5. Identify risks and dependencies

### Sprint Review (1 hour)
- **When:** Last day of sprint
- **Attendees:** Team, Product Owner, Stakeholders
- **Agenda:**
  1. Demo completed stories
  2. Review sprint metrics (velocity, completion rate)
  3. Stakeholder feedback

### Sprint Retrospective (1 hour)
- **When:** Last day of sprint (after review)
- **Attendees:** Team, Scrum Master
- **Agenda:**
  1. What went well?
  2. What didn't go well?
  3. Action items for next sprint

### Dependency Check-ins (30 minutes, as needed)
- **When:** Ad-hoc when dependencies at risk
- **Attendees:** Affected developers, Scrum Master
- **Purpose:** Unblock dependencies, adjust sprint scope

---

## Appendix D: Success Metrics

### Epic 6 Success Metrics
- **Performance:** Settings reads <1ms (target: <1ms, acceptable: <5ms)
- **Load:** 1000 reads/second without errors
- **Availability:** 99.9% cache hit rate (cache miss only on first read)
- **Reliability:** Cache-DB consistency 100% (no stale data bugs)

### Epic 7 Success Metrics
- **Coverage:** 100% of orders have structured clientOrderId
- **Accuracy:** Parser handles 100% of generated IDs + legacy IDs gracefully
- **Traceability:** Order chains correctly linked (validated manually for 10 chains)
- **UI Performance:** Trade Lifecycle Tab loads within 2 seconds

### Epic 8 Success Metrics
- **Accuracy:** Daily P&L matches Binance totals within $1 variance
- **Timeliness:** Settlement completes within 5 minutes per user
- **Reliability:** Settlement success rate >95% (with retry)
- **Coverage:** Historical data available beyond 90 days

---

## Appendix E: Contact Information

| Role | Name | Responsibility | Contact |
|------|------|----------------|---------|
| **Scrum Master** | Bob | Sprint planning, facilitation, risk management | bob@binance-bot.local |
| **Product Owner** | John (PM) | Backlog prioritization, acceptance | john@binance-bot.local |
| **Tech Lead** | Winston (Architect) | Architecture decisions, code review | winston@binance-bot.local |
| **Analyst** | Mary | Requirements, acceptance criteria | mary@binance-bot.local |
| **QA Lead** | Murat | Test strategy, quality gates | murat@binance-bot.local |

---

## Document Version History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-01-06 | Bob (Scrum Master) | Initial sprint plan creation for Epics 6, 7, 8 |

---

## Approval

**Reviewed By:**
- [ ] John (Product Owner) - Scope and priorities approved
- [ ] Winston (Architect) - Technical dependencies validated
- [ ] Mary (Analyst) - Requirements alignment confirmed
- [ ] Murat (QA Lead) - Testing strategy approved

**Approved By:** Bob (Scrum Master)
**Date:** 2026-01-06

---

**END OF SPRINT PLAN**
