# Story 11.23: Decision Engine Settings Structure

**Epic:** Epic 11: Position Decision Engine
**Priority:** P0 (Critical - Follows Settings Lifecycle Rule)
**Status:** ready-for-dev
**Dependencies:** Stories 11.1 (Redis State Management), 11.15 (Additive Score Calculator), 11.18 (Blocking Reason Tracker)

---

## Goal Statement

Create a comprehensive settings structure for the Decision Engine that follows the established Settings Lifecycle Rule pattern. This mirrors the mode_configs pattern used for trading modes but applies settings to decision engine strategies (trend_following, mean_reversion, breakout, range_trading). Settings flow from default-settings.json → user_settings table → Redis cache → system reads.

---

## Acceptance Criteria

- [x] **AC1: Settings Structure Mirrors mode_configs Pattern** - Decision engine settings use same layered pattern with strategy-level granularity instead of mode-level
- [x] **AC2: Independent Strategy Settings** - Each strategy (trend_following, mean_reversion, breakout, range_trading) has complete, independent configuration
- [x] **AC3: User Settings Persistence** - Settings stored in user_settings table as JSONB column following existing pattern
- [x] **AC4: Redis Cache Integration** - On login, user's settings loaded to Redis with proper key structure and TTL
- [x] **AC5: CRUD API Endpoints** - Complete API endpoints for reading and updating strategy settings with write-through caching

---

## Implementation Tasks

### Task 1: Design Decision Engine Settings Data Structures
**Description:** Create Go structs for decision engine settings that parallel mode configuration pattern

**Subtasks:**
- Define `DecisionEngineSettings` root struct containing all strategies
- Define `StrategySettings` struct for each strategy (trend_following, mean_reversion, breakout, range_trading)
- Define `MarketRegimeConfig` struct with regime detection parameters
- Define `IndicatorsConfig` struct with segment-specific indicator thresholds
- Define `EntryExitConditions` struct with entry/exit trigger rules
- Define `ScoringConfig` struct with component weights (technical, context, llm, history)
- Define `CalibrationConfig` struct with risk-adjusted parameters
- Create `StrategyType` enum for strategy identification

**Files to Create/Modify:**
- Create: `internal/models/decision_engine_settings.go`
- Modify: `internal/models/models_user.go` (add decision_engine_settings field)

### Task 2: Implement Decision Engine Settings Service
**Description:** Build core service for loading, managing, and caching decision engine settings

**Subtasks:**
- Implement `DecisionEngineSettingsService` struct with dependencies on repository and cache service
- Implement `LoadUserSettings()` - retrieve user's decision engine settings from database
- Implement `GetSettingsFromCache()` - retrieve cached settings for fast access
- Implement `SaveSettingsToDatabase()` - persist settings to user_settings table
- Implement `UpdateStrategySettings()` - update single strategy configuration
- Implement `ResetToDefaults()` - reset all or specific strategy to default-settings.json values
- Implement `ValidateSettings()` - validate settings against constraints and ranges
- Add proper error handling and logging

**Files to Create/Modify:**
- Create: `internal/services/decision_engine_settings_service.go`
- Modify: `internal/api/handlers_decisions.go`

### Task 3: Integrate with Settings Lifecycle
**Description:** Add decision engine settings to default-settings.json and implement full lifecycle following Settings Lifecycle Rule

**Subtasks:**
- Add `decision_engine_settings` section to `default-settings.json` with complete structure for all 4 strategies
- Add `DecisionEngineSettings` Go struct to settings model in `models_user.go`
- Create database migration for `user_settings` table (if not exists) with decision_engine_settings JSONB column
- Implement settings persistence in `repository_user_settings.go` (new file following repository pattern)
- Add cache handlers in `settings_cache_service.go` for decision_engine settings extraction/merge
- Update `admin_defaults_cache.go` to include default decision engine settings
- Add initialization in `user_initialization.go` to copy decision engine defaults on new user registration

