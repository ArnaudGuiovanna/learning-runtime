# [FEATURE] Route review intent through the regulation pipeline

Suggested labels: `enhancement`, `p2`, `todo`

## Description

> Priority: p2
> Classification: REFACTOR

When `intent=review`, `get_next_activity` bypasses the orchestrator and uses `selectReviewIntentActivity`. This creates a second routing policy with its own scoring and activity selection logic instead of expressing review as a constraint inside the regulation pipeline.

Affected files:

- `tools/activity.go` — review intent bypasses `engine.OrchestrateWithPhase`.
- `tools/activity_intent.go` — separate review selector and scoring policy.
- `engine/orchestrator.go` — no intent-aware routing input today.

## Proposed solution

Add intent awareness to the regulation pipeline. For `intent=review`, bias selection toward retention and previously studied concepts while preserving Gate, ConceptSelector, ActionSelector, and evidence logic.

Suggested semantics:

- phase bias: `MAINTENANCE`
- allowed activity families: `RECALL_EXERCISE`, `PRACTICE`, `DEBUG_MISCONCEPTION`
- concept scoring: retention-first, no new concepts

## Acceptance criteria

- [ ] `intent=review` goes through the orchestrator path.
- [ ] Review still avoids introducing new concepts.
- [ ] Review still prioritizes low retention and active misconceptions.
- [ ] Existing review behavior has migration tests.
- [ ] The duplicate review selector is removed or clearly marked as fallback-only.
