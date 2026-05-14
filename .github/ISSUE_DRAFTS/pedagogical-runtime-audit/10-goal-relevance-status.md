# [FEATURE] Expose goal_relevance_status in get_next_activity

Suggested labels: `enhancement`, `p2`, `todo`

## Description

> Priority: p2
> Classification: PERSONALIZATION

Goal relevance is optional. After `init_domain`, the runtime suggests `set_goal_relevance`, but marks it as not required. If no relevance vector exists, the orchestrator and selector fall back to uniform relevance (`rel=1.0`), which keeps routing functional but silently reduces personalization.

Affected files:

- `tools/domain.go` — `init_domain` emits an optional `set_goal_relevance` next action.
- `engine/orchestrator.go` — missing goal relevance becomes uniform relevance.
- `engine/concept_selector.go` — nil relevance falls back to uniform scoring.
- `tools/activity.go` — response does not expose whether goal-aware routing is missing, partial, stale, or valid.

## Proposed solution

Add `goal_relevance_status` to `get_next_activity`:

```json
{
  "goal_relevance_status": {
    "status": "missing",
    "message": "Goal-aware routing is using uniform relevance because no relevance vector exists.",
    "recommended_tool": "set_goal_relevance"
  }
}
```

Statuses should distinguish at least `missing`, `partial`, and `valid`; `stale` can be included if graph-version drift is already available.

## Acceptance criteria

- [ ] `get_next_activity` includes `goal_relevance_status`.
- [ ] Missing, partial, and valid relevance vectors are distinguishable.
- [ ] Missing status recommends `set_goal_relevance`.
- [ ] Tests cover missing, partial, and valid statuses.
