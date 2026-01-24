# Story 11.46: Volume Imbalance - UI Components

## Story Overview

**Story ID:** 11.46
**Epic:** Epic 11 - Position Decision Engine
**Parent Story:** 11.43 (Ravindra Volume Imbalance Strategy)
**Priority:** P1 (High)
**Status:** Done
**Created:** 2026-01-24

---

## Business Context

This story adds the UI components for:
1. Mode Configuration - configure strategy groups and sub-strategies
2. Entry Decision Engine - display pattern states and entry signals

---

## Dependencies

- **Story 11.44:** Database Schema (MUST be completed first)
- **Story 11.45:** API Endpoints (MUST be completed first)

---

## Scope

### In Scope
- Mode Configuration page updates
- Entry Decision Engine pattern display
- Trailing stop status display
- R:R display as "1:4" format

### Out of Scope
- LLM validation UI (Story 11.47)

---

## Technical Implementation

### Task 1: Mode Configuration - Strategy Groups

**File:** `web/src/components/ModeConfiguration/StrategyGroupPanel.tsx`

```tsx
interface StrategyGroupPanelProps {
    mode: string;           // 'scalp', 'swing', 'position'
    group: string;          // 'breakout', 'trending', 'range', 'volatile'
    settings: StrategyGroupSettings;
    onUpdate: (settings: Partial<StrategyGroupSettings>) => void;
}

// Display:
// - Enable/Disable toggle
// - Base settings (timeframe, position_size_percent, max_leverage, etc.)
// - List of sub-strategies with toggles
// - Expand/collapse for sub-strategy details
```

**UI Mockup:**
```
┌─────────────────────────────────────────────────────────┐
│ BREAKOUT STRATEGY GROUP                        [ENABLED]│
├─────────────────────────────────────────────────────────┤
│ Base Settings                                           │
│ ┌─────────────┬─────────────┬─────────────┐            │
│ │ Timeframe   │ Position %  │ Max Leverage│            │
│ │ [15m    ▼]  │ [2.0    ]   │ [10     ]   │            │
│ └─────────────┴─────────────┴─────────────┘            │
│                                                         │
│ Sub-Strategies                                          │
│ ┌─────────────────────────────────────────────────────┐│
│ │ [✓] Ravindra Volume Imbalance              [Config] ││
│ │     R:R: 1:4 | Trailing: ON | LLM: ON              ││
│ ├─────────────────────────────────────────────────────┤│
│ │ [ ] Classic Breakout                       [Config] ││
│ │     Threshold: 1.5% | Confirmation: 2 candles      ││
│ └─────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────┘
```

### Task 2: Sub-Strategy Settings Modal

**File:** `web/src/components/ModeConfiguration/SubStrategySettingsModal.tsx`

```tsx
// For Ravindra Volume Imbalance:
interface VolumeImbalanceSettings {
    min_rr_ratio: string;        // "1:4"
    llm_validation: boolean;
    trailing_stop: {
        enabled: boolean;
        milestones: Array<{
            at_rr: string;       // "1:2"
            move_sl_to: string;  // "entry" or "1:1"
        }>;
        target_rr: string;       // "1:4"
    };
    pattern_detection: {
        reference_lookback_candles: number;
        min_consolidation_candles: number;
        max_consolidation_candles: number;
        volume_spike_threshold: number;
        breakout_volume_surge: number;
    };
}
```

**UI Mockup:**
```
┌─────────────────────────────────────────────────────────┐
│ Ravindra Volume Imbalance Settings              [X]     │
├─────────────────────────────────────────────────────────┤
│ Risk-Reward                                             │
│ ┌─────────────────────────────────────────────────────┐│
│ │ Minimum R:R Ratio:  [1:4 ▼]                        ││
│ │ LLM Validation:     [✓] Enabled                    ││
│ └─────────────────────────────────────────────────────┘│
│                                                         │
│ Trailing Stop                                           │
│ ┌─────────────────────────────────────────────────────┐│
│ │ [✓] Enabled                                        ││
│ │ Milestones:                                        ││
│ │   At 1:2 → Move SL to Entry (Breakeven)           ││
│ │   At 1:3 → Move SL to 1:1 (Lock Profit)           ││
│ │ Target: 1:4 (Take Profit)                         ││
│ └─────────────────────────────────────────────────────┘│
│                                                         │
│ Pattern Detection                                       │
│ ┌─────────────────────────────────────────────────────┐│
│ │ Lookback Candles:    [20  ]                        ││
│ │ Min Consolidation:   [2   ] candles                ││
│ │ Max Consolidation:   [6   ] candles                ││
│ │ Volume Spike:        [2.0x] average                ││
│ │ Breakout Surge:      [1.5x] consolidation avg      ││
│ └─────────────────────────────────────────────────────┘│
│                                                         │
│                              [Cancel]  [Save Changes]   │
└─────────────────────────────────────────────────────────┘
```

