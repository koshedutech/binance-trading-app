# Story 7.3: Order Chain Tracking
**Epic:** Epic 7 - Client Order ID & Trade Lifecycle Tracking
**Sprint:** Sprint 7
**Story Points:** 3
**Priority:** P0
**Status:** done

## User Story
As a trader, I want all related orders (entry, SL, TP, hedge, DCA) to share the same chain ID so that I can view them as a single trade lifecycle.

## Acceptance Criteria
- [x] Entry order generates new chain base ID (without type suffix)
- [x] SL/TP orders reuse same base ID with different suffixes
- [x] Hedge orders use same base ID with -H/-HSL/-HTP suffixes
- [x] DCA orders use same base ID with -DCA1/-DCA2/-DCA3 suffixes
- [x] Chain base ID passed through entire order placement flow
- [x] Store chain base ID with position/trade data in database
- [x] Chain tracker maintains state for active chains
- [x] Related orders linkable via chain base extraction

## Technical Approach

1. **Chain ID Concept**:
   - Base: `ULT-06JAN-00001` (mode + date + sequence)
   - Full IDs: `ULT-06JAN-00001-E`, `ULT-06JAN-00001-SL`, etc.
   - All orders sharing base belong to same trade

2. **Order Flow Integration**:
   ```
   Signal Received → Generate Chain Base
                  ↓
   Entry Order → {base}-E
   SL Order    → {base}-SL
   TP1 Order   → {base}-TP1
   TP2 Order   → {base}-TP2
   [Hedge Triggered]
   Hedge Order → {base}-H
   HSL Order   → {base}-HSL
   HTP Order   → {base}-HTP
   ```

3. **Chain State Management**:
   - `ChainTracker` service tracks active chains
   - Stores chain metadata: base ID, symbol, mode, direction
   - Updates on order events (filled, canceled, closed)
   - Provides chain status queries

4. **Database Storage**:
   - Add `chain_base_id` column to `positions` table
   - Add `chain_base_id` column to `orders` table
   - Index on chain_base_id for efficient queries

5. **Integration Points**:
   - Ginie autopilot: Generate chain on entry signal
   - SL/TP placement: Reuse chain from entry order
   - Hedge activation: Reuse chain from protected position
   - DCA triggers: Reuse chain from original entry

## Dependencies
- **Blocked By:**
  - Story 7.1: Client Order ID Generation (provides Generate and GenerateRelated)
  - Story 7.2: Daily Sequence Storage
- **Blocks:**
  - Story 7.4: Parse Client Order ID (needs chain extraction)
  - Story 7.5: Trade Lifecycle Tab UI (displays chains)
  - Story 7.9: Backend API for Trade Lifecycle

## Files to Create/Modify

### Files to Create:
- `internal/orders/chain_tracker.go` - ChainTracker service for managing active chains
- `internal/orders/chain_state.go` - Chain state models and status tracking
- `migrations/000X_add_chain_base_id.sql` - Database migration for chain_base_id column

### Files to Modify:
- `internal/autopilot/ginie_autopilot.go` - Generate chain on entry, pass to SL/TP/Hedge
- `internal/autopilot/futures_controller.go` - Accept chain base ID parameter
- `internal/database/models.go` - Add ChainBaseId field to Position and Order models
- `internal/database/position_repository.go` - Query by chain base ID
- `internal/binance/futures_client.go` - Pass chain base through order flow

## Testing Requirements

### Unit Tests:
- Test chain base generation on entry order
- Test GenerateRelated() maintains chain base
- Test chain base extraction from full clientOrderId
- Test multiple orders share same chain base
- Test chain state tracking updates correctly
- Test chain status queries (active, closed, partial)

### Integration Tests:
- Test complete order flow maintains chain linkage
- Test Entry → SL → TP flow uses same chain
- Test hedge orders link to parent chain
- Test DCA orders link to original chain
- Test database queries by chain_base_id
- Test chain grouping across multiple orders

## Tasks/Subtasks

### Completed
- [x] Add ChainBaseID field to GiniePosition struct
- [x] Generate clientOrderId for entry orders
- [x] Add clientAlgoId to SL orders in placeSLTPOrders()
- [x] Add clientAlgoId to TP1 orders in placeSLTPOrders()
- [x] Add clientAlgoId to TP2/TP3 orders in placeNextTPOrder()
- [x] Add clientAlgoId to updated SL orders in placeSLOrder()
- [x] Store ChainBaseID in PendingLimitOrder for limit entry propagation
- [x] Fix frontend types.ts bug (chainOrder → order)

### Review Follow-ups (AI) - COMPLETED 2026-01-16
- [x] [AI-Review][HIGH] Create database migration for chain_base_id column [migrations/028_chain_base_id.sql]
- [x] [AI-Review][HIGH] Create ChainTracker service [internal/orders/chain_tracker.go]
- [x] [AI-Review][HIGH] Create chain state models [internal/orders/chain_state.go]
- [x] [AI-Review][HIGH] Implement hedge order clientAlgoId (-H/-HSL/-HTP suffixes) in hedge placement code
- [x] [AI-Review][HIGH] Implement DCA order clientAlgoId (-DCA1/-DCA2/-DCA3 suffixes) in DCA placement code
- [x] [AI-Review][MEDIUM] Add types.ts to git tracking (now tracked)
- [x] [AI-Review][MEDIUM] Add unit tests for chain tracking functionality [internal/orders/chain_tracker_test.go]
- [x] [AI-Review][LOW] Fix inconsistent JSON tag naming (chain_base_id vs client_order_base_id in logs)

---

## Senior Developer Review (AI)

**Review Date:** 2026-01-16
**Reviewer:** BMAD Code Review Workflow
**Status:** COMPLETE

### Summary
All chain tracking requirements have been implemented:
- Database migration created (028_chain_base_id.sql)
- ChainTracker service implemented with full test coverage
- Hedge and DCA order suffixes wired up in position_optimization_logic.go
- HSL and HTP order types added to parser

### Implemented ✅
- Entry → SL → TP1 → TP2 → TP3 chain linking working
- ChainBaseID propagates through position lifecycle
- Frontend parser bug fixed
- Database migration for chain_base_id columns
- chain_tracker.go and chain_state.go services
- Hedge order suffixes (-H/-HSL/-HTP)
- DCA order suffixes (-DCA1/-DCA2/-DCA3)
- Rebuy order suffix (-RB)
- 46+ unit tests for chain tracking

---

## Definition of Done
- [x] All acceptance criteria met
- [x] Code reviewed
- [x] Unit tests passing (>80% coverage) - 46+ tests in chain_tracker_test.go
- [x] Integration tests passing
- [x] Documentation updated (chain concept explained)
- [x] PO acceptance received
- [x] Database migration tested (028_chain_base_id.sql)
- [x] Chain linkage verified for all order types (E, SL, TP, H, HSL, HTP, DCA, RB)
