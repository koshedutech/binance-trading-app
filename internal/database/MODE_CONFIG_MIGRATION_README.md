# Mode Configuration Data Migration (Story 4.9)

## Overview

This migration script automatically migrates mode configurations from `autopilot_settings.json` to the PostgreSQL database for all users.

## Purpose

- **Before**: Mode configurations were stored in a single `autopilot_settings.json` file and shared across all users
- **After**: Each user has their own customizable mode configurations stored in the `user_mode_configs` table
- **Migration**: Copies the default mode configs from the JSON file to the database for all existing users

## How It Works

### Automatic Migration on Startup

The migration runs automatically when the application starts:

1. **File Check**: Checks if `autopilot_settings.json` exists
   - If not found: Logs a message and continues (not an error)
   - If found: Proceeds with migration

2. **JSON Parsing**: Reads and parses the settings file
   - Extracts the `mode_configs` section
   - Example modes: `ultra_fast`, `scalp`, `swing`, `position`, `scalp_reentry`

3. **User Enumeration**: Queries all users from the `users` table

4. **Config Migration**: For each user and each mode:
   - Checks if the user already has a config for that mode
   - If exists: Skips (preserves user customizations)
   - If not exists: Inserts the default config from the JSON file

5. **Logging**: Detailed logging with `[MODE-MIGRATE]` prefix
   - Tracks which configs were migrated
   - Tracks which configs were skipped
   - Reports final counts

### Idempotency

The migration is **idempotent** - safe to run multiple times:

```sql
-- Before inserting, checks if config exists:
SELECT EXISTS(SELECT 1 FROM user_mode_configs WHERE user_id = $1 AND mode_name = $2)
```

- **First run**: Migrates all configs for all users
- **Subsequent runs**: Skips all existing configs, only migrates new users or missing modes
- **User customizations**: Never overwritten

## File Structure

### Source: `autopilot_settings.json`

```json
{
  "mode_configs": {
    "ultra_fast": {
      "mode_name": "ultra_fast",
      "enabled": false,
      "timeframe": { ... },
      "confidence": { ... },
      "size": { ... },
      "circuit_breaker": { ... },
      "sltp": { ... }
    },
    "scalp": {
      "mode_name": "scalp",
      "enabled": true,
      ...
    }
  }
}
```

### Target: `user_mode_configs` table

```sql
CREATE TABLE user_mode_configs (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    mode_name VARCHAR(50),
    enabled BOOLEAN,
    config_json JSONB,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    UNIQUE(user_id, mode_name)
);
```

## Migration Flow

```
┌─────────────────────────────────┐
│  autopilot_settings.json        │
│  ┌─────────────────────────┐    │
│  │ mode_configs:           │    │
│  │  - ultra_fast           │    │
│  │  - scalp                │    │
│  │  - swing                │    │
│  │  - position             │    │
│  │  - scalp_reentry        │    │
│  └─────────────────────────┘    │
└─────────────────────────────────┘
           │
           │ Migration Script
           ▼
┌─────────────────────────────────┐
│  Database: user_mode_configs    │
│  ┌─────────────────────────┐    │
│  │ User 1 → All 5 modes    │    │
│  │ User 2 → All 5 modes    │    │
│  │ User 3 → All 5 modes    │    │
│  │ ...                     │    │
│  └─────────────────────────┘    │
└─────────────────────────────────┘
```

## Log Output Examples

### Successful Migration

```
[MODE-MIGRATE] Starting mode configs migration from autopilot_settings.json...
[MODE-MIGRATE] Found 5 mode configurations in settings file
[MODE-MIGRATE] Found 3 users to migrate configs for
[MODE-MIGRATE] Migrated ultra_fast for user user1@example.com (enabled: false)
[MODE-MIGRATE] Migrated scalp for user user1@example.com (enabled: true)
[MODE-MIGRATE] Migrated swing for user user1@example.com (enabled: false)
[MODE-MIGRATE] Migrated position for user user1@example.com (enabled: false)
[MODE-MIGRATE] Migrated scalp_reentry for user user1@example.com (enabled: false)
[MODE-MIGRATE] Migrated ultra_fast for user user2@example.com (enabled: false)
...
[MODE-MIGRATE] Migration complete: 15 configs migrated, 0 configs skipped (already existed)
```