### Task 3: Entry Decision Engine - Pattern State Display

**File:** `web/src/components/EntryDecisionEngine/VolumeImbalanceCard.tsx`

```tsx
interface VolumeImbalanceCardProps {
    symbol: string;
    pattern: VolumeImbalancePattern;
    onExecute?: () => void;
    onSkip?: () => void;
}

// State badges:
// - WATCHING (gray)
// - CONSOLIDATING (yellow) - show progress
// - READY (green) - show Entry/SL/TP and Execute button
```

**UI Mockup:**
```
┌─────────────────────────────────────────────────────────┐
│ BTCUSDT                                [READY]  [SCALP] │
├─────────────────────────────────────────────────────────┤
│ Ravindra Volume Imbalance Pattern                       │
│                                                         │
│ ┌─ Pattern Progress ────────────────────────────────┐  │
│ │ [✓] Step 1: Accumulation Start (2.3x volume)      │  │
│ │ [✓] Step 2: Sideways Consolidation (4 candles)    │  │
│ │ [✓] Step 3: Breakout Detected (1.8x surge)        │  │
│ └───────────────────────────────────────────────────┘  │
│                                                         │
│ ┌─ Trade Setup ─────────────────────────────────────┐  │
│ │ Entry:      $89,865.53                            │  │
│ │ Stop-Loss:  $89,428.48  (Risk: $437.05)          │  │
│ │ Take-Profit: $91,613.73 (Reward: $1,748.20)      │  │
│ │ R:R Ratio:  1:4                                   │  │
│ └───────────────────────────────────────────────────┘  │
│                                                         │
│ ┌─ Trailing Stop Plan ──────────────────────────────┐  │
│ │ At 1:2 ($90,302.58) → Move SL to Entry (0 risk)  │  │
│ │ At 1:3 ($90,739.63) → Move SL to 1:1 (lock $437) │  │
│ │ At 1:4 ($91,613.73) → Take Profit                │  │
│ └───────────────────────────────────────────────────┘  │
│                                                         │
│ LLM Validation: [Pending...]                           │
│                                                         │
│              [Skip Signal]        [Execute Trade]       │
└─────────────────────────────────────────────────────────┘
```

### Task 4: Pattern State in Entry Decision Engine List

**File:** `web/src/pages/EntryDecisionEngine.tsx` (update)

Add new section for Volume Imbalance patterns:
```tsx
// New section showing active patterns
<VolumeImbalanceSection>
    <h3>Volume Imbalance Patterns</h3>
    {patterns.map(pattern => (
        <VolumeImbalanceCard key={pattern.symbol} pattern={pattern} />
    ))}
</VolumeImbalanceSection>
```

### Task 5: Trailing Stop Status in Active Position

**File:** `web/src/components/PositionCard.tsx` (update)

Show trailing stop status for positions entered via Volume Imbalance:
```tsx
// If position has trailing stop manager
{position.trailingStop && (
    <TrailingStopStatus>
        <span>SL at: {position.trailingStop.currentSL}</span>
        <span>Next milestone: {position.trailingStop.nextMilestone}</span>
    </TrailingStopStatus>
)}
```

---

## Acceptance Criteria

### AC1: Mode Configuration UI
- [ ] Strategy groups displayed per mode
- [ ] Base settings configurable (timeframe, position %, leverage)
- [ ] Sub-strategies toggleable with expand for details
- [ ] R:R displayed as "1:4" format (not "4.0")

### AC2: Sub-Strategy Settings Modal
- [ ] All Volume Imbalance settings editable
- [ ] Trailing stop milestones displayed
- [ ] Pattern detection parameters configurable
- [ ] Validation on input

### AC3: Entry Decision Engine - Pattern Display
- [ ] Pattern state visible (WATCHING → CONSOLIDATING → READY)
- [ ] Progress indicator for each step
- [ ] Entry/SL/TP displayed when READY
- [ ] Execute/Skip buttons for ready signals

### AC4: Trailing Stop Display
- [ ] Current SL level shown
- [ ] Next milestone indicated
- [ ] Visual progress toward target

### AC5: Only Show Enabled
- [ ] Disabled modes not shown in Entry Decision Engine
- [ ] Disabled strategy groups not shown
- [ ] Disabled sub-strategies not shown

---

## Test Plan

1. **Component Tests:** Each component renders correctly
2. **Integration Tests:** Settings save and load correctly
3. **E2E Tests:** Full flow from configuration to signal display

---

## Estimation

| Task | Effort |
|------|--------|
| Strategy Group Panel | Medium |
| Sub-Strategy Modal | Medium |
| Volume Imbalance Card | High |
| Pattern State Display | Medium |
| Trailing Stop Status | Small |
| Testing | High |

**Total:** High
