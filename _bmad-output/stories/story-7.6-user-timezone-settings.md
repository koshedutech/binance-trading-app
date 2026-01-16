# Story 7.6: User Timezone Settings
**Epic:** Epic 7 - Client Order ID & Trade Lifecycle Tracking
**Sprint:** Sprint 7
**Story Points:** 5
**Priority:** P1
**Status:** done

## User Story
As a trader, I want to configure my timezone preference so that the date component in clientOrderId and all timestamps reflect my local time, not UTC or server time.

## Acceptance Criteria
- [x] User settings page has timezone preference field
- [x] Default timezone: Asia/Kolkata (GMT+5:30) from Docker container TZ variable
- [x] Preset timezone options: India (IST), Cambodia (ICT), UTC
- [x] Custom option: Full IANA timezone database selector (via presets table)
- [x] Timezone used for date component in clientOrderId (DDMMM)
- [x] Timezone used for sequence reset timing (midnight rollover)
- [x] Timezone used for Trade Lifecycle display timestamps
- [x] Docker container TZ variable as system fallback
- [x] Timezone persisted in database per user
- [x] API endpoint to update user timezone

## Technical Approach

1. **Database Schema**:
   ```sql
   ALTER TABLE users ADD COLUMN timezone VARCHAR(50) DEFAULT 'Asia/Kolkata';

   CREATE TABLE timezone_presets (
       id SERIAL PRIMARY KEY,
       display_name VARCHAR(100),      -- "India Standard Time (IST)"
       tz_identifier VARCHAR(50),      -- "Asia/Kolkata"
       gmt_offset VARCHAR(10),         -- "+05:30"
       is_default BOOLEAN DEFAULT false
   );

   INSERT INTO timezone_presets VALUES
       (1, 'India Standard Time (IST)', 'Asia/Kolkata', '+05:30', true),
       (2, 'Indochina Time (ICT)', 'Asia/Phnom_Penh', '+07:00', false);
   ```

2. **Timezone Usage Points**:
   - **ClientOrderId Date**: `time.Now().In(userTimezone).Format("02Jan")`
   - **Sequence Reset**: Redis key based on date in user timezone
   - **Trade Lifecycle Display**: Convert all UTC times to user timezone
   - **Dashboard Timestamps**: Display in user timezone with offset indicator

3. **Settings UI Components**:
   ```tsx
   <TimezoneSelector
       presets={[
           { name: 'India Standard Time (IST)', tz: 'Asia/Kolkata', offset: '+05:30' },
           { name: 'Indochina Time (ICT)', tz: 'Asia/Phnom_Penh', offset: '+07:00' }
       ]}
       customTimezones={allIANATimezones}
       currentTimezone={user.timezone}
       onChange={handleTimezoneChange}
   />
   ```

4. **Backend Integration**:
   - Load user timezone on authentication
   - Pass timezone to ClientOrderIdGenerator
   - Store timezone in user session/context
   - Apply timezone to all timestamp conversions

5. **Fallback Strategy**:
   - User preference (database)
   - Docker container TZ environment variable
   - System default (Asia/Kolkata)
   - UTC (last resort)

6. **IANA Timezone Support**:
   - Full list of valid IANA timezone identifiers
   - Search/autocomplete for custom selection
   - Validate timezone identifier before saving
   - Display current offset (e.g., "+05:30")

## Dependencies
- **Blocked By:**
  - Story 7.1: Client Order ID Generation (needs timezone)
  - Story 7.2: Daily Sequence Storage (midnight reset)
- **Blocks:**
  - Story 7.5: Trade Lifecycle Tab UI (display timestamps)
  - Story 7.10: Edge Case Test Suite (midnight rollover tests)

## Files to Create/Modify

### Files to Create:
- `migrations/000X_add_user_timezone.sql` - Database migration for timezone column and presets table
- `web/src/components/Settings/TimezoneSelector.tsx` - Timezone selector UI component
- `web/src/services/timezoneApi.ts` - API client for timezone endpoints
- `internal/api/timezone_handlers.go` - API handlers for timezone CRUD operations
- `internal/utils/timezone.go` - Timezone utilities (load, validate, format)

### Files to Modify:
- `internal/database/models.go` - Add Timezone field to User model
- `internal/database/user_repository.go` - CRUD methods for timezone preference
- `internal/orders/client_order_id.go` - Accept timezone parameter in constructor
- `web/src/components/Settings/ProfileSettings.tsx` - Add timezone selector to settings page
- `web/src/contexts/AuthContext.tsx` - Store user timezone in auth context
- `main.go` - Load timezone from Docker TZ variable as fallback

## Testing Requirements

### Unit Tests:
- Test timezone loading from database
- Test fallback cascade (user → container → default → UTC)
- Test IANA timezone validation
- Test timezone conversion accuracy
- Test preset timezone options
- Test custom timezone selection
- Test invalid timezone handling

### Integration Tests:
- Test clientOrderId date uses user timezone
- Test sequence reset at user's midnight (not UTC)
- Test Trade Lifecycle timestamps display correctly
- Test timezone update propagates to all displays
- Test concurrent users with different timezones

### Edge Case Tests:
- Test DST (Daylight Saving Time) transitions
- Test midnight rollover in user timezone
- Test timezone change during active trading
- Test year boundary with timezone offset

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Documentation updated (timezone configuration guide)
- [ ] PO acceptance received
- [ ] Database migration tested
- [ ] Settings UI functional and intuitive
- [ ] All timestamp displays use user timezone
- [ ] Midnight rollover tested with Asia/Kolkata