**Files to Create/Modify:**
- Modify: `default-settings.json`
- Modify: `internal/models/models_user.go`
- Create: `internal/database/repository_user_settings.go`
- Create: `internal/database/migrations/000X_add_decision_engine_settings.sql`
- Modify: `internal/cache/settings_cache_service.go`
- Modify: `internal/cache/admin_defaults_cache.go`
- Modify: `internal/database/user_initialization.go`

### Task 4: Implement Redis Cache Integration
**Description:** Store and retrieve decision engine settings in Redis for efficient access

**Subtasks:**
- Define Redis key structure: `decision_engine:settings:{userID}:{strategyType}`
- Implement `StoreDecisionEngineSettingsInRedis()` with 24-hour TTL (same as other settings)
- Implement `GetDecisionEngineSettingsFromRedis()` with cache miss handling
- Store full strategy settings as JSON in Redis
- Implement cache invalidation on settings update
- Add cleanup routines for expired settings data
- Coordinate with existing Redis key namespacing

**Files to Create/Modify:**
- Create: `internal/cache/decision_engine_settings_cache.go`
- Modify: `internal/cache/redis.go`

### Task 5: Add Decision Engine Settings Database Repository
**Description:** Implement database CRUD operations following existing repository pattern

**Subtasks:**
- Implement `GetDecisionEngineSettings()` - retrieve user's decision engine settings JSON
- Implement `GetDecisionEngineSettingsByStrategy()` - retrieve single strategy settings
- Implement `SaveDecisionEngineSettings()` - UPSERT complete decision engine settings
- Implement `UpdateDecisionEngineStrategy()` - update single strategy settings
- Implement `GetAllUsersDecisionEngineSettings()` - for admin operations
- Follow existing `repository_user_mode_config.go` pattern for consistency

**Files to Create/Modify:**
- Create: `internal/database/repository_user_settings.go`

### Task 6: Add Decision Engine Settings API Endpoints
**Description:** Expose decision engine settings via REST API with CRUD operations

**Subtasks:**
- Create `GET /api/v1/decision-engine/settings` endpoint
  - Return user's complete decision engine settings for all strategies
  - Include current and default values for comparison
- Create `GET /api/v1/decision-engine/settings/{strategy}` endpoint
  - Return settings for specific strategy only
- Create `PUT /api/v1/decision-engine/settings/{strategy}` endpoint
  - Update specific strategy settings with validation
  - Implement write-through caching pattern
  - Return updated settings
- Create `POST /api/v1/decision-engine/settings/{strategy}/reset` endpoint
  - Reset strategy to default values from default-settings.json
  - Update both database and cache
- Create `GET /api/v1/decision-engine/settings/defaults/{strategy}` endpoint
  - Return default settings for strategy from default-settings.json
- Add request validation, authorization checks, and error handling
- Implement write-through caching: update DB → update cache → return result

**Files to Create/Modify:**
- Create: `internal/api/handlers_decision_engine_settings.go`
- Modify: `internal/api/routes.go`

### Task 7: Integration with Decision Engine Components
**Description:** Connect settings to Stories 11.1, 11.15, 11.18 components

**Subtasks:**
- Update `DecisionEngine` to load strategy settings on initialization
- Update `AdditiveScoreCalculator` to read weights from decision engine settings
- Update `BlockingReasonTracker` to read threshold values from decision engine settings
- Ensure settings changes invalidate relevant cached calculations
- Add settings context to decision logging for debugging

**Files to Create/Modify:**
- Modify: `internal/decision/decision_engine.go`
- Modify: `internal/services/scoring_calculator.go`
- Modify: `internal/services/blocking_tracker_service.go`

---

## Technical Design

### Settings Lifecycle Flow

```
NEW USER REGISTRATION:
default-settings.json
  ↓
Read decision_engine_settings section
  ↓
Copy to user_settings table (JSONB)
  ↓
On first login, load to Redis cache
  ↓
System reads from Redis for all decision logic

USER UPDATES SETTINGS:
API PUT /api/v1/decision-engine/settings/{strategy}
  ↓
Update user_settings table in database
  ↓
Update Redis cache (write-through)
  ↓
Return updated settings to client

RESET/RESTORE:
User requests reset via API
  ↓
Load defaults from default-settings.json
  ↓
Replace user_settings in database
  ↓
Update Redis cache
  ↓
Confirm reset completion
```