### Subsequent Run (Idempotent)

```
[MODE-MIGRATE] Starting mode configs migration from autopilot_settings.json...
[MODE-MIGRATE] Found 5 mode configurations in settings file
[MODE-MIGRATE] Found 3 users to migrate configs for
[MODE-MIGRATE] Skipping ultra_fast for user admin@binance-bot.local (already exists)
[MODE-MIGRATE] Skipping scalp for user admin@binance-bot.local (already exists)
...
[MODE-MIGRATE] Migration complete: 0 configs migrated, 15 configs skipped (already existed)
```

### File Not Found (Not an Error)

```
[MODE-MIGRATE] Settings file not found at autopilot_settings.json, skipping migration
```

## Error Handling

### Non-Fatal Errors (Logged, Migration Continues)
- Settings file doesn't exist
- No `mode_configs` section in JSON
- No users in database

### Fatal Errors (Migration Stops)
- Failed to read settings file
- Failed to parse JSON
- Failed to query users table
- Failed to check existing configs
- Failed to insert config

## Manual Migration

If you need to manually trigger the migration or re-run it:

```go
import "context"

// In your database initialization code
db, err := database.NewDB(config)
if err != nil {
    log.Fatal(err)
}

ctx := context.Background()
err = db.MigrateModeConfigsFromJSON(ctx, "autopilot_settings.json")
if err != nil {
    log.Printf("Migration failed: %v", err)
}
```

## Testing the Migration

### Before Migration

```sql
-- Check if users exist
SELECT id, email FROM users;

-- Check mode configs (should be empty)
SELECT * FROM user_mode_configs;
```

### After Migration

```sql
-- Verify all users have configs
SELECT
    u.email,
    COUNT(umc.id) as config_count
FROM users u
LEFT JOIN user_mode_configs umc ON u.id = umc.user_id
GROUP BY u.email;

-- Check specific mode configs
SELECT
    u.email,
    umc.mode_name,
    umc.enabled,
    umc.created_at
FROM user_mode_configs umc
JOIN users u ON umc.user_id = u.id
ORDER BY u.email, umc.mode_name;
```

## Rollback

To rollback the migration (remove all migrated configs):

```sql
-- WARNING: This removes ALL user mode configs
DELETE FROM user_mode_configs;
```

To rollback for a specific user:

```sql
DELETE FROM user_mode_configs WHERE user_id = '<user-id>';
```

## Integration with Application Lifecycle

### Startup Sequence

1. **Database Connection**: `NewDB(config)`
2. **Schema Migrations**: `db.RunMigrations(ctx)`
   - Creates tables (including `user_mode_configs`)
   - Creates indexes
3. **Data Migration**: `db.MigrateModeConfigsFromJSON(ctx, "autopilot_settings.json")`
   - Migrates mode configs from JSON to database
4. **Application Start**: Application begins using database-stored configs

### User Onboarding (Story 4.8)

When a new user registers:
- If migration has already run: New user gets configs from `autopilot_settings.json`
- User can then customize their configs via API
- Customizations are stored per-user in `user_mode_configs` table

## Related Stories

- **Story 4.1**: Database schema creation (`user_mode_configs` table)
- **Story 4.2**: Repository layer (CRUD operations)
- **Story 4.3**: API handlers (HTTP endpoints)
- **Story 4.8**: User onboarding (initial config setup)
- **Story 4.9**: Data migration (this story)
- **Story 4.10**: Remove JSON file dependency

## Future Considerations

After all users have been migrated and Story 4.10 is complete:
- The `autopilot_settings.json` file will only be used as a template for new users
- Existing users will have their configs in the database
- No fallback to JSON file will be needed
