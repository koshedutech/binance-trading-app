# Product Requirements Document: Editable Paper Trading Balance

**Author:** John (Product Manager)
**Date:** 2026-01-02
**Version:** 1.0
**Status:** Approved

---

## Executive Summary

Enable users to customize their paper trading balance to match real-world scenarios, replacing the current hardcoded $10,000 default. Users can manually set custom balances or sync directly from their live Binance account with one click.

---

## Problem Statement

### Current Pain Points

1. **Unrealistic Testing Environment**: Users with $500 or $50,000 real accounts are forced to test strategies with a hardcoded $10,000 paper balance, leading to invalid strategy validation
2. **Manual Calculation Overhead**: Users must mentally adjust risk calculations and position sizes because paper balance doesn't match their real capital
3. **Poor Onboarding Experience**: New users enabling paper mode have no control over their testing environment parameters
4. **Workflow Friction**: No easy way to keep paper balance synchronized with evolving real account balance

### User Impact

- **Power Users**: Cannot accurately simulate portfolio-specific scenarios
- **New Traders**: Confused why paper balance doesn't match their actual Binance balance
- **Strategy Testers**: Wasted time recalculating position sizes for realistic constraints

---

## User Stories

### Primary User Stories

**US-1: Manual Paper Balance Configuration**
> As a trader with a $2,500 real account,
> I want to set my paper trading balance to $2,500,
> So that I can test strategies with realistic capital constraints.

**US-2: Sync from Real Balance**
> As a trader who frequently switches between paper and real trading,
> I want to click "Sync from Real Balance" to automatically copy my live Binance balance,
> So that I don't have to manually enter my current balance each time.

**US-3: Persistent Balance Storage**
> As a returning user,
> I want my custom paper balance to persist across sessions,
> So that I don't have to reconfigure it every time I log in.

**US-4: Separate Spot and Futures Balances**
> As a trader using both Spot and Futures markets,
> I want independent paper balances for each trading type,
> So that my testing environments accurately reflect my real account structure.

### Secondary User Stories (Future Scope)

- **US-5**: Balance change history/audit log
- **US-6**: Scheduled auto-sync (daily/weekly)
- **US-7**: Configurable validation limits (custom max >$1M)

---

## Requirements

### Functional Requirements

#### FR-1: Manual Balance Entry
- Users can input custom paper balance via text field
- Range validation: $10 minimum, $1,000,000 maximum
- Input formatting: Display as currency with thousands separator
- Real-time validation feedback (client-side)
- Server-side validation enforcement

#### FR-2: Sync from Real Balance
- One-click button to fetch current USDT balance from Binance API
- Separate sync for Spot trading and Futures trading
- Button states: Default, Loading, Success, Error
- Error handling for:
  - Missing API credentials
  - Binance API failures (timeout, rate limit, auth error)
  - Network connectivity issues

#### FR-3: Balance Persistence
- Paper balance stored in PostgreSQL database
- Per-user, per-trading-type storage
- Default value: $10,000 for backward compatibility
- Balance persists across sessions and browser refreshes

#### FR-4: Trading Integration
- All paper trades (Spot and Futures) use custom balance
- Balance decrements on position entry
- Balance increments on position exit (profit/loss applied)
- Fallback to $10,000 if database value is null (defensive coding)

#### FR-5: UI/UX Requirements
- Paper balance controls visible ONLY in paper trading mode (`dry_run_mode = true`)
- Clear visual distinction between Spot and Futures balance controls
- Success/error toast notifications for all balance operations
- Disabled state for sync button when API keys not configured
- Loading spinner during sync operation

### Non-Functional Requirements

#### NFR-1: Performance
- Sync operation completes within 5 seconds under normal conditions
- Database query for balance retrieval < 100ms
- UI remains responsive during sync (async operation)

#### NFR-2: Security
- All API endpoints require authentication (JWT token)
- User can only modify their own paper balance
- No SQL injection vulnerabilities in balance update queries
- Binance API keys stored securely (existing auth service)

#### NFR-3: Data Integrity
- Decimal precision: DECIMAL(20,8) matches Binance precision
- No floating-point precision loss in JSON serialization
- Database constraints prevent negative balances
- Transaction safety for sync operation (atomic read-write)

#### NFR-4: Reliability
- Sync operation is idempotent (safe to retry)
- Database migration has tested rollback script
- Error states don't leave balance in corrupted state
- Graceful degradation if Binance API unavailable

#### NFR-5: Usability
- Balance input accepts common formats: "5000", "5,000", "5000.50"
- Clear error messages guide user to resolution
- No confirmation modal for sync (low-friction workflow)
- Accessible: ARIA labels, keyboard navigation, screen reader support

---

## Success Metrics

### Primary KPIs

1. **Adoption Rate**: % of paper mode users who customize their balance within first week
   - Target: >40% adoption

2. **Sync Usage**: Ratio of sync operations vs manual edits
   - Hypothesis: Sync is preferred method (>60% of balance changes)

3. **Error Rate**: % of sync operations that fail
   - Target: <5% error rate (indicates Binance API reliability)

### Secondary Metrics

4. **Session Persistence**: % of users who return to find balance unchanged
   - Validates database persistence working correctly

5. **Support Tickets**: Reduction in paper trading confusion tickets
   - Baseline from current quarter, measure post-launch

---

## MVP Scope

### In Scope (Release 1.0)

- ✅ Manual balance entry with validation
- ✅ Sync from real Binance balance (Spot & Futures)
- ✅ Database persistence per user/trading-type
- ✅ Settings page UI (conditional visibility)
- ✅ Error handling for API failures
- ✅ Toast notifications for user feedback

### Out of Scope (Future Releases)

- ❌ Balance change history/audit log (v2.0)
- ❌ Scheduled auto-sync (v2.1)
- ❌ Configurable validation limits >$1M (v2.2)
- ❌ Multi-currency support beyond USDT (v3.0)
- ❌ Paper balance sharing/templates (v3.0)

---

## Dependencies

### Technical Dependencies
- Existing Binance API integration (Spot & Futures)
- Existing authentication service (JWT)
- PostgreSQL database with `user_trading_configs` table

### Team Dependencies
- Backend: Database migration + API endpoints
- Frontend: React components + API service integration
- DevOps: Database migration deployment process

---

## Risks and Mitigations

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Binance API rate limiting during sync | High | Medium | Implement exponential backoff, cache balance for 5 min |
| Users set unrealistic balances ($1M when real is $500) | Low | High | Education: Add tooltip explaining sync benefits |
| Database migration fails in production | High | Low | Tested rollback script, staged rollout |
| Precision loss in balance calculations | High | Low | Use `decimal.Decimal` library, comprehensive unit tests |

---

## Open Questions

1. **Resolved**: Should sync require confirmation modal? → No, low-friction preferred
2. **Resolved**: $1M hard cap sufficient? → Yes for MVP, raise in v2 if needed
3. **Pending**: Analytics tracking - which balance change events to log?

---

## Approval Sign-Off

- **Product Manager (John)**: ✅ Approved 2026-01-02
- **Business Analyst (Mary)**: _Pending Review_
- **Architect (Winston)**: _Pending Review_
- **UX Designer (Sally)**: _Pending Review_

---

## Appendix

### Competitive Analysis
- **TradingView**: Custom starting capital $100 - $1M+
- **eToro**: One-click "Match Real Account" feature
- **MetaTrader**: Persistent paper balance per account

### User Research
- Survey data: 68% of paper mode users want balance customization
- Support tickets: 23 tickets/month related to "paper balance doesn't match real"