### Data Structures

```go
// internal/models/decision_engine_settings.go

type StrategyType string

const (
    StrategyTrendFollowing StrategyType = "trend_following"
    StrategyMeanReversion  StrategyType = "mean_reversion"
    StrategyBreakout       StrategyType = "breakout"
    StrategyRangeTrading   StrategyType = "range_trading"
)

type DecisionEngineSettings struct {
    TrendFollowing  StrategySettings `json:"trend_following"`
    MeanReversion   StrategySettings `json:"mean_reversion"`
    Breakout        StrategySettings `json:"breakout"`
    RangeTrading    StrategySettings `json:"range_trading"`
    CreatedAt       time.Time        `json:"created_at"`
    UpdatedAt       time.Time        `json:"updated_at"`
    Version         int              `json:"version"` // For future migrations
}

type StrategySettings struct {
    StrategyName      string                `json:"strategy_name"`
    Enabled           bool                  `json:"enabled"`
    MarketRegime      MarketRegimeConfig    `json:"market_regime"`
    Indicators        IndicatorsConfig      `json:"indicators"`
    EntryExitRules    EntryExitConditions   `json:"entry_exit_rules"`
    ScoringWeights    ScoringConfig         `json:"scoring_weights"`
    Calibration       CalibrationConfig     `json:"calibration"`
    Timeframes        TimeframeConfig       `json:"timeframes"`
}

type MarketRegimeConfig struct {
    DetectionEnabled     bool        `json:"detection_enabled"`
    TrendStrengthMin     float64     `json:"trend_strength_min"`
    VolatilityThreshold  float64     `json:"volatility_threshold"`
    ADXMinimum          float64     `json:"adx_minimum"`
    RSIOversold         float64     `json:"rsi_oversold"`
    RSIOverbought       float64     `json:"rsi_overbought"`
}

type IndicatorsConfig struct {
    // Per-segment indicator thresholds
    StrongTrend struct {
        ADXMin              float64 `json:"adx_min"`
        EMAAlignment        float64 `json:"ema_alignment_percent"`
        RSIThreshold        float64 `json:"rsi_threshold"`
    } `json:"strong_trend"`

    MediumTrend struct {
        ADXMin              float64 `json:"adx_min"`
        EMAAlignment        float64 `json:"ema_alignment_percent"`
        RSIThreshold        float64 `json:"rsi_threshold"`
    } `json:"medium_trend"`

    WeakTrend struct {
        ADXMin              float64 `json:"adx_min"`
        EMAAlignment        float64 `json:"ema_alignment_percent"`
        RSIThreshold        float64 `json:"rsi_threshold"`
    } `json:"weak_trend"`

    Sideways struct {
        ADXMax              float64 `json:"adx_max"`
        VolumeThreshold     float64 `json:"volume_threshold"`
        BreakoutPercent     float64 `json:"breakout_percent"`
    } `json:"sideways"`
}

type EntryExitConditions struct {
    EntrySignalMinScore          float64         `json:"entry_signal_min_score"`
    EntryConfidenceThreshold     float64         `json:"entry_confidence_threshold"`
    ExitProfitTarget             float64         `json:"exit_profit_target_percent"`
    ExitStopLoss                 float64         `json:"exit_stop_loss_percent"`
    ExitTrailingStop             float64         `json:"exit_trailing_stop_percent"`
    MaxHoldDuration              string          `json:"max_hold_duration"` // e.g., "24h", "7d"
    RequireConfirmationCandles   int             `json:"require_confirmation_candles"`
}

type ScoringConfig struct {
    TechnicalWeight    float64 `json:"technical_weight"`
    ContextWeight      float64 `json:"context_weight"`
    LLMWeight          float64 `json:"llm_weight"`
    HistoryWeight      float64 `json:"history_weight"`
    MinimumThreshold   float64 `json:"minimum_threshold"`
}

type CalibrationConfig struct {
    RiskLevel               string  `json:"risk_level"` // "conservative", "moderate", "aggressive"
    PositionSizeUSD        float64 `json:"position_size_usd"`
    MaxConcurrentSignals   int     `json:"max_concurrent_signals"`
    RebalanceInterval      string  `json:"rebalance_interval"`
    DrawdownLimit          float64 `json:"drawdown_limit_percent"`
}

type TimeframeConfig struct {
    TrendTimeframe    string `json:"trend_timeframe"`
    EntryTimeframe    string `json:"entry_timeframe"`
    AnalysisTimeframe string `json:"analysis_timeframe"`
}
```

