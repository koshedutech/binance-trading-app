---
title: 'Story Complete Cycle - Full Lifecycle with Epic Mode & Context Management'
validation-target: 'Complete story/epic lifecycle with intelligent orchestration'
validation-criticality: 'HIGHEST'
required-inputs:
  - 'Epic file OR story file OR auto-discovery from sprint-status'
  - 'Sprint-status.yaml (created if not exists)'
  - 'Sub-workflow access for all lifecycle phases'
optional-inputs:
  - 'epic_number: Process specific epic'
  - 'story_path: Process specific story'
  - 'context_budget: small|medium|large|unlimited'
  - 'stories_per_batch: Override auto-estimation'
validation-rules:
  - 'Orchestrator delegates heavy work to sub-agents'
  - 'Context preserved for multi-story sessions'
  - 'Strict gates enforced at each phase'
  - 'Progress tracked across stories and sessions'
---

# Story Complete Cycle - Definition of Done

**Master orchestrator supporting STORY MODE and EPIC MODE with intelligent context management**

---

## Workflow Modes

### Story Mode (Default)
Process single story through complete lifecycle.
```
story-complete-cycle [story_path=path/to/story.md]
```

### Epic Mode
Process entire epic with intelligent batching.
```
story-complete-cycle epic=12
story-complete-cycle epic_path=path/to/epic-12.md
```

### Auto Mode
Discover next available work from sprint-status.
```
story-complete-cycle
```

---

## Context Management Strategy

### Delegation Pattern
| Phase | Delegated? | Why |
|-------|------------|-----|
| Sprint Planning | No | Lightweight (file operations) |
| Story Discovery | No | Lightweight (file operations) |
| Story Validation | Yes | May involve deep analysis |
| Implementation | **Yes** | Heavy work (coding, testing) |
| Code Review | **Yes** | Heavy work (analysis, fixes) |
| QA Trace | **Yes** | Heavy work (traceability) |
| Completion | No | Lightweight (status updates) |

### Context Budget Recommendations
| Budget | Stories/Session | Best For |
|--------|-----------------|----------|
| small | ~3 | Complex stories, limited context |
| medium | ~5 | Mixed complexity |
| large | ~8 | Simple stories, ample context |
| unlimited | All | Full epic in one session |

### Complexity Estimation
| Story Type | Tasks | Est. Tokens |
|------------|-------|-------------|
| Small | 1-2 | ~15,000 |
| Medium | 3-5 | ~30,000 |
| Large | 6+ | ~50,000 |

**Orchestrator Overhead:** ~5,000 + (2,000 per story)

---

