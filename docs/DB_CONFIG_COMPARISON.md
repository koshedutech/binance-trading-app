# Database Configuration Comparison - Epic 4 Stories 4.5/4.6

## Purpose
This document records the configuration values synchronized between the GitHub code (`autopilot_settings.json`) and the database (`user_mode_configs` table) for the test user. These values should be used to verify that signal detection behaves identically after merging the DB-first implementation.

## User Details
- **Email**: (test user account)
- **User ID**: (redacted)
- **Sync Date**: 2026-01-04

---

## Mode Configuration Comparison

### Scalp Mode
| Setting | GitHub JSON Value | Database Value |
|---------|-------------------|----------------|
| enabled | true | true |
| min_confidence | 40 | 40 |
| high_confidence | 75 | 75 |
| ultra_confidence | 85 | 85 |
| leverage | 10 | 10 |
| min_adx | 20 | 20 |
| llm_enabled | true | true |
| llm_weight | 0.2 | 0.2 |
| skip_on_timeout | true | true |
| min_llm_confidence | 50 | 50 |
| block_on_disagreement | false | false |

### Scalp Reentry Mode
| Setting | GitHub JSON Value | Database Value |
|---------|-------------------|----------------|
| enabled | true | true |
| min_confidence | 60 | 60 |
| high_confidence | 75 | 75 |
| ultra_confidence | 88 | 88 |
| leverage | 10 | 10 |
| min_adx | 20 | 20 |
| llm_enabled | true | true |
| llm_weight | 0.35 | 0.35 |
| skip_on_timeout | false | false |
| min_llm_confidence | 55 | 55 |
| block_on_disagreement | false | false |

### Swing Mode
| Setting | GitHub JSON Value | Database Value |
|---------|-------------------|----------------|
| enabled | true | true |
| min_confidence | 40 | 40 |
| high_confidence | 80 | 80 |
| ultra_confidence | 90 | 90 |
| leverage | 10 | 10 |
| min_adx | 25 | 25 |
| llm_enabled | true | true |
| llm_weight | 0.4 | 0.4 |
| skip_on_timeout | false | false |
| min_llm_confidence | 60 | 60 |
| block_on_disagreement | true | true |

### Position Mode
| Setting | GitHub JSON Value | Database Value |
|---------|-------------------|----------------|
| enabled | false | false |
| min_confidence | 40 | 40 |
| high_confidence | 85 | 85 |
| ultra_confidence | 92 | 92 |
| leverage | 3 | 3 |
| min_adx | 30 | 30 |
| llm_enabled | true | true |
| llm_weight | 0.5 | 0.5 |
| skip_on_timeout | false | false |
| min_llm_confidence | 65 | 65 |
| block_on_disagreement | true | true |

### Ultra Fast Mode
| Setting | GitHub JSON Value | Database Value |
|---------|-------------------|----------------|
| enabled | true | true |
| min_confidence | 40 | 40 |
| high_confidence | 80 | 80 |
| ultra_confidence | 90 | 90 |
| leverage | 10 | 10 |
| min_adx | 15 | 15 |
| llm_enabled | true | true |
| llm_weight | 0.1 | 0.1 |
| skip_on_timeout | true | true |
| min_llm_confidence | 40 | 40 |
| block_on_disagreement | false | false |

---

## ADX Threshold Defaults (Hardcoded in GitHub Code)
| Mode | ADX Threshold |
|------|---------------|
| ultra_fast | 15 |
| scalp | 20 |
| swing | 25 |
| position | 30 |

These are also stored in the `risk.min_adx` field in the database config_json.

---

## Global Settings (From autopilot_settings.json)
| Setting | Value |
|---------|-------|
| MinConfidenceToTrade (hardcoded) | 35% |
| risk_level | moderate |
| ginie_dry_run_mode | false |
| llm_config.enabled | true |
| llm_config.provider | deepseek |
| llm_config.timeout_ms | 5000 |

---

## Verification Steps After Merging DB-First Code

1. **Unstash the DB-first changes**: `git stash pop`
2. **Restart the container**: `./scripts/docker-dev.sh`
3. **Verify signals are generated** with the same behavior as GitHub code
4. **Check confidence calculations** match the fusion formula:
   - `baseFusion = (tech × techWeight) + (llm × llmWeight)`
   - With agreement bonus when tech and LLM signals align

---

## SQL to Verify Database Values

```sql
SELECT mode_name, enabled,
       config_json->'confidence' as confidence,
       config_json->'risk' as risk,
       config_json->'size'->>'leverage' as leverage,
       config_json->'llm' as llm
FROM user_mode_configs
WHERE user_id = '35d1a6ba-2143-4327-8e28-1b7417281b97'
ORDER BY mode_name;
```

---

## Known Differences Between JSON and DB-First Code

1. **Configuration Source**:
   - GitHub code: Reads from `autopilot_settings.json`
   - DB-first code: Reads from `user_mode_configs` table

2. **MinConfidenceToTrade**:
   - GitHub code: Hardcoded to 35% globally
   - DB-first code: Should read from `confidence.min_confidence` per mode

3. **LLM Settings**:
   - GitHub code: Reads from `mode_llm_settings` section
   - DB-first code: Should read from `llm` object in config_json