### default-settings.json Addition

```json
{
  "decision_engine_settings": {
    "trend_following": {
      "strategy_name": "trend_following",
      "enabled": true,
      "market_regime": {
        "detection_enabled": true,
        "trend_strength_min": 0.6,
        "volatility_threshold": 2.0,
        "adx_minimum": 25,
        "rsi_oversold": 30,
        "rsi_overbought": 70
      },
      "indicators": {
        "strong_trend": {
          "adx_min": 30,
          "ema_alignment_percent": 80,
          "rsi_threshold": 50
        },
        "medium_trend": {
          "adx_min": 20,
          "ema_alignment_percent": 70,
          "rsi_threshold": 45
        },
        "weak_trend": {
          "adx_min": 15,
          "ema_alignment_percent": 60,
          "rsi_threshold": 40
        },
        "sideways": {
          "adx_max": 15,
          "volume_threshold": 1.2,
          "breakout_percent": 1.5
        }
      },
      "entry_exit_rules": {
        "entry_signal_min_score": 60,
        "entry_confidence_threshold": 70,
        "exit_profit_target_percent": 5,
        "exit_stop_loss_percent": 2,
        "exit_trailing_stop_percent": 1.5,
        "max_hold_duration": "7d",
        "require_confirmation_candles": 1
      },
      "scoring_weights": {
        "technical_weight": 40,
        "context_weight": 30,
        "llm_weight": 20,
        "history_weight": 10,
        "minimum_threshold": 50
      },
      "calibration": {
        "risk_level": "moderate",
        "position_size_usd": 500,
        "max_concurrent_signals": 3,
        "rebalance_interval": "1h",
        "drawdown_limit_percent": 5
      },
      "timeframes": {
        "trend_timeframe": "4h",
        "entry_timeframe": "1h",
        "analysis_timeframe": "1d"
      }
    },
    "mean_reversion": {
      "strategy_name": "mean_reversion",
      "enabled": true,
      "market_regime": {
        "detection_enabled": true,
        "trend_strength_min": 0.3,
        "volatility_threshold": 3.0,
        "adx_minimum": 10,
        "rsi_oversold": 25,
        "rsi_overbought": 75
      },
      "indicators": {
        "strong_trend": {
          "adx_min": 0,
          "ema_alignment_percent": 0,
          "rsi_threshold": 0
        },
        "medium_trend": {
          "adx_min": 0,
          "ema_alignment_percent": 0,
          "rsi_threshold": 0
        },
        "weak_trend": {
          "adx_min": 0,
          "ema_alignment_percent": 0,
          "rsi_threshold": 0
        },
        "sideways": {
          "adx_max": 20,
          "volume_threshold": 1.0,
          "breakout_percent": 2.0
        }
      },
      "entry_exit_rules": {
        "entry_signal_min_score": 55,
        "entry_confidence_threshold": 65,
        "exit_profit_target_percent": 3,
        "exit_stop_loss_percent": 1.5,
        "exit_trailing_stop_percent": 1.0,
        "max_hold_duration": "3d",
        "require_confirmation_candles": 2
      },
      "scoring_weights": {
        "technical_weight": 50,
        "context_weight": 25,
        "llm_weight": 15,
        "history_weight": 10,
        "minimum_threshold": 50
      },
      "calibration": {
        "risk_level": "moderate",
        "position_size_usd": 300,
        "max_concurrent_signals": 2,
        "rebalance_interval": "30m",
        "drawdown_limit_percent": 3
      },
      "timeframes": {
        "trend_timeframe": "1h",
        "entry_timeframe": "15m",
        "analysis_timeframe": "4h"
      }
    },
    "breakout": {
      "strategy_name": "breakout",
      "enabled": false,
      "market_regime": {
        "detection_enabled": true,
        "trend_strength_min": 0.4,
        "volatility_threshold": 1.5,
        "adx_minimum": 15,
        "rsi_oversold": 30,
        "rsi_overbought": 70
      },
      "indicators": {
        "strong_trend": {
          "adx_min": 25,
          "ema_alignment_percent": 75,
          "rsi_threshold": 50
        },
        "medium_trend": {
          "adx_min": 15,
          "ema_alignment_percent": 65,
          "rsi_threshold": 45
        },
        "weak_trend": {
          "adx_min": 10,
          "ema_alignment_percent": 55,
          "rsi_threshold": 40
        },
        "sideways": {
          "adx_max": 15,
          "volume_threshold": 1.5,
          "breakout_percent": 1.0
        }
      },
      "entry_exit_rules": {
        "entry_signal_min_score": 65,
        "entry_confidence_threshold": 75,
        "exit_profit_target_percent": 4,
        "exit_stop_loss_percent": 2.5,
        "exit_trailing_stop_percent": 2.0,
        "max_hold_duration": "2d",
        "require_confirmation_candles": 1
      },
      "scoring_weights": {
        "technical_weight": 45,
        "context_weight": 30,
        "llm_weight": 15,
        "history_weight": 10,
        "minimum_threshold": 55
      },
      "calibration": {
        "risk_level": "moderate",
        "position_size_usd": 400,
        "max_concurrent_signals": 2,
        "rebalance_interval": "2h",
        "drawdown_limit_percent": 4
      },
      "timeframes": {
        "trend_timeframe": "1h",
        "entry_timeframe": "15m",
        "analysis_timeframe": "4h"
      }
    },
    "range_trading": {
      "strategy_name": "range_trading",
      "enabled": false,
      "market_regime": {
        "detection_enabled": true,
        "trend_strength_min": 0.2,
        "volatility_threshold": 2.5,
        "adx_minimum": 8,
        "rsi_oversold": 20,
        "rsi_overbought": 80
      },
      "indicators": {
        "strong_trend": {
          "adx_min": 0,
          "ema_alignment_percent": 0,
          "rsi_threshold": 0
        },
        "medium_trend": {
          "adx_min": 0,
          "ema_alignment_percent": 0,
          "rsi_threshold": 0
        },
        "weak_trend": {
          "adx_min": 0,
          "ema_alignment_percent": 0,
          "rsi_threshold": 0
        },
        "sideways": {
          "adx_max": 12,
          "volume_threshold": 0.8,
          "breakout_percent": 0.5
        }
      },
      "entry_exit_rules": {
        "entry_signal_min_score": 50,
        "entry_confidence_threshold": 60,
        "exit_profit_target_percent": 2,
        "exit_stop_loss_percent": 1.0,
        "exit_trailing_stop_percent": 0.5,
        "max_hold_duration": "1d",
        "require_confirmation_candles": 1
      },
      "scoring_weights": {
        "technical_weight": 55,
        "context_weight": 20,
        "llm_weight": 15,
        "history_weight": 10,
        "minimum_threshold": 45
      },
      "calibration": {
        "risk_level": "conservative",
        "position_size_usd": 200,
        "max_concurrent_signals": 1,
        "rebalance_interval": "15m",
        "drawdown_limit_percent": 2
      },
      "timeframes": {
        "trend_timeframe": "15m",
        "entry_timeframe": "5m",
        "analysis_timeframe": "1h"
      }
    }
  }
}
```

