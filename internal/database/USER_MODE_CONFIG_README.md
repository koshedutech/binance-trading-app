# User Mode Configuration Repository

## Overview

This repository provides database operations for storing and retrieving per-user trading mode configurations in the Binance Trading Bot. Each user can customize their trading modes (ultra_fast, scalp, swing, position, scalp_reentry) with unique settings.

## Architecture

### Database Schema

**Table: `user_mode_configs`**

```sql
CREATE TABLE user_mode_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode_name VARCHAR(50) NOT NULL CHECK (mode_name IN ('ultra_fast', 'scalp', 'swing', 'position', 'scalp_reentry')),
    enabled BOOLEAN NOT NULL DEFAULT true,
    config_json JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, mode_name)
);
```

**Indexes:**
- `idx_user_mode_configs_user_id` - Fast lookups by user
- `idx_user_mode_configs_mode_name` - Fast lookups by mode
- `idx_user_mode_configs_enabled` - Fast filtering by enabled status
- `idx_user_mode_configs_config_json` - GIN index for JSON queries

### Data Model

**Go Struct:**

```go
type UserModeConfig struct {
    ID         string `json:"id"`
    UserID     string `json:"user_id"`
    ModeName   string `json:"mode_name"`
    Enabled    bool   `json:"enabled"`
    ConfigJSON []byte `json:"config_json"` // Contains autopilot.ModeFullConfig
    CreatedAt  string `json:"created_at"`
    UpdatedAt  string `json:"updated_at"`
}
```

## Repository Methods

### Read Operations

#### `GetUserModeConfig(ctx, userID, modeName) (*UserModeConfig, error)`
- Retrieves a specific mode configuration for a user
- Returns `nil, nil` if config doesn't exist (not an error)
- Returns `config, nil` if found
- Returns `nil, error` only for database errors

**When to use:** Get a single mode config when you know the mode name

**Example:**
```go
config, err := repo.GetUserModeConfig(ctx, "user-123", "scalp")
if err != nil {
    return err // Database error
}
if config == nil {
    // Use system defaults
}
```

#### `GetAllUserModeConfigs(ctx, userID) (map[string]*UserModeConfig, error)`
- Retrieves all mode configurations for a user
- Returns empty map if no configs exist (not an error)
- Returns map[modeName]*UserModeConfig if found

**When to use:** Loading all mode configs at once (more efficient than multiple calls)

**Example:**
```go
allConfigs, err := repo.GetAllUserModeConfigs(ctx, "user-123")
if err != nil {
    return err
}
for modeName, config := range allConfigs {
    // Process each config
}
```

#### `GetEnabledUserModes(ctx, userID) ([]string, error)`
- Returns list of mode names that are enabled
- Returns empty slice if no modes are enabled

**When to use:** Checking which modes are active before scanning/trading

**Example:**
```go
enabledModes, err := repo.GetEnabledUserModes(ctx, "user-123")
// enabledModes = ["scalp", "swing", "position"]
```

#### `IsModeEnabledForUser(ctx, userID, modeName) (bool, error)`
- Checks if a specific mode is enabled
- Returns `false, nil` if config doesn't exist

**When to use:** Quick check before executing a trade

**Example:**
```go
enabled, err := repo.IsModeEnabledForUser(ctx, "user-123", "scalp")
if err != nil {
    return err
}
if !enabled {
    return errors.New("scalp mode is disabled")
}
```

### Write Operations

#### `SaveUserModeConfig(ctx, userID, modeName, enabled, configJSON) error`
- Creates or updates a mode configuration (UPSERT)
- Validates JSON before saving
- If config exists, it will be replaced

**When to use:** Creating new config or full update of existing config

**Example:**
```go
configJSON, _ := json.Marshal(modeFullConfig)
err := repo.SaveUserModeConfig(ctx, "user-123", "scalp", true, configJSON)
```

#### `UpdateUserModeConfig(ctx, userID, modeName, enabled, configJSON) error`
- Updates an existing mode configuration
- Returns error if config doesn't exist
- Full replacement of config JSON

**When to use:** When you know config exists and want explicit error if it doesn't

**Example:**
```go
configJSON, _ := json.Marshal(modifiedConfig)
err := repo.UpdateUserModeConfig(ctx, "user-123", "scalp", true, configJSON)
// Returns error if config doesn't exist
```

#### `UpdateUserModeEnabled(ctx, userID, modeName, enabled) error`
- Updates only the enabled flag (partial update)
- Does NOT modify config JSON
- Returns error if config doesn't exist

**When to use:** Toggle mode on/off without changing settings

**Example:**
```go
err := repo.UpdateUserModeEnabled(ctx, "user-123", "scalp", false)
```

#### `InitializeUserModeConfigs(ctx, userID, defaultConfigs) error`
- Creates default configurations for multiple modes
- Uses ON CONFLICT DO NOTHING - will NOT overwrite existing configs
- Atomic operation (uses transaction)

**When to use:** First-time user setup or ensuring defaults exist