## Full Lifecycle Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    STORY COMPLETE CYCLE                      │
├─────────────────────────────────────────────────────────────┤
│ Epic Mode Only:                                              │
│   [1] Load Epic → [2] Analyze Stories → [3] Estimate Context │
│   [4] Build Queue → [5] Loop Through Stories                 │
├─────────────────────────────────────────────────────────────┤
│ Per Story (Epic or Story Mode):                              │
│   [6] Route by Status                                        │
│   [7] Create Story (if needed)           ← Sub-agent         │
│   [8] Validate Story (SM Review)         ← Sub-agent         │
│   [9] Mark Ready-for-Dev                                     │
│   [10] Implement (dev-story)             ← Sub-agent         │
│   [11] Code Review (with fix loop)       ← Sub-agent         │
│   [12] QA Trace (testarch-trace)         ← Sub-agent         │
│   [13] Mark Done                                             │
│   [14] Next Story or Complete                                │
├─────────────────────────────────────────────────────────────┤
│ Session Summary:                                             │
│   [15] Report Results → Suggest Next Steps                   │
└─────────────────────────────────────────────────────────────┘
```

---

## Per-Story Checklist

### Phase 1: Story Creation (if needed)
- [ ] **Story File Created:** From epic definition
- [ ] **Sprint Status Updated:** story_key = "draft"
- [ ] **Delegated:** Sub-agent handled creation

### Phase 2: Story Validation (SM Review)
- [ ] **Required Sections:** Story, ACs, Tasks, Dev Notes present
- [ ] **Architecture Aligned:** Follows project patterns
- [ ] **Validation Result:** PASS or PASS_WITH_NOTES
- [ ] **Delegated:** Sub-agent performed validation

### Phase 3: Mark Ready for Development
- [ ] **Story Status:** draft → ready-for-dev
- [ ] **Sprint Status Synced:** Updated in sprint-status.yaml
- [ ] **Change Log:** Entry added

### Phase 4: Implementation
- [ ] **All Tasks Complete:** Every task/subtask marked [x]
- [ ] **Tests Written:** Unit, integration, E2E as required
- [ ] **Tests Pass:** No regressions
- [ ] **Story Status:** ready-for-dev → in-progress → review
- [ ] **Delegated:** Sub-agent did all implementation

### Phase 5: Code Review
- [ ] **Review Executed:** Adversarial review completed
- [ ] **Issues Addressed:** AUTO-FIXED by sub-agent
- [ ] **Review Outcome:** Approve (within 3 attempts)
- [ ] **Delegated:** Sub-agent handled review and fixes

### Phase 6: QA Traceability
- [ ] **Trace Executed:** Requirements-to-tests matrix generated
- [ ] **QA Result:** **PASS** (strict gate)
- [ ] **Trace Report:** Generated and accessible
- [ ] **Delegated:** Sub-agent ran testarch-trace

### Phase 7: Completion
- [ ] **Story Status:** review → done
- [ ] **Sprint Status:** Updated to "done"
- [ ] **Change Log:** Completion entry with timestamp

---

## Epic Mode Additional Checks

### Initialization
- [ ] **Epic File Loaded:** All stories extracted
- [ ] **Complexity Analyzed:** Stories categorized (small/medium/large)
- [ ] **Context Estimated:** Recommended batch size calculated
- [ ] **Queue Built:** Stories prioritized by status

### Progress Tracking
- [ ] **Stories Completed:** Count tracked across session
- [ ] **Stories Failed:** Blocked stories recorded with reasons
- [ ] **Epic Progress:** Remaining stories calculated

### Session Continuity
- [ ] **Partial Progress Saved:** Sprint-status.yaml updated after each story
- [ ] **Resume Supported:** Re-running continues from current state
- [ ] **Next Session Guidance:** Clear instructions for continuation

---

## Gate Enforcement

| Gate | Requirement | Blocking? | Fallback |
|------|-------------|-----------|----------|
| Story Validation | PASS/PASS_WITH_NOTES | Yes | Mark blocked, next story |
| Implementation | Complete without HALT | Yes | Mark blocked, next story |
| Code Review | Approve (≤3 attempts) | Yes | Mark blocked, next story |
| QA Trace | **PASS only** | Yes | Mark blocked, next story |

**Strict Policy:** Failed gates don't halt entire epic - story is marked blocked and orchestrator continues to next story.

---

## Status Transitions

### Story Status Flow
```
not-started → draft → ready-for-dev → in-progress → review → done
     ↓          ↓           ↓              ↓           ↓
   Step 7    Step 8      Step 9        Step 10     Step 11-13
```

### Sprint-Status.yaml Sync
Every status change is immediately synced to sprint-status.yaml.

---

## Session Summary Output

```
===== SESSION COMPLETE =====

Mode: epic
Epic: 12 - WebSocket Real-Time Data

Results:
- Stories Processed: 5
- Completed: 4
- Failed/Blocked: 1

Success Rate: 80%

Epic Progress:
- Total Stories: 10
- Completed: 7
- Remaining: 3

To Continue:
Run `story-complete-cycle epic=12` to process remaining stories.

Attention Required:
1 story was blocked:
- 12-5-ginie-autopilot: QA gate returned CONCERNS
```

---

## Success Criteria

```
Story Complete Cycle: {{PASS/PARTIAL/FAIL}}

Mode: {{story/epic/auto}}
Epic: {{epic_number}} (if epic mode)

Session Results:
- Stories Queued: {{queue_size}}
- Completed: {{stories_completed}}
- Blocked: {{stories_failed}}
- Success Rate: {{percentage}}%

Context Usage:
- Budget: {{context_budget}}
- Recommended: {{recommended_stories}}
- Processed: {{queue_size}}

Delegation Stats:
- Sub-agents Spawned: {{agent_count}}
- Phases Delegated: validation, implementation, code-review, qa-trace
```

**PASS:** All queued stories completed successfully
**PARTIAL:** Some stories completed, others blocked (epic continues)
**FAIL:** No stories could be completed (all blocked)