### Redis Key Structure

```
// Per-user, per-strategy decision engine settings
decision_engine:settings:{userID}:{strategyType} → JSON(StrategySettings)
TTL: 24 hours (same as other user settings)

Example keys for user "user-123":
- decision_engine:settings:user-123:trend_following
- decision_engine:settings:user-123:mean_reversion
- decision_engine:settings:user-123:breakout
- decision_engine:settings:user-123:range_trading

// Complete decision engine settings (all strategies)
decision_engine:settings:user-{userID}:all → JSON(DecisionEngineSettings)
TTL: 24 hours
```

### Database Schema Addition

```sql
-- Add to user_settings table if not exists
CREATE TABLE IF NOT EXISTS user_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    decision_engine_settings JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_settings_user_id
ON user_settings(user_id);
```

---

## Dependencies

### Hard Dependencies
- **Story 11.1: Redis State Management** - Provides Redis infrastructure for caching
- **Story 11.15: Additive Score Calculator** - Consumes scoring weights from settings
- **Story 11.18: Blocking Reason Tracker** - Consumes threshold values from settings

### Soft Dependencies
- **Settings Lifecycle Rule** - Must follow exact pattern documented in CLAUDE.md

---

## Test Requirements

### Unit Tests (internal/services/decision_engine_settings_service_test.go)
- Test loading user settings from database
- Test loading settings from cache
- Test validation of settings values
- Test reset to defaults functionality
- Test settings merge for all strategies
- Test all four strategy types independently