**Example:**
```go
defaultConfigs := map[string][]byte{
    "scalp": scalpJSON,
    "swing": swingJSON,
}
err := repo.InitializeUserModeConfigs(ctx, "user-123", defaultConfigs)
```

#### `BulkUpdateUserModeConfigs(ctx, userID, configs) error`
- Updates multiple mode configs in a single transaction
- Atomic operation - all or nothing
- Performs upsert for each mode

**When to use:** Applying configuration profile changes across all modes

**Example:**
```go
updates := map[string]*UserModeConfig{
    "scalp": {Enabled: true, ConfigJSON: scalpJSON},
    "swing": {Enabled: false, ConfigJSON: swingJSON},
}
err := repo.BulkUpdateUserModeConfigs(ctx, "user-123", updates)
```

### Delete Operations

#### `DeleteUserModeConfig(ctx, userID, modeName) error`
- Removes a specific mode configuration
- Returns error if config doesn't exist
- After deletion, system will use defaults

**When to use:** Reset a mode to system defaults

**Example:**
```go
err := repo.DeleteUserModeConfig(ctx, "user-123", "scalp")
```

#### `DeleteAllUserModeConfigs(ctx, userID) error`
- Removes all mode configurations for a user
- Does not error if no configs exist

**When to use:** Reset all user settings to defaults

**Example:**
```go
err := repo.DeleteAllUserModeConfigs(ctx, "user-123")
```

## Integration with Autopilot Package

The `config_json` column stores a marshaled `autopilot.ModeFullConfig` struct. Here's how to work with it:

### Saving a Config

```go
import (
    "encoding/json"
    "github.com/yourusername/binance-trading-bot/internal/autopilot"
)

// Get default settings
defaultSettings := autopilot.GetDefaultSettings()
scalpConfig := defaultSettings.ModeConfigs.Scalp

// Modify settings
scalpConfig.Size.BaseSizeUSD = 300.0
scalpConfig.Size.Leverage = 10
scalpConfig.Confidence.MinConfidence = 65.0

// Marshal to JSON
configJSON, err := json.Marshal(scalpConfig)
if err != nil {
    return err
}

// Save to database
err = repo.SaveUserModeConfig(ctx, userID, "scalp", true, configJSON)
```

### Loading a Config with Fallback to Defaults

```go
// Try to get user's custom config
userConfig, err := repo.GetUserModeConfig(ctx, userID, "scalp")
if err != nil {
    return err
}

var modeConfig *autopilot.ModeFullConfig

if userConfig == nil {
    // User has no custom config - use system defaults
    defaultSettings := autopilot.GetDefaultSettings()
    modeConfig = defaultSettings.ModeConfigs.Scalp
} else {
    // Unmarshal user's custom config
    var config autopilot.ModeFullConfig
    if err := json.Unmarshal(userConfig.ConfigJSON, &config); err != nil {
        return err
    }
    modeConfig = &config
}

// Use modeConfig for trading decisions
```

## Migration

To create the table, add this to your migration sequence:

```go
import "github.com/yourusername/binance-trading-bot/internal/database"

// In your migration code
db := database.NewDB(pool)
if err := db.MigrateUserModeConfigs(ctx); err != nil {
    log.Fatal(err)
}
```

The migration file is located at: `internal/database/db_user_mode_config_migration.go`

## Error Handling Philosophy

This repository follows a specific error handling pattern:

1. **Not Found ≠ Error**: When a config doesn't exist, methods return `nil` instead of an error
   - This allows easy fallback to system defaults
   - Errors are reserved for actual database failures

2. **Validation**: JSON validation happens at repository layer
   - Business logic validation should happen at service/handler layer

3. **Explicit vs Implicit**:
   - `GetUserModeConfig`: Returns nil if not found (implicit)
   - `UpdateUserModeConfig`: Returns error if not found (explicit)
   - Choose based on whether you want to handle missing config as an error

## Performance Considerations

### Best Practices

1. **Batch Reads**: Use `GetAllUserModeConfigs` instead of multiple `GetUserModeConfig` calls
   ```go
   // Good
   allConfigs, _ := repo.GetAllUserModeConfigs(ctx, userID)
   scalpConfig := allConfigs["scalp"]
   swingConfig := allConfigs["swing"]

   // Bad
   scalpConfig, _ := repo.GetUserModeConfig(ctx, userID, "scalp")
   swingConfig, _ := repo.GetUserModeConfig(ctx, userID, "swing")
   ```

2. **Batch Writes**: Use `BulkUpdateUserModeConfigs` for multi-mode updates
   ```go
   // Good - Single transaction
   repo.BulkUpdateUserModeConfigs(ctx, userID, allUpdates)

   // Bad - Multiple transactions
   for mode, config := range allUpdates {
       repo.SaveUserModeConfig(ctx, userID, mode, ...)
   }
   ```

3. **Partial Updates**: Use `UpdateUserModeEnabled` for simple toggles
   ```go
   // Good - Only updates enabled flag
   repo.UpdateUserModeEnabled(ctx, userID, "scalp", false)

   // Bad - Full config replacement just to change enabled
   repo.UpdateUserModeConfig(ctx, userID, "scalp", false, configJSON)
   ```

### Indexing Strategy

The table has optimized indexes for common query patterns:

- User lookup: `user_id` (covered by unique constraint)
- Mode filtering: `mode_name`
- Enabled filtering: `(user_id, enabled)` composite
- JSON queries: GIN index on `config_json`

## Common Patterns

### Pattern 1: First-Time User Setup

```go
func SetupNewUser(ctx context.Context, repo *Repository, userID string) error {
    // Get system defaults
    defaults := autopilot.GetDefaultSettings()

    // Prepare configs
    defaultConfigs := make(map[string][]byte)
    modes := []string{"ultra_fast", "scalp", "swing", "position", "scalp_reentry"}

    for _, mode := range modes {
        var config *autopilot.ModeFullConfig
        switch mode {
        case "ultra_fast":
            config = defaults.ModeConfigs.UltraFast
        case "scalp":
            config = defaults.ModeConfigs.Scalp
        // ... other modes
        }

        configJSON, _ := json.Marshal(config)
        defaultConfigs[mode] = configJSON
    }

    return repo.InitializeUserModeConfigs(ctx, userID, defaultConfigs)
}
```

### Pattern 2: Config with Fallback Helper

```go
func GetModeConfigWithDefaults(ctx context.Context, repo *Repository, userID, modeName string) (*autopilot.ModeFullConfig, error) {
    userConfig, err := repo.GetUserModeConfig(ctx, userID, modeName)
    if err != nil {
        return nil, err
    }

    if userConfig == nil {
        // Return system defaults
        defaults := autopilot.GetDefaultSettings()
        switch modeName {
        case "scalp":
            return defaults.ModeConfigs.Scalp, nil
        // ... other modes
        }
    }

    var config autopilot.ModeFullConfig
    if err := json.Unmarshal(userConfig.ConfigJSON, &config); err != nil {
        return nil, err
    }

    return &config, nil
}
```

### Pattern 3: Apply Risk Profile

```go
func ApplyConservativeProfile(ctx context.Context, repo *Repository, userID string) error {
    // Get all configs
    allConfigs, err := repo.GetAllUserModeConfigs(ctx, userID)
    if err != nil {
        return err
    }

    updates := make(map[string]*UserModeConfig)

    for modeName, config := range allConfigs {
        var modeConfig autopilot.ModeFullConfig
        json.Unmarshal(config.ConfigJSON, &modeConfig)

        // Apply conservative settings
        modeConfig.Confidence.MinConfidence += 10.0
        modeConfig.Size.SizeMultiplierHi = 1.2
        modeConfig.SLTP.StopLossPercent *= 1.5

        configJSON, _ := json.Marshal(modeConfig)
        updates[modeName] = &UserModeConfig{
            Enabled:    config.Enabled,
            ConfigJSON: configJSON,
        }
    }

    return repo.BulkUpdateUserModeConfigs(ctx, userID, updates)
}
```

## Testing

See `repository_user_mode_config_test_example.go` for comprehensive test examples.

Key testing scenarios:
- Config not found returns nil
- Save with valid JSON succeeds
- Save with invalid JSON fails
- Update enabled flag only
- Initialize defaults doesn't overwrite existing
- Cascade deletion when user is deleted
- Bulk updates are atomic

## Files

- `repository_user_mode_config.go` - Main repository implementation
- `db_user_mode_config_migration.go` - Database migration
- `repository_user_mode_config_example.go` - Usage examples
- `repository_user_mode_config_test_example.go` - Test examples
- `USER_MODE_CONFIG_README.md` - This file

## FAQ

**Q: What happens if a user has no custom config?**
A: Repository returns `nil` for that mode. Application should fall back to system defaults from `autopilot.GetDefaultSettings()`.

**Q: Can I update just one field in the config?**
A: No, you must update the entire `config_json` field. Load the full config, modify it, then save it back.

**Q: What's the difference between SaveUserModeConfig and UpdateUserModeConfig?**
A: `SaveUserModeConfig` is an upsert (creates if doesn't exist). `UpdateUserModeConfig` returns error if doesn't exist.

**Q: How do I reset a mode to defaults?**
A: Use `DeleteUserModeConfig(ctx, userID, modeName)`. After deletion, application will use system defaults.

**Q: Are bulk updates atomic?**
A: Yes, `BulkUpdateUserModeConfigs` uses a transaction. Either all updates succeed or all fail.

**Q: What happens when a user is deleted?**
A: All their mode configs are automatically deleted (ON DELETE CASCADE).

**Q: Can I query inside the JSON config?**
A: Yes, the `config_json` column is JSONB with a GIN index. You can use PostgreSQL JSON operators in custom queries.

**Q: What's the maximum size of config_json?**
A: JSONB in PostgreSQL can store up to 1GB, but typical configs are a few KB.

## Support

For questions or issues, contact the development team or create an issue in the repository.