### Integration Tests (internal/api/handlers_decision_engine_settings_test.go)
- Test GET /api/v1/decision-engine/settings returns correct structure
- Test GET /api/v1/decision-engine/settings/{strategy} for each strategy
- Test PUT /api/v1/decision-engine/settings/{strategy} with valid updates
- Test validation errors for invalid values
- Test write-through caching (DB update → cache update)
- Test POST /api/v1/decision-engine/settings/{strategy}/reset
- Test settings persistence across sessions
- Test concurrent settings updates

### Database Tests (internal/database/repository_user_settings_test.go)
- Test CRUD operations on user_settings table
- Test JSONB storage and retrieval
- Test strategy-level updates
- Test default value initialization

### E2E Tests (cypress/e2e/decision-engine-settings.cy.ts)
- Load user settings on login
- Display settings in UI with current and default values
- Update strategy settings via API
- Verify settings changes affect decision engine behavior
- Test reset to defaults
- Verify settings persist across logout/login

---

## Files to Create

### Backend
```
internal/models/decision_engine_settings.go
internal/services/decision_engine_settings_service.go
internal/cache/decision_engine_settings_cache.go
internal/database/repository_user_settings.go
internal/database/migrations/000X_add_decision_engine_settings.sql
internal/api/handlers_decision_engine_settings.go
```

### Tests
```
internal/services/decision_engine_settings_service_test.go
internal/cache/decision_engine_settings_cache_test.go
internal/database/repository_user_settings_test.go
internal/api/handlers_decision_engine_settings_test.go
cypress/e2e/decision-engine-settings.cy.ts
```

---

## Files to Modify

### Backend Configuration
```
default-settings.json
internal/models/models_user.go
internal/database/user_initialization.go
internal/cache/settings_cache_service.go
internal/cache/admin_defaults_cache.go
internal/cache/redis.go
internal/api/routes.go
```

### Decision Engine Integration
```
internal/decision/decision_engine.go
internal/services/scoring_calculator.go
internal/services/blocking_tracker_service.go
```

---

## Success Metrics

- All 5 acceptance criteria satisfied
- Settings load from cache in <50ms
- Settings update (write-through) in <100ms
- Default values accessible and documented
- Settings persist correctly across sessions
- Reset to defaults works for all strategies
- Test coverage >85%
- API response times <200ms
- All four strategies configurable independently
- Settings changes reflected in decision engine behavior immediately

---

## Notes

- This story only covers BACKEND settings structure and integration
- Frontend components for settings UI are out of scope (future frontend story)
- Must strictly follow Settings Lifecycle Rule from CLAUDE.md
- Settings are per-user (different users can have different configs)
- Default values in default-settings.json are source of truth for new users
- All strategy thresholds and parameters must be documented with ranges
- Consider making settings codes/descriptions i18n-compatible for future multilingual support
- Do NOT create new admin-only settings - use existing admin defaults cache pattern

---

## Change Log

| Date | Status | Notes |
|------|--------|-------|
| 2026-01-17 | ready-for-dev | Story created with complete backend implementation plan |
